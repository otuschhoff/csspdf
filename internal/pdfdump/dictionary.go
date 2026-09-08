package pdfdump

import (
	"sort"
	"strings"
)

func formatDictionary(dict string) string {
	d := strings.TrimSpace(dict)
	if !strings.HasPrefix(d, "<<") || !strings.HasSuffix(d, ">>") {
		return d
	}
	entries := parseDictEntries(d)
	if len(entries) == 0 {
		return d
	}
	var builder strings.Builder
	builder.WriteString("<<\n")
	for _, entry := range orderDictEntries(entries) {
		builder.WriteString("    /")
		builder.WriteString(entry.Key)
		if entry.Value != "" {
			builder.WriteString(" ")
			builder.WriteString(entry.Value)
		}
		builder.WriteString("\n")
	}
	builder.WriteString(">>")
	return builder.String()
}

type dictEntry struct {
	Key, Value string
	Index      int
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
	var entries []dictEntry
	for index, position := 0, 0; position < len(inner); index++ {
		position = skipSpaces(inner, position)
		if position >= len(inner) {
			break
		}
		if inner[position] != '/' {
			_, next := parsePDFObjectToken(inner, position)
			if next <= position {
				break
			}
			position = next
			index--
			continue
		}
		key, next := parseNameToken(inner, position)
		position = skipSpaces(inner, next)
		value := ""
		if position < len(inner) {
			if parsed, valueEnd := parsePDFObjectToken(inner, position); valueEnd > position {
				value = compactSpaces(parsed)
				position = valueEnd
			}
		}
		entries = append(entries, dictEntry{Key: key, Value: value, Index: index})
	}
	return entries
}

func isDictDelimiter(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '/', '[', ']', '<', '>', '(', ')':
		return true
	}
	return false
}

func parseNameToken(value string, start int) (string, int) {
	if start >= len(value) || value[start] != '/' {
		return "", start
	}
	end := start + 1
	for end < len(value) && !isDictDelimiter(value[end]) {
		end++
	}
	return value[start+1 : end], end
}

func parsePDFObjectToken(value string, start int) (string, int) {
	if start >= len(value) {
		return "", start
	}
	if value[start] == '<' && start+1 < len(value) && value[start+1] == '<' {
		return parseNestedDelimitedToken(value, start, "<<", ">>")
	}
	switch value[start] {
	case '[':
		return parseNestedDelimitedToken(value, start, "[", "]")
	case '(':
		return parseLiteralStringToken(value, start)
	case '<':
		return parseTerminatedToken(value, start, '>')
	case '/':
		_, end := parseNameToken(value, start)
		return value[start:end], end
	}
	return parseBareOrReferenceToken(value, start)
}

func parseNestedDelimitedToken(value string, start int, open, close string) (string, int) {
	depth, index := 1, start+len(open)
	for index < len(value) {
		if strings.HasPrefix(value[index:], open) {
			depth++
			index += len(open)
			continue
		}
		if strings.HasPrefix(value[index:], close) {
			depth--
			index += len(close)
			if depth == 0 {
				return value[start:index], index
			}
			continue
		}
		index++
	}
	return value[start:], len(value)
}

func parseLiteralStringToken(value string, start int) (string, int) {
	depth := 1
	for index := start + 1; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if value[index] == '(' {
			depth++
		}
		if value[index] == ')' {
			depth--
			if depth == 0 {
				return value[start : index+1], index + 1
			}
		}
	}
	return value[start:], len(value)
}

func parseTerminatedToken(value string, start int, terminator byte) (string, int) {
	end := start + 1
	for end < len(value) && value[end] != terminator {
		end++
	}
	if end < len(value) {
		end++
	}
	return value[start:end], end
}

func parseBareOrReferenceToken(value string, start int) (string, int) {
	end := start
	for end < len(value) && !isSpace(value[end]) {
		if strings.ContainsRune("/[]<>()", rune(value[end])) {
			break
		}
		end++
	}
	if end == start {
		return string(value[start]), start + 1
	}
	base := value[start:end]
	if reference, referenceEnd, ok := parseObjectReference(value, start, end, base); ok {
		return reference, referenceEnd
	}
	return base, end
}

func parseObjectReference(value string, start, end int, base string) (string, int, bool) {
	if !isIntegerToken(base) {
		return "", end, false
	}
	second, secondEnd := parseSimpleToken(value, skipSpaces(value, end))
	if !isIntegerToken(second) {
		return "", end, false
	}
	reference, referenceEnd := parseSimpleToken(value, skipSpaces(value, secondEnd))
	if reference != "R" {
		return "", end, false
	}
	return compactSpaces(value[start:referenceEnd]), referenceEnd, true
}

func skipSpaces(value string, index int) int {
	for index < len(value) && isSpace(value[index]) {
		index++
	}
	return index
}

func parseSimpleToken(value string, start int) (string, int) {
	if start >= len(value) {
		return "", start
	}
	end := start
	for end < len(value) && !isSpace(value[end]) {
		if strings.ContainsRune("/[]<>()", rune(value[end])) {
			break
		}
		end++
	}
	if end == start {
		return "", start
	}
	return value[start:end], end
}

func isIntegerToken(token string) bool {
	if token == "" {
		return false
	}
	start := 0
	if token[0] == '-' || token[0] == '+' {
		if len(token) == 1 {
			return false
		}
		start = 1
	}
	for index := start; index < len(token); index++ {
		if token[index] < '0' || token[index] > '9' {
			return false
		}
	}
	return true
}

func isSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}
func compactSpaces(value string) string { return strings.Join(strings.Fields(value), " ") }

func orderDictEntries(entries []dictEntry) []dictEntry {
	priority := map[string]int{
		"Type": 0, "Subtype": 1, "FunctionType": 2, "ShadingType": 3, "PatternType": 4,
		"ColorSpace": 5, "Domain": 6, "Bounds": 7, "Encode": 8, "Functions": 9, "C0": 10,
		"C1": 11, "N": 12, "Coords": 13, "Extend": 14, "Function": 15, "Shading": 16,
		"Matrix": 17, "Filter": 18, "DecodeParms": 19, "Length": 20, "First": 21,
	}
	result := append([]dictEntry(nil), entries...)
	sort.SliceStable(result, func(left, right int) bool {
		leftPriority, leftKnown := priority[result[left].Key]
		rightPriority, rightKnown := priority[result[right].Key]
		if leftKnown && rightKnown {
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			return result[left].Index < result[right].Index
		}
		if leftKnown {
			return true
		}
		if rightKnown {
			return false
		}
		return result[left].Index < result[right].Index
	})
	return result
}

func dictEntriesMap(dict string) map[string]string {
	result := map[string]string{}
	for _, entry := range parseDictEntries(dict) {
		result[entry.Key] = entry.Value
	}
	return result
}

func isOptionalObject(dict string) bool {
	entries := dictEntriesMap(strings.TrimSpace(dict))
	if _, ok := entries["Linearized"]; ok {
		return true
	}
	_, hasS := entries["S"]
	_, hasI := entries["I"]
	_, hasLength := entries["Length"]
	_, hasType := entries["Type"]
	return hasS && hasI && hasLength && !hasType
}
