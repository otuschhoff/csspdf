package pdfdom

import "strings"

func htmlNormaliseTextAlign(value string) TextAlign {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "left", "l":
		return TextAlignLeft
	case "center", "c":
		return TextAlignCenter
	case "right", "r":
		return TextAlignRight
	default:
		return TextAlign(value)
	}
}
