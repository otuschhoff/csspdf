package pdfdump

import (
	"fmt"
	"strconv"
	"strings"
)

func parseIntArray(value string) ([]int, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, false
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return []int{}, true
	}
	result := make([]int, 0, len(strings.Fields(inner)))
	for _, part := range strings.Fields(inner) {
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		result = append(result, number)
	}
	return result, true
}

func readBigEndianField(data []byte, position *int, width int) (uint64, bool) {
	if width == 0 {
		return 0, true
	}
	if *position+width > len(data) {
		return 0, false
	}
	var value uint64
	for index := 0; index < width; index++ {
		value = value<<8 | uint64(data[*position+index])
	}
	*position += width
	return value, true
}

func paethPredictor(left, upper, upperLeft byte) byte {
	leftDistance := int(upper) - int(upperLeft)
	upperDistance := int(left) - int(upperLeft)
	upperLeftDistance := leftDistance + upperDistance
	if upperLeftDistance < 0 {
		upperLeftDistance = -upperLeftDistance
	}
	if leftDistance < 0 {
		leftDistance = -leftDistance
	}
	if upperDistance < 0 {
		upperDistance = -upperDistance
	}
	if leftDistance <= upperDistance && leftDistance <= upperLeftDistance {
		return left
	}
	if upperDistance <= upperLeftDistance {
		return upper
	}
	return upperLeft
}

