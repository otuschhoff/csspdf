package pdfdump

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	objRe    = regexp.MustCompile(`(?s)(\d+)\s+0\s+obj\s*(.*?)\s*endobj`)
	streamRe = regexp.MustCompile(`(?s)stream\s*(.*?)\s*endstream`)
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
	state := textArrayState{}
	for index := 0; index < len(arrayContent); index++ {
		state.consume(arrayContent[index])
	}
	return strings.Join(state.parts, "")
}

type textArrayState struct {
	parts                     []string
	current                   strings.Builder
	inLiteral, inHex, escaped bool
}

func (state *textArrayState) consume(character byte) {
	if state.escaped {
		state.current.WriteByte(character)
		state.escaped = false
		return
	}
	if state.inLiteral {
		state.consumeLiteral(character)
		return
	}
	if state.inHex {
		state.consumeHex(character)
		return
	}
	switch character {
	case '(':
		state.inLiteral = true
		state.current.Reset()
	case '<':
		state.inHex = true
		state.current.Reset()
	}
}

func (state *textArrayState) consumeLiteral(character byte) {
	switch character {
	case '\\':
		state.current.WriteByte(character)
		state.escaped = true
	case '(':
		state.current.Reset()
	case ')':
		state.parts = append(state.parts, state.current.String())
		state.inLiteral = false
	default:
		state.current.WriteByte(character)
	}
}

func (state *textArrayState) consumeHex(character byte) {
	switch character {
	case '<':
		state.current.Reset()
	case '>':
		if decoded := decodeHexString(state.current.String()); decoded != "" {
			state.parts = append(state.parts, decoded)
		}
		state.inHex = false
	default:
		state.current.WriteByte(character)
	}
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

func decompressFlate(data []byte, maxBytes int64) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	reader := io.Reader(r)
	if maxBytes > 0 {
		reader = io.LimitReader(r, maxBytes+1)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(decoded)) > maxBytes {
		return nil, &InspectionLimitError{Stage: "decompressed stream", Limit: maxBytes, Actual: int64(len(decoded))}
	}
	return decoded, nil
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
		writeHexDumpLine(&b, data[off:end], off, bytesPerLine)
	}
	if truncated {
		b.WriteString(fmt.Sprintf("... (%d bytes omitted)\n", len(data)-limit))
	}
	return b.String()
}

func writeHexDumpLine(builder *strings.Builder, line []byte, offset, width int) {
	fmt.Fprintf(builder, "%04x: ", offset)
	for index := 0; index < width; index++ {
		if index < len(line) {
			fmt.Fprintf(builder, "%02x ", line[index])
		} else {
			builder.WriteString("   ")
		}
	}
	builder.WriteString(" |")
	for _, character := range line {
		if character >= 32 && character <= 126 {
			builder.WriteByte(character)
		} else {
			builder.WriteByte('.')
		}
	}
	builder.WriteString("|\n")
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
	objIDs := objectStreamIDs(indexPart)

	var b strings.Builder
	b.WriteString("Object stream index:\n")
	for i := 0; i+1 < len(indexPart); i += 2 {
		b.WriteString(fmt.Sprintf("- obj %s @ %s\n", indexPart[i], indexPart[i+1]))
	}

	b.WriteString("Embedded object payload:\n")
	b.WriteString(formatEmbeddedObjectStream(contentPart, objIDs))
	return b.String()
}

func objectStreamIDs(index []string) []string {
	ids := make([]string, 0, len(index)/2)
	for position := 0; position+1 < len(index); position += 2 {
		ids = append(ids, index[position])
	}
	return ids
}

func formatEmbeddedObjectStream(content string, objectIDs []string) string {
	var builder strings.Builder
	position := 0
	objectIndex := 0
	for position < len(content) {
		for position < len(content) && isSpace(content[position]) {
			position++
		}
		if position >= len(content) {
			break
		}
		token, next := parsePDFObjectToken(content, position)
		if next <= position || strings.TrimSpace(token) == "" {
			break
		}
		trimTok := strings.TrimSpace(token)
		if strings.HasPrefix(trimTok, "<<") && strings.HasSuffix(trimTok, ">>") {
			pretty := formatDictionary(trimTok)
			if objectIndex < len(objectIDs) {
				pretty = annotateDictStartWithObjectID(pretty, objectIDs[objectIndex])
			}
			builder.WriteString(pretty)
			builder.WriteString("\n")
			objectIndex++
		} else {
			builder.WriteString(formatStreamContent([]byte(trimTok)))
		}
		position = next
	}
	return builder.String()
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
