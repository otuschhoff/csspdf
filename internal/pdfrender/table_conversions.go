package pdfrender

import "strings"

// tableHexToRGB parses a CSS hex color string and returns r, g, b in [0,255].
func tableHexToRGB(hex string) (int, int, int) {
	hex = strings.ToLower(strings.TrimSpace(hex))
	if hex == "" {
		return 0, 0, 0
	}
	if value, ok := tableNamedColors[hex]; ok {
		return value.red, value.green, value.blue
	}
	if hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) == 3 {
		red, okRed := tableHexNibble(hex[0])
		green, okGreen := tableHexNibble(hex[1])
		blue, okBlue := tableHexNibble(hex[2])
		if okRed && okGreen && okBlue {
			return red * 17, green * 17, blue * 17
		}
	}
	if len(hex) == 6 {
		red, okRed := tableHexByte(hex[0], hex[1])
		green, okGreen := tableHexByte(hex[2], hex[3])
		blue, okBlue := tableHexByte(hex[4], hex[5])
		if okRed && okGreen && okBlue {
			return red, green, blue
		}
	}
	return 0, 0, 0
}

func (tr *TableRenderer) tableColor(color string) (int, int, int) {
	key := strings.ToLower(strings.TrimSpace(color))
	if cached, ok := tr.colorCache[key]; ok {
		return cached.red, cached.green, cached.blue
	}
	red, green, blue := tableHexToRGB(key)
	tr.colorCache[key] = tableRGB{red: red, green: green, blue: blue}
	return red, green, blue
}

func tableHexByte(high, low byte) (int, bool) {
	highValue, highOK := tableHexNibble(high)
	lowValue, lowOK := tableHexNibble(low)
	return highValue*16 + lowValue, highOK && lowOK
}

func tableHexNibble(value byte) (int, bool) {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0'), true
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10, true
	}
	return 0, false
}

var tableNamedColors = map[string]tableRGB{
	"black": {0, 0, 0}, "white": {255, 255, 255}, "gray": {128, 128, 128}, "grey": {128, 128, 128},
	"lightgray": {211, 211, 211}, "lightgrey": {211, 211, 211}, "darkgray": {169, 169, 169}, "darkgrey": {169, 169, 169},
	"red": {255, 0, 0}, "green": {0, 128, 0}, "blue": {0, 0, 255},
}

// tableEncodePDFTextLatin1 converts a UTF-8 string to a CP-1252/Latin-1 byte
// string suitable for gofpdf's standard fonts.
func tableEncodePDFTextLatin1(text string) string {
	if text == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(text))
	for _, character := range text {
		switch {
		case character == '\n' || character == '\r' || character == '\t':
			builder.WriteRune(character)
		case character <= 0xFF:
			builder.WriteByte(byte(character))
		default:
			if encoded, ok := tableCP1252[character]; ok {
				builder.WriteByte(encoded)
			} else {
				builder.WriteByte('?')
			}
		}
	}
	return builder.String()
}

var tableCP1252 = map[rune]byte{
	8364: 0x80, 8218: 0x82, 8222: 0x84, 8230: 0x85, 8224: 0x86, 8225: 0x87,
	710: 0x88, 8240: 0x89, 352: 0x8A, 8249: 0x8B, 338: 0x8C, 381: 0x8E,
	8216: 0x91, 8217: 0x92, 8220: 0x93, 8221: 0x94,
	8226: 0x95, 8211: 0x96, 8212: 0x97, 732: 0x98, 8482: 0x99,
	353: 0x9A, 8250: 0x9B, 339: 0x9C, 382: 0x9E, 376: 0x9F,
}