func decodePNGPredictor(data []byte, columns int) ([]byte, error) {
	if columns <= 0 {
		return nil, fmt.Errorf("invalid columns: %d", columns)
	}
	rowSize := columns + 1
	if len(data)%rowSize != 0 {
		return nil, fmt.Errorf("predictor row size mismatch: len=%d row=%d", len(data), rowSize)
	}
	result := make([]byte, len(data)/rowSize*columns)
	for row := 0; row < len(data)/rowSize; row++ {
		input := data[row*rowSize : (row+1)*rowSize]
		output := result[row*columns : (row+1)*columns]
		var previous []byte
		if row > 0 {
			previous = result[(row-1)*columns : row*columns]
		}
		if err := decodePNGRow(input[0], input[1:], output, previous); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func decodePNGRow(filter byte, current, output, previous []byte) error {
	switch filter {
	case 0:
		copy(output, current)
	case 1:
		decodePNGSub(current, output)
	case 2:
		decodePNGUp(current, output, previous)
	case 3:
		decodePNGAverage(current, output, previous)
	case 4:
		decodePNGPaeth(current, output, previous)
	default:
		return fmt.Errorf("unsupported PNG predictor filter: %d", filter)
	}
	return nil
}

func decodePNGSub(current, output []byte) {
	for index := range current {
		left := byte(0)
		if index > 0 {
			left = output[index-1]
		}
		output[index] = current[index] + left
	}
}
func decodePNGUp(current, output, previous []byte) {
	for index := range current {
		upper := byte(0)
		if previous != nil {
			upper = previous[index]
		}
		output[index] = current[index] + upper
	}
}
func decodePNGAverage(current, output, previous []byte) {
	for index := range current {
		left, upper := byte(0), byte(0)
		if index > 0 {
			left = output[index-1]
		}
		if previous != nil {
			upper = previous[index]
		}
		output[index] = current[index] + byte((int(left)+int(upper))/2)
	}
}
func decodePNGPaeth(current, output, previous []byte) {
	for index := range current {
		left, upper, upperLeft := byte(0), byte(0), byte(0)
		if index > 0 {
			left = output[index-1]
		}
		if previous != nil {
			upper = previous[index]
			if index > 0 {
				upperLeft = previous[index-1]
			}
		}
		output[index] = current[index] + paethPredictor(left, upper, upperLeft)
	}
}

func formatXRefStream(decoded []byte, dictionary string) string {
	entries := dictEntriesMap(strings.TrimSpace(dictionary))
	var err error
	decoded, err = decodeXRefPredictor(decoded, entries["DecodeParms"])
	if err != nil {
		return fmt.Sprintf("XRef decode: predictor decode failed: %v\n", err)
	}
	widths, ok := parseIntArray(entries["W"])
	if !ok || len(widths) != 3 {
		return "XRef decode: missing or invalid /W array\n"
	}
	entryWidth := widths[0] + widths[1] + widths[2]
	if entryWidth <= 0 {
		return "XRef decode: invalid entry width\n"
	}
	index, err := parseXRefIndex(entries)
	if err != nil {
		return "XRef decode: " + err.Error() + "\n"
	}
	required := countXRefEntries(index) * entryWidth
	if len(decoded) < required {
		return fmt.Sprintf("XRef decode: stream too short (%d < %d)\n", len(decoded), required)
	}
	var builder strings.Builder
	builder.WriteString("Decoded XRef entries:\n")
	fmt.Fprintf(&builder, "- W: [%d %d %d]\n", widths[0], widths[1], widths[2])
	builder.WriteString("- Index ranges:\n")
	for position := 0; position < len(index); position += 2 {
		fmt.Fprintf(&builder, "  - [%d .. %d] (%d entries)\n", index[position], index[position]+index[position+1]-1, index[position+1])
	}
	builder.WriteString("- Entries:\n")
	if !writeXRefEntries(&builder, decoded, index, widths) {
		return builder.String()
	}
	if len(decoded) > required {
		fmt.Fprintf(&builder, "- Note: %d trailing byte(s) after xref entries\n", len(decoded)-required)
	}
	return builder.String()
}

func decodeXRefPredictor(decoded []byte, raw string) ([]byte, error) {
	if raw == "" {
		return decoded, nil
	}
	parameters := dictEntriesMap(raw)
	predictor, _ := strconv.Atoi(parameters["Predictor"])
	if predictor < 10 {
		return decoded, nil
	}
	columns, _ := strconv.Atoi(parameters["Columns"])
	if columns <= 0 {
		columns = 1
	}
	return decodePNGPredictor(decoded, columns)
}
func parseXRefIndex(dictionary map[string]string) ([]int, error) {
	if raw, ok := dictionary["Index"]; ok {
		index, valid := parseIntArray(raw)
		if !valid || len(index)%2 != 0 {
			return nil, fmt.Errorf("invalid /Index array")
		}
		return index, nil
	}
	size, err := strconv.Atoi(dictionary["Size"])
	if err != nil {
		return nil, fmt.Errorf("missing /Index and invalid /Size")
	}
	return []int{0, size}, nil
}
func countXRefEntries(index []int) int {
	total := 0
	for position := 1; position < len(index); position += 2 {
		total += index[position]
	}
	return total
}

func writeXRefEntries(builder *strings.Builder, decoded []byte, index, widths []int) bool {
	position := 0
	for rangeIndex := 0; rangeIndex < len(index); rangeIndex += 2 {
		for offset := 0; offset < index[rangeIndex+1]; offset++ {
			first, ok1 := readBigEndianField(decoded, &position, widths[0])
			second, ok2 := readBigEndianField(decoded, &position, widths[1])
			third, ok3 := readBigEndianField(decoded, &position, widths[2])
			if !ok1 || !ok2 || !ok3 {
				builder.WriteString("  - [decode error: truncated entry]\n")
				return false
			}
			entryType := first
			if widths[0] == 0 {
				entryType = 1
			}
			writeXRefEntry(builder, index[rangeIndex]+offset, entryType, second, third)
		}
	}
	return true
}

func writeXRefEntry(builder *strings.Builder, object int, entryType, second, third uint64) {
	switch entryType {
	case 0:
		fmt.Fprintf(builder, "  - obj %d: free, next=%d gen=%d\n", object, second, third)
	case 1:
		fmt.Fprintf(builder, "  - obj %d: in-use, offset=%d gen=%d\n", object, second, third)
	case 2:
		fmt.Fprintf(builder, "  - obj %d: compressed, objstm=%d index=%d\n", object, second, third)
	default:
		fmt.Fprintf(builder, "  - obj %d: type=%d field2=%d field3=%d\n", object, entryType, second, third)
	}
}
