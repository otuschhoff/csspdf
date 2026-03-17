package pdfdump

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	objRe    = regexp.MustCompile(`(?s)(\d+)\s+0\s+obj\s*(.*?)\s*endobj`)
	streamRe = regexp.MustCompile(`(?s)stream\s*(.*?)\s*endstream`)
	binaryRe = regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F-\xFF]{10,}`)
	useColor = stdoutSupportsColor()
)

func stdoutSupportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "" || term == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func ansiStyle(s, code string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func objectHeader(objNum string, optional bool) string {
	banner := "OBJECT: " + objNum
	if optional {
		banner += " (optional)"
	}
	if useColor {
		if optional {
			return ansiStyle("=== "+banner+" ===", "4;36")
		}
		return ansiStyle("=== "+banner+" ===", "1;4;31")
	}
	return "==================== " + banner + " ===================="
}

func commandCommentary(cmdLine string) string {
	line := strings.TrimSpace(cmdLine)
	if line == "" {
		return ""
	}
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return ""
	}
	op := tokens[len(tokens)-1]

	comments := map[string]string{
		"q": "save graphics state", "Q": "restore graphics state",
		"cm": "concatenate transformation matrix", "CS": "set stroking color space",
		"SCN": "set stroking color/pattern", "w": "set line width",
		"m": "move to start point", "c": "cubic Bezier curve segment",
		"h": "close current path", "S": "stroke path",
		"J": "set line cap style", "j": "set line join style",
		"G": "set stroking gray color", "g": "set non-stroking gray color",
		"W": "clip using non-zero winding rule", "n": "end path without filling or stroking",
		"re": "rectangle", "sh": "paint shading pattern",
		"f":  "fill path using non-zero winding rule",
		"BT": "begin text object", "ET": "end text object",
		"Tm": "set text matrix", "Tf": "set text font and size",
		"TJ": "show text with individual glyph positioning",
		"cs": "set non-stroking color space", "scn": "set non-stroking color/pattern",
		"B": "fill and stroke path",
	}

	msg, ok := comments[op]
	if !ok {
		return ""
	}

	if op == "TJ" {
		textContent := extractTJText(line)
		if textContent != "" && isReadable(textContent) {
			if useColor {
				return "\n  " + ansiStyle("# "+textContent, "90")
			}
			return "\n  # " + textContent
		}
		return ""
	}

	if useColor {
		return "  " + ansiStyle("# "+msg, "90")
	}
	return "  # " + msg
}

func extractTJText(cmdLine string) string {
	start := strings.Index(cmdLine, "[")
	end := strings.LastIndex(cmdLine, "]")
	if start == -1 || end == -1 || start >= end {
		return ""
	}

	arrayContent := cmdLine[start+1 : end]
	var textParts []string
	inString := false
	inHexString := false
	escaped := false
	var currentString strings.Builder

	for i := 0; i < len(arrayContent); i++ {
		ch := arrayContent[i]
		if escaped {
			currentString.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			currentString.WriteByte(ch)
			continue
		}
		if ch == '(' && !inHexString {
			inString = true
			currentString.Reset()
		} else if ch == ')' && inString {
			textParts = append(textParts, currentString.String())
			inString = false
		} else if ch == '<' && !inString {
			inHexString = true
			currentString.Reset()
		} else if ch == '>' && inHexString {
			hexStr := currentString.String()
			decoded := decodeHexString(hexStr)
			if decoded != "" {
				textParts = append(textParts, decoded)
			}
			inHexString = false
		} else if inString || inHexString {
			currentString.WriteByte(ch)
		}
	}
	return strings.Join(textParts, "")
}

func decodeHexString(hexStr string) string {
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	if len(hexStr)%2 != 0 {
		return ""
	}
	var result strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		hexByte := hexStr[i : i+2]
		val, err := strconv.ParseUint(hexByte, 16, 8)
		if err != nil {
			continue
		}
		result.WriteByte(byte(val))
	}
	return result.String()
}

func isReadable(s string) bool {
	if len(s) == 0 {
		return false
	}
	readableCount := 0
	for _, r := range s {
		if (r >= 32 && r <= 126) || r == '\t' || r == '\n' || r == '\r' {
			readableCount++
		}
	}
	return float64(readableCount)/float64(len(s)) >= 0.7
}

func objectLabelText(objID string) string {
	label := "Object: " + objID
	if useColor {
		return ansiStyle(label, "90")
	}
	return label
}

func annotateDictStartWithObjectID(prettyDict, objID string) string {
	lines := strings.Split(prettyDict, "\n")
	if len(lines) == 0 {
		return prettyDict
	}
	if strings.TrimSpace(lines[0]) == "<<" {
		lines[0] = "<<  " + objectLabelText(objID)
	}
	return strings.Join(lines, "\n")
}

func decompressFlate(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func formatDictionary(dict string) string {
	d := strings.TrimSpace(dict)
	if !strings.HasPrefix(d, "<<") || !strings.HasSuffix(d, ">>") {
		return d
	}
	entries := parseDictEntries(d)
	if len(entries) == 0 {
		return d
	}
	ordered := orderDictEntries(entries)
	var b strings.Builder
	b.WriteString("<<\n")
	for _, e := range ordered {
		b.WriteString("    /")
		b.WriteString(e.Key)
		if e.Value != "" {
			b.WriteString(" ")
			b.WriteString(e.Value)
		}
		b.WriteString("\n")
	}
	b.WriteString(">>")
	return b.String()
}

type dictEntry struct {
	Key   string
	Value string
	Index int
}

func parseDictEntries(dict string) []dictEntry {
	inner := strings.TrimSpace(dict)
	if strings.HasPrefix(inner, "<<") {
		inner = strings.TrimSpace(inner[2:])
	}
	if strings.HasSuffix(inner, ">>") {
		inner = strings.TrimSpace(inner[:len(inner)-2])
	}
	if inner == "" {
		return nil
	}

	entries := []dictEntry{}
	i := 0
	idx := 0
	for i < len(inner) {
		for i < len(inner) && isSpace(inner[i]) {
			i++
		}
		if i >= len(inner) {
			break
		}
		if inner[i] != '/' {
			_, ni := parsePDFObjectToken(inner, i)
			if ni <= i {
				break
			}
			i = ni
			continue
		}

		key, ni := parseNameToken(inner, i)
		i = ni
		for i < len(inner) && isSpace(inner[i]) {
			i++
		}

		value := ""
		if i < len(inner) {
			val, nvi := parsePDFObjectToken(inner, i)
			if nvi > i {
				value = compactSpaces(val)
				i = nvi
			}
		}

		entries = append(entries, dictEntry{Key: key, Value: value, Index: idx})
		idx++
	}
	return entries
}

func isDictDelimiter(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '/', '[', ']', '<', '>', '(', ')':
		return true
	default:
		return false
	}
}

func parseNameToken(s string, i int) (string, int) {
	if i >= len(s) || s[i] != '/' {
		return "", i
	}
	j := i + 1
	for j < len(s) && !isDictDelimiter(s[j]) {
		j++
	}
	return s[i+1 : j], j
}

func parsePDFObjectToken(s string, i int) (string, int) {
	if i >= len(s) {
		return "", i
	}

	if s[i] == '<' && i+1 < len(s) && s[i+1] == '<' {
		depth := 1
		j := i + 2
		for j < len(s) {
			if s[j] == '<' && j+1 < len(s) && s[j+1] == '<' {
				depth++
				j += 2
				continue
			}
			if s[j] == '>' && j+1 < len(s) && s[j+1] == '>' {
				depth--
				j += 2
				if depth == 0 {
					return s[i:j], j
				}
				continue
			}
			j++
		}
		return s[i:], len(s)
	}

	if s[i] == '[' {
		depth := 1
		j := i + 1
		for j < len(s) {
			if s[j] == '[' {
				depth++
			} else if s[j] == ']' {
				depth--
				if depth == 0 {
					j++
					return s[i:j], j
				}
			}
			j++
		}
		return s[i:], len(s)
	}

	if s[i] == '(' {
		depth := 1
		j := i + 1
		for j < len(s) {
			if s[j] == '\\' {
				j += 2
				continue
			}
			if s[j] == '(' {
				depth++
			} else if s[j] == ')' {
				depth--
				if depth == 0 {
					j++
					return s[i:j], j
				}
			}
			j++
		}
		return s[i:], len(s)
	}

	if s[i] == '<' {
		j := i + 1
		for j < len(s) && s[j] != '>' {
			j++
		}
		if j < len(s) {
			j++
		}
		return s[i:j], j
	}

	if s[i] == '/' {
		_, j := parseNameToken(s, i)
		return s[i:j], j
	}

	j := i
	for j < len(s) && !isSpace(s[j]) {
		if s[j] == '/' || s[j] == '[' || s[j] == ']' || s[j] == '<' || s[j] == '>' || s[j] == '(' || s[j] == ')' {
			break
		}
		j++
	}
	if j == i {
		return string(s[i]), i + 1
	}
	base := s[i:j]

	if isIntegerToken(base) {
		k := j
		for k < len(s) && isSpace(s[k]) {
			k++
		}
		objNum, k2 := parseSimpleToken(s, k)
		if isIntegerToken(objNum) {
			k3 := k2
			for k3 < len(s) && isSpace(s[k3]) {
				k3++
			}
			rTok, k4 := parseSimpleToken(s, k3)
			if rTok == "R" {
				return compactSpaces(s[i:k4]), k4
			}
		}
	}
	return base, j
}

func parseSimpleToken(s string, i int) (string, int) {
	if i >= len(s) {
		return "", i
	}
	j := i
	for j < len(s) && !isSpace(s[j]) {
		if s[j] == '/' || s[j] == '[' || s[j] == ']' || s[j] == '<' || s[j] == '>' || s[j] == '(' || s[j] == ')' {
			break
		}
		j++
	}
	if j == i {
		return "", i
	}
	return s[i:j], j
}

func isIntegerToken(tok string) bool {
	if tok == "" {
		return false
	}
	start := 0
	if tok[0] == '-' || tok[0] == '+' {
		if len(tok) == 1 {
			return false
		}
		start = 1
	}
	for i := start; i < len(tok); i++ {
		if tok[i] < '0' || tok[i] > '9' {
			return false
		}
	}
	return true
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func compactSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func orderDictEntries(entries []dictEntry) []dictEntry {
	priority := map[string]int{
		"Type": 0, "Subtype": 1, "FunctionType": 2, "ShadingType": 3,
		"PatternType": 4, "ColorSpace": 5, "Domain": 6, "Bounds": 7,
		"Encode": 8, "Functions": 9, "C0": 10, "C1": 11, "N": 12,
		"Coords": 13, "Extend": 14, "Function": 15, "Shading": 16,
		"Matrix": 17, "Filter": 18, "DecodeParms": 19, "Length": 20,
		"First": 21,
	}

	out := make([]dictEntry, len(entries))
	copy(out, entries)
	sort.SliceStable(out, func(i, j int) bool {
		pi, okI := priority[out[i].Key]
		pj, okJ := priority[out[j].Key]
		if okI && okJ {
			if pi != pj {
				return pi < pj
			}
			return out[i].Index < out[j].Index
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return out[i].Index < out[j].Index
	})
	return out
}

func dictEntriesMap(dict string) map[string]string {
	m := map[string]string{}
	for _, e := range parseDictEntries(dict) {
		m[e.Key] = e.Value
	}
	return m
}

func isOptionalObject(dictStr string) bool {
	dm := dictEntriesMap(strings.TrimSpace(dictStr))
	if _, ok := dm["Linearized"]; ok {
		return true
	}
	_, hasS := dm["S"]
	_, hasI := dm["I"]
	_, hasLength := dm["Length"]
	_, hasType := dm["Type"]
	if hasS && hasI && hasLength && !hasType {
		return true
	}
	return false
}

func parseIntArray(s string) ([]int, bool) {
	v := strings.TrimSpace(s)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, false
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return []int{}, true
	}
	parts := strings.Fields(inner)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

func readBigEndianField(data []byte, pos *int, width int) (uint64, bool) {
	if width == 0 {
		return 0, true
	}
	if *pos+width > len(data) {
		return 0, false
	}
	var v uint64
	for i := 0; i < width; i++ {
		v = (v << 8) | uint64(data[*pos+i])
	}
	*pos += width
	return v, true
}

func paethPredictor(a, b, c byte) byte {
	pa := int(b) - int(c)
	pb := int(a) - int(c)
	pc := pa + pb
	if pc < 0 {
		pc = -pc
	}
	if pa < 0 {
		pa = -pa
	}
	if pb < 0 {
		pb = -pb
	}
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func decodePNGPredictor(data []byte, columns int) ([]byte, error) {
	if columns <= 0 {
		return nil, fmt.Errorf("invalid columns: %d", columns)
	}
	rowSize := columns + 1
	if len(data)%rowSize != 0 {
		return nil, fmt.Errorf("predictor row size mismatch: len=%d row=%d", len(data), rowSize)
	}
	rows := len(data) / rowSize
	out := make([]byte, rows*columns)
	for r := 0; r < rows; r++ {
		inRow := data[r*rowSize : (r+1)*rowSize]
		f := inRow[0]
		cur := inRow[1:]
		outRow := out[r*columns : (r+1)*columns]
		var prev []byte
		if r > 0 {
			prev = out[(r-1)*columns : r*columns]
		}
		switch f {
		case 0:
			copy(outRow, cur)
		case 1:
			for i := 0; i < columns; i++ {
				left := byte(0)
				if i > 0 {
					left = outRow[i-1]
				}
				outRow[i] = cur[i] + left
			}
		case 2:
			for i := 0; i < columns; i++ {
				up := byte(0)
				if prev != nil {
					up = prev[i]
				}
				outRow[i] = cur[i] + up
			}
		case 3:
			for i := 0; i < columns; i++ {
				left := byte(0)
				up := byte(0)
				if i > 0 {
					left = outRow[i-1]
				}
				if prev != nil {
					up = prev[i]
				}
				outRow[i] = cur[i] + byte((int(left)+int(up))/2)
			}
		case 4:
			for i := 0; i < columns; i++ {
				left := byte(0)
				up := byte(0)
				upLeft := byte(0)
				if i > 0 {
					left = outRow[i-1]
				}
				if prev != nil {
					up = prev[i]
					if i > 0 {
						upLeft = prev[i-1]
					}
				}
				outRow[i] = cur[i] + paethPredictor(left, up, upLeft)
			}
		default:
			return nil, fmt.Errorf("unsupported PNG predictor filter: %d", f)
		}
	}
	return out, nil
}

func formatXRefStream(decoded []byte, dictStr string) string {
	dm := dictEntriesMap(strings.TrimSpace(dictStr))

	if dpRaw, hasDecodeParms := dm["DecodeParms"]; hasDecodeParms {
		dp := dictEntriesMap(dpRaw)
		predictor := 1
		columns := 0
		if p, err := strconv.Atoi(dp["Predictor"]); err == nil {
			predictor = p
		}
		if c, err := strconv.Atoi(dp["Columns"]); err == nil {
			columns = c
		}
		if predictor >= 10 {
			if columns <= 0 {
				columns = 1
			}
			dec, err := decodePNGPredictor(decoded, columns)
			if err != nil {
				return fmt.Sprintf("XRef decode: predictor decode failed: %v\n", err)
			}
			decoded = dec
		}
	}

	wArr, ok := parseIntArray(dm["W"])
	if !ok || len(wArr) != 3 {
		return "XRef decode: missing or invalid /W array\n"
	}
	w0, w1, w2 := wArr[0], wArr[1], wArr[2]
	entryWidth := w0 + w1 + w2
	if entryWidth <= 0 {
		return "XRef decode: invalid entry width\n"
	}

	index := []int{}
	if idxRaw, hasIndex := dm["Index"]; hasIndex {
		idxArr, ok := parseIntArray(idxRaw)
		if !ok || len(idxArr)%2 != 0 {
			return "XRef decode: invalid /Index array\n"
		}
		index = idxArr
	} else {
		size, err := strconv.Atoi(dm["Size"])
		if err != nil {
			return "XRef decode: missing /Index and invalid /Size\n"
		}
		index = []int{0, size}
	}

	totalEntries := 0
	for i := 0; i < len(index); i += 2 {
		totalEntries += index[i+1]
	}
	required := totalEntries * entryWidth
	if len(decoded) < required {
		return fmt.Sprintf("XRef decode: stream too short (%d < %d)\n", len(decoded), required)
	}

	var b strings.Builder
	b.WriteString("Decoded XRef entries:\n")
	b.WriteString(fmt.Sprintf("- W: [%d %d %d]\n", w0, w1, w2))
	b.WriteString("- Index ranges:\n")
	for i := 0; i < len(index); i += 2 {
		start, count := index[i], index[i+1]
		b.WriteString(fmt.Sprintf("  - [%d .. %d] (%d entries)\n", start, start+count-1, count))
	}
	b.WriteString("- Entries:\n")

	pos := 0
	for i := 0; i < len(index); i += 2 {
		startObj, count := index[i], index[i+1]
		for j := 0; j < count; j++ {
			f0, ok0 := readBigEndianField(decoded, &pos, w0)
			f1, ok1 := readBigEndianField(decoded, &pos, w1)
			f2, ok2 := readBigEndianField(decoded, &pos, w2)
			if !ok0 || !ok1 || !ok2 {
				b.WriteString("  - [decode error: truncated entry]\n")
				return b.String()
			}

			typeField := f0
			if w0 == 0 {
				typeField = 1
			}

			objNum := startObj + j
			switch typeField {
			case 0:
				b.WriteString(fmt.Sprintf("  - obj %d: free, next=%d gen=%d\n", objNum, f1, f2))
			case 1:
				b.WriteString(fmt.Sprintf("  - obj %d: in-use, offset=%d gen=%d\n", objNum, f1, f2))
			case 2:
				b.WriteString(fmt.Sprintf("  - obj %d: compressed, objstm=%d index=%d\n", objNum, f1, f2))
			default:
				b.WriteString(fmt.Sprintf("  - obj %d: type=%d field2=%d field3=%d\n", objNum, typeField, f1, f2))
			}
		}
	}

	if len(decoded) > required {
		b.WriteString(fmt.Sprintf("- Note: %d trailing byte(s) after xref entries\n", len(decoded)-required))
	}
	return b.String()
}

func parseIntValue(raw string) (int, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func isHintBinaryStream(dictStr string) bool {
	dm := dictEntriesMap(strings.TrimSpace(dictStr))
	_, hasS := parseIntValue(dm["S"])
	_, hasI := parseIntValue(dm["I"])
	_, hasFilter := dm["Filter"]
	_, hasType := dm["Type"]
	return hasS && hasI && hasFilter && !hasType
}

func hexDump(data []byte, bytesPerLine, maxBytes int) string {
	if len(data) == 0 {
		return "(empty)"
	}
	if bytesPerLine <= 0 {
		bytesPerLine = 16
	}
	limit := len(data)
	truncated := false
	if maxBytes > 0 && limit > maxBytes {
		limit = maxBytes
		truncated = true
	}

	var b strings.Builder
	for off := 0; off < limit; off += bytesPerLine {
		end := off + bytesPerLine
		if end > limit {
			end = limit
		}
		line := data[off:end]
		b.WriteString(fmt.Sprintf("%04x: ", off))
		for i := 0; i < bytesPerLine; i++ {
			if off+i < end {
				b.WriteString(fmt.Sprintf("%02x ", line[i]))
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" |")
		for _, c := range line {
			if c >= 32 && c <= 126 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	if truncated {
		b.WriteString(fmt.Sprintf("... (%d bytes omitted)\n", len(data)-limit))
	}
	return b.String()
}

func formatHintStream(decoded []byte, dictStr string) string {
	dm := dictEntriesMap(strings.TrimSpace(dictStr))
	s, okS := parseIntValue(dm["S"])
	i, okI := parseIntValue(dm["I"])
	if !okS || !okI {
		return formatStreamContent(decoded)
	}
	if s < 0 {
		s = 0
	}
	if i < 0 {
		i = 0
	}
	if s > len(decoded) {
		s = len(decoded)
	}
	if i > len(decoded) {
		i = len(decoded)
	}
	if i < s {
		i = s
	}

	secA := decoded[:s]
	secB := decoded[s:i]
	secC := decoded[i:]

	var b strings.Builder
	b.WriteString("Decoded hint stream (heuristic):\n")
	b.WriteString(fmt.Sprintf("- total bytes: %d\n", len(decoded)))
	b.WriteString(fmt.Sprintf("- /S boundary: %d\n", s))
	b.WriteString(fmt.Sprintf("- /I boundary: %d\n", i))
	b.WriteString("- Sections:\n")
	b.WriteString(fmt.Sprintf("  - A [0:%d] (%d bytes)\n", s, len(secA)))
	b.WriteString(fmt.Sprintf("  - B [%d:%d] (%d bytes)\n", s, i, len(secB)))
	b.WriteString(fmt.Sprintf("  - C [%d:%d] (%d bytes)\n", i, len(decoded), len(secC)))

	b.WriteString("- A hex dump:\n")
	b.WriteString(hexDump(secA, 16, 128))
	if len(secB) > 0 {
		b.WriteString("- B hex dump:\n")
		b.WriteString(hexDump(secB, 16, 128))
	}
	if len(secC) > 0 {
		b.WriteString("- C hex dump:\n")
		b.WriteString(hexDump(secC, 16, 128))
	}
	return b.String()
}

func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	nonPrintable := 0
	for _, b := range data {
		if (b < 32 && b != 9 && b != 10 && b != 13) || b > 126 {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(len(data)) > 0.3
}

func formatStreamContent(content []byte) string {
	if isBinaryContent(content) {
		return fmt.Sprintf("[binary data, %d bytes]", len(content))
	}

	text := string(content)
	if strings.Contains(text, ">><") || strings.Contains(text, ">> <") {
		text = strings.ReplaceAll(text, ">><<", ">>\n<<")
		text = strings.ReplaceAll(text, ">><", ">>\n<")
		text = regexp.MustCompile(`>>\s*<<`).ReplaceAllString(text, ">>\n<<")
	}

	objStreamRe := regexp.MustCompile(`(\d+\s+\d+)\s*(<<)`)
	text = objStreamRe.ReplaceAllString(text, "$1\n$2")

	lines := strings.Split(text, "\n")
	var result strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		isPdfCommands := regexp.MustCompile(`\s+(m|l|c|v|y|re|S|s|f|F|f\*|B|B\*|b|b\*|n|W|W\*|q|Q|cm|sh|SCN|scn|rg|RG|g|G|Do|Tf|Td|TD|Tm|T\*|Tj|TJ|'|"|d0|d1|ri|i|gs|CS|cs|SC|sc|BT|ET|BDC|BMC|EMC|DP|MP|h|w|J|j|M|d|k|K)\s*$`).MatchString

		if isPdfCommands(" " + trimmed + " ") {
			formatted := formatPdfCommands(trimmed)
			for _, cmdLine := range strings.Split(formatted, "\n") {
				if strings.TrimSpace(cmdLine) != "" {
					clean := strings.TrimSpace(cmdLine)
					result.WriteString(clean)
					result.WriteString(commandCommentary(clean))
					result.WriteString("\n")
				}
			}
		} else {
			if strings.HasPrefix(trimmed, "<<") && strings.HasSuffix(trimmed, ">>") {
				pretty := formatDictionary(trimmed)
				for _, dl := range strings.Split(pretty, "\n") {
					result.WriteString(strings.TrimRight(dl, "\r\n"))
					result.WriteString("\n")
				}
			} else {
				result.WriteString(trimmed)
				result.WriteString("\n")
			}
		}
	}
	return result.String()
}

