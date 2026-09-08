package pdfdom

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
)

func htmlNormaliseAttrKey(key string) string {
	if normalized, ok := normalizedHTMLAttributes[key]; ok {
		return normalized
	}
	return key
}

var normalizedHTMLAttributes = map[string]string{
	"row-height-min": "rowHeightMin", "padding-top": "paddingTop", "padding-right": "paddingRight",
	"padding-bottom": "paddingBottom", "padding-left": "paddingLeft", "background-color": "backgroundColor",
	"border-width": "borderWidth", "border-style": "borderStyle", "border-color": "borderColor",
	"margin-top": "marginTop", "margin-bottom": "marginBottom", "break-before": "breakBefore",
	"break-after": "breakAfter", "white-space": "whiteSpace", "table-layout": "tableLayout",
}

func htmlBuildSpan(n *html.Node) *PDFTextNode {
	style := &PDFTextStyle{}
	fontStyle, fontWeight, fontStyleSet := "", "", false
	for _, attribute := range n.Attr {
		if applySpanFontAttribute(style, attribute) {
			if attribute.Key == "font-style" {
				fontStyle = attribute.Val
			}
			if attribute.Key == "font-weight" {
				fontWeight = attribute.Val
			}
			fontStyleSet = fontStyleSet || attribute.Key == "font-style" || attribute.Key == "font-weight"
			continue
		}
		applySpanBoxAttribute(style, attribute)
	}
	if fontStyleSet {
		style.FontStyle = htmlNormaliseFontStyle(fontStyle + " " + fontWeight)
		style.FontStyleSet = true
	}
	if emptyTextStyle(style) {
		style = nil
	}
	return &PDFTextNode{Text: tmpl.CollectText(n), Style: style}
}

func applySpanFontAttribute(style *PDFTextStyle, attribute html.Attribute) bool {
	switch attribute.Key {
	case "font-style", "font-weight":
		return true
	case "font-face":
		style.FontFace = attribute.Val
	case "font-size":
		style.FontSize, _ = strconv.ParseFloat(attribute.Val, 64)
	case "font-color":
		style.FontColor = attribute.Val
	case "align":
		style.Align = htmlNormaliseTextAlign(attribute.Val)
	default:
		return false
	}
	return true
}

func applySpanBoxAttribute(style *PDFTextStyle, attribute html.Attribute) {
	switch attribute.Key {
	case "background-color", "backgroundColor":
		style.BackgroundColor = strings.TrimSpace(attribute.Val)
	case "border-color", "borderColor":
		style.BorderColor = strings.TrimSpace(attribute.Val)
	case "border-style", "borderStyle":
		style.BorderStyle = strings.ToLower(strings.TrimSpace(attribute.Val))
	case "border-width", "borderWidth":
		style.BorderWidth, _ = tmpl.ParseLengthValue(attribute.Val)
	case "border":
		applySpanBorder(style, attribute.Val)
	}
}

func applySpanBorder(style *PDFTextStyle, value string) {
	width, borderStyle, color := tmpl.ParseBorderShorthand(value)
	if width > 0 {
		style.BorderWidth = width
	}
	if borderStyle != "" {
		style.BorderStyle = borderStyle
	}
	if color != "" {
		style.BorderColor = color
	}
}

func emptyTextStyle(style *PDFTextStyle) bool {
	return style.FontFace == "" && style.FontStyle == "" && style.FontSize == 0 && style.FontColor == "" &&
		style.Align == "" && style.LineHeight == 0 && style.BackgroundColor == "" && style.BorderColor == "" &&
		style.BorderStyle == "" && style.BorderWidth == 0 && !style.FontStyleSet
}
