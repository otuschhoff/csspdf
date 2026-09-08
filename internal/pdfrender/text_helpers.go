package pdfrender

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (e *PDFTextEngine) maximumTextWidth(lines []string) float64 {
	width := 0.0
	for _, line := range lines {
		width = pdfMax(width, e.pdf.GetStringWidth(line))
	}
	return width
}

func (e *PDFTextEngine) clipToWidth(text string, width float64) (string, bool) {
	if width <= 0 || text == "" || e.pdf.GetStringWidth(text) <= width {
		return text, false
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes)
		if e.pdf.GetStringWidth(candidate) <= width {
			return candidate, true
		}
	}
	return "", true
}

func (e *PDFTextEngine) resolveText(node *PDFTextNode) string {
	if node == nil {
		return ""
	}
	if node.I18nKey == "" {
		return node.Text
	}
	if e.i18n == nil {
		if node.Text != "" {
			return node.Text
		}
		return node.I18nKey
	}
	if len(node.I18nVars) > 0 {
		return e.i18n.TWithVars(node.I18nKey, node.I18nVars)
	}
	return e.i18n.T(node.I18nKey)
}

func (e *PDFTextEngine) applyStyle(style PDFTextStyle) {
	e.pdf.SetFont(style.FontFace, style.FontStyle, style.FontSize)
	if style.FontColor != "" {
		red, green, blue := hexToRGB(style.FontColor)
		e.pdf.SetTextColor(red, green, blue)
	}
}

func alignedX(align TextAlign, x, width, lineWidth float64) float64 {
	if width <= 0 {
		return x
	}
	switch align {
	case TextAlignRight:
		return x + width - lineWidth
	case TextAlignCenter:
		return x + (width-lineWidth)/2
	default:
		return x
	}
}

func resolveBox(box *PDFTextBox) PDFTextBox {
	if box != nil {
		return ensureTextBoxDefaults(*box)
	}
	return ensureTextBoxDefaults(PDFTextBox{Fit: TextFitWrap})
}

func ensureTextStyleDefaults(style PDFTextStyle) PDFTextStyle {
	if style.FontFace == "" {
		style.FontFace = "Helvetica"
	}
	if style.FontSize <= 0 {
		style.FontSize = 10
	}
	if style.FontColor == "" {
		style.FontColor = "#000"
	}
	if style.Align == "" {
		style.Align = TextAlignLeft
	}
	if style.LineHeight <= 0 {
		style.LineHeight = 1.2
	}
	return style
}

func ensureTextBoxDefaults(box PDFTextBox) PDFTextBox {
	if box.Fit == "" {
		box.Fit = TextFitWrap
	}
	return box
}

func htmlLengthToFloat(node PDFElementNode, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := node.Attribute(key)
		if !ok {
			continue
		}
		value := strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), "px"), "pt")
		if number, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return number
		}
	}
	return 0
}

func resolveImageElement(node PDFElementNode, fallbackWidth float64, searchDirs []string, deferResolution bool) (path string, width, height, marginTop, marginBottom float64, err error) {
	if node == nil {
		return "", 0, 0, 0, 0, fmt.Errorf("image node cannot be nil")
	}
	source, ok := node.Attribute("src")
	if !ok || strings.TrimSpace(source) == "" {
		return "", 0, 0, 0, 0, fmt.Errorf("img missing src attribute")
	}
	if deferResolution {
		path = strings.TrimSpace(source)
	} else if path, ok = resolveImagePath(source, searchDirs); !ok {
		return "", 0, 0, 0, 0, fmt.Errorf("image not found: %s", source)
	}
	width = htmlLengthToFloat(node, "width")
	if width <= 0 {
		width = fallbackWidth
	}
	height = htmlLengthToFloat(node, "height")
	if height <= 0 {
		height = width
	}
	if width <= 0 || height <= 0 {
		return "", 0, 0, 0, 0, fmt.Errorf("img %q requires width/height or a positive fallback width", source)
	}
	return path, width, height, htmlLengthToFloat(node, "marginTop", "margin-top"), htmlLengthToFloat(node, "marginBottom", "margin-bottom"), nil
}

func resolveImagePath(source string, searchDirs []string) (string, bool) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", false
	}
	candidates := imagePathCandidates(trimmed, searchDirs)
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func imagePathCandidates(source string, searchDirs []string) []string {
	candidates := []string{source}
	if filepath.IsAbs(source) {
		return candidates
	}
	base := filepath.Base(source)
	for _, directory := range searchDirs {
		if directory = strings.TrimSpace(directory); directory != "" {
			candidates = append(candidates, filepath.Join(directory, source), filepath.Join(directory, base))
		}
	}
	for _, prefix := range []string{"resources", "..", "../resources", "../..", "../../resources"} {
		candidates = append(candidates, filepath.Join(prefix, source), filepath.Join(prefix, base))
	}
	return candidates
}

func imageTypeFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "JPG"
	case ".gif":
		return "GIF"
	default:
		return "PNG"
	}
}

func encodePDFTextLatin1(text string) string {
	cp1252 := map[rune]byte{'€': 0x80, '‚': 0x82, '„': 0x84, '…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88, '‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C, 'Ž': 0x8E, '\u2018': 0x91, '\u2019': 0x92, '\u201C': 0x93, '\u201D': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, '˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B, 'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F}
	var builder strings.Builder
	builder.Grow(len(text))
	for _, character := range text {
		switch {
		case character == '\n' || character == '\r' || character == '\t':
			builder.WriteRune(character)
		case character <= 0xFF:
			builder.WriteByte(byte(character))
		case cp1252[character] != 0:
			builder.WriteByte(cp1252[character])
		default:
			if utf8.ValidRune(character) {
				builder.WriteByte('?')
			} else {
				builder.WriteByte('?')
			}
		}
	}
	return builder.String()
}

func hexToRGB(value string) (int, int, int) {
	value = strings.ToLower(strings.TrimSpace(value))
	if color, ok := namedRGBColors[value]; ok {
		return color[0], color[1], color[2]
	}
	value = strings.TrimPrefix(value, "#")
	var red, green, blue int
	if len(value) == 3 {
		fmt.Sscanf(value, "%1x%1x%1x", &red, &green, &blue)
		return red * 17, green * 17, blue * 17
	}
	if len(value) == 6 {
		fmt.Sscanf(value, "%02x%02x%02x", &red, &green, &blue)
	}
	return red, green, blue
}

var namedRGBColors = map[string][3]int{"black": {0, 0, 0}, "white": {255, 255, 255}, "gray": {128, 128, 128}, "grey": {128, 128, 128}, "lightgray": {211, 211, 211}, "lightgrey": {211, 211, 211}, "darkgray": {169, 169, 169}, "darkgrey": {169, 169, 169}, "red": {255, 0, 0}, "green": {0, 128, 0}, "blue": {0, 0, 255}, "yellow": {255, 255, 0}, "orange": {255, 165, 0}}

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

func htmlParseBorderShorthand(value string) (width float64, style, color string) {
	for _, part := range strings.Fields(strings.ToLower(strings.TrimSpace(value))) {
		switch part {
		case "none", "solid", "dashed", "dotted", "double":
			style = part
		default:
			number := strings.TrimSuffix(strings.TrimSuffix(part, "px"), "pt")
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64); err == nil {
				width = parsed
			} else {
				color = part
			}
		}
	}
	return width, style, color
}

func pdfMax(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
func pdfMaxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