func indentLines(text, indent string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatObjectStreamContent(content []byte) string {
	text := string(content)
	matches := regexp.MustCompile(`^\s*((?:\d+\s+\d+\s+)+)(.*)$`).FindStringSubmatch(text)
	if len(matches) != 3 {
		return formatStreamContent(content)
	}

	indexPart := strings.Fields(matches[1])
	contentPart := strings.TrimSpace(matches[2])
	objIDs := make([]string, 0, len(indexPart)/2)
	for i := 0; i+1 < len(indexPart); i += 2 {
		objIDs = append(objIDs, indexPart[i])
	}

	var b strings.Builder
	b.WriteString("Object stream index:\n")
	for i := 0; i+1 < len(indexPart); i += 2 {
		b.WriteString(fmt.Sprintf("- obj %s @ %s\n", indexPart[i], indexPart[i+1]))
	}

	b.WriteString("Embedded object payload:\n")
	pos := 0
	objIdx := 0
	for pos < len(contentPart) {
		for pos < len(contentPart) && isSpace(contentPart[pos]) {
			pos++
		}
		if pos >= len(contentPart) {
			break
		}
		tok, next := parsePDFObjectToken(contentPart, pos)
		if next <= pos || strings.TrimSpace(tok) == "" {
			break
		}
		trimTok := strings.TrimSpace(tok)
		if strings.HasPrefix(trimTok, "<<") && strings.HasSuffix(trimTok, ">>") {
			pretty := formatDictionary(trimTok)
			if objIdx < len(objIDs) {
				pretty = annotateDictStartWithObjectID(pretty, objIDs[objIdx])
			}
			b.WriteString(pretty)
			b.WriteString("\n")
			objIdx++
		} else {
			b.WriteString(formatStreamContent([]byte(trimTok)))
		}
		pos = next
	}
	return b.String()
}

func formatPdfCommands(text string) string {
	operators := []string{
		"m", "l", "c", "v", "y", "re", "S", "s", "f", "F", "f*",
		"B", "B*", "b", "b*", "n", "W", "W*", "q", "Q", "cm", "sh",
		"SCN", "scn", "rg", "RG", "g", "G", "Do", "Tf", "Td", "TD",
		"Tm", "T*", "Tj", "TJ", "'", "\"", "d0", "d1", "ri", "i",
		"gs", "CS", "cs", "SC", "sc", "BT", "ET", "BDC", "BMC", "EMC",
		"DP", "MP", "h", "w", "J", "j", "M", "d", "k", "K",
	}

	result := text
	for _, op := range operators {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(op) + `\b(\s+)`)
		result = re.ReplaceAllString(result, op+"\n")
	}
	result = regexp.MustCompile(`\n\n+`).ReplaceAllString(result, "\n")
	return result
}

// DumpPDF reads a PDF file and prints its structure with sophisticated parsing.
func DumpPDF(pdfPath string) error {
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF: %w", err)
	}

	fmt.Printf("PDF Stream Dump: %s\n", pdfPath)
	fmt.Println(strings.Repeat("=", 80))

	matches := objRe.FindAllSubmatch(data, -1)

	for _, match := range matches {
		objNum := string(match[1])
		objBody := match[2]

		streamMatch := streamRe.FindSubmatch(objBody)
		if streamMatch == nil {
			dictBody := strings.TrimSpace(string(objBody))
			optional := isOptionalObject(dictBody)
			if len(dictBody) > 500 {
				dictBody = dictBody[:500] + "..."
			}
			if !isBinaryContent([]byte(dictBody)) && len(dictBody) < 2000 {
				fmt.Printf("\n%s\n", objectHeader(objNum, optional))
				fmt.Println("  Kind: no stream")
				fmt.Println(indentLines(formatDictionary(dictBody), "  "))
			}
			continue
		}

		streamData := streamMatch[1]
		streamData = bytes.TrimPrefix(streamData, []byte("\r\n"))
		streamData = bytes.TrimPrefix(streamData, []byte("\n"))
		streamData = bytes.TrimPrefix(streamData, []byte("\r"))
		streamData = bytes.TrimRight(streamData, "\r\n \t")

		dictPart := objBody[:bytes.Index(objBody, []byte("stream"))]
		dictStr := strings.TrimSpace(string(dictPart))
		optional := isOptionalObject(dictStr)
		isHint := isHintBinaryStream(dictStr)

		fmt.Printf("\n%s\n", objectHeader(objNum, optional))
		if isHint {
			fmt.Println("  Kind: Hint stream")
		}
		fmt.Println("  Dictionary:")
		fmt.Print(indentLines(annotateDictStartWithObjectID(formatDictionary(dictStr), objNum), "    "))

		var decodedData []byte
		if bytes.Contains(dictPart, []byte("FlateDecode")) {
			decodedData, err = decompressFlate(streamData)
			if err != nil {
				fmt.Printf("  Stream: [failed to decompress: %v, raw %d bytes]\n", err, len(streamData))
				continue
			}
			fmt.Printf("  Stream (decompressed %d -> %d bytes):\n", len(streamData), len(decodedData))
		} else {
			decodedData = streamData
			fmt.Printf("  Stream (%d bytes, uncompressed):\n", len(decodedData))
		}

		formatted := ""
		if bytes.Contains(dictPart, []byte("/ObjStm")) || bytes.Contains(dictPart, []byte("/Type/ObjStm")) {
			formatted = formatObjectStreamContent(decodedData)
		} else if bytes.Contains(dictPart, []byte("/Type /XRef")) || bytes.Contains(dictPart, []byte("/Type/XRef")) {
			formatted = formatXRefStream(decodedData, dictStr)
		} else if isHint {
			formatted = formatHintStream(decodedData, dictStr)
		} else {
			formatted = formatStreamContent(decodedData)
		}
		if formatted != "" {
			fmt.Print(indentLines(formatted, "    "))
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Printf("Total objects processed: %d\n", len(matches))
	return nil
}
