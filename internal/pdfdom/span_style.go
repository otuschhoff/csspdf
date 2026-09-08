package pdfdom

import (
	"fmt"
	"math"
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

func htmlBuildSpan(n *html.Node, options ParseOptions) (*PDFTextNode, error) {
	style := &PDFTextStyle{}
	fontStyle, fontWeight, fontStyleSet := "", "", false
	for _, attribute := range n.Attr {
		handled, err := applySpanFontAttribute(style, attribute)
		if err != nil {
			err = fmt.Errorf("invalid span attribute %s=%q: %w", attribute.Key, attribute.Val, err)
			if !options.AllowInvalidSpanAttributes {
				return nil, err
			}
			if options.Warnf != nil {
				options.Warnf("ignored %v", err)
			}
			continue
		}
		if handled {
			if attribute.Key == "font-style" {
				fontStyle = attribute.Val
			}
			if attribute.Key == "font-weight" {
				fontWeight = attribute.Val
			}
			fontStyleSet = fontStyleSet || attribute.Key == "font-style" || attribute.Key == "font-weight"
			continue
		}
		if err := applySpanBoxAttribute(style, attribute); err != nil {
			err = fmt.Errorf("invalid span attribute %s=%q: %w", attribute.Key, attribute.Val, err)
			if !options.AllowInvalidSpanAttributes {
				return nil, err
			}
			if options.Warnf != nil {
				options.Warnf("ignored %v", err)
			}
		}
	}
	if fontStyleSet {
		style.FontStyle = htmlNormaliseFontStyle(fontStyle + " " + fontWeight)
		style.FontStyleSet = true
	}
	if emptyTextStyle(style) {
		style = nil
	}
	return &PDFTextNode{Text: tmpl.CollectText(n), Style: style}, nil
}

func applySpanFontAttribute(style *PDFTextStyle, attribute html.Attribute) (bool, error) {
	switch attribute.Key {
	case "font-style", "font-weight":
		return true, nil
	case "font-face":
		style.FontFace = attribute.Val
	case "font-size":
		value, ok := tmpl.ParseLengthValue(attribute.Val)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return true, fmt.Errorf("font-size must be a positive finite length")
		}
		style.FontSize = value
	case "font-color":
		style.FontColor = attribute.Val
	case "align":
		style.Align = htmlNormaliseTextAlign(attribute.Val)
	default:
		return false, nil
	}
	return true, nil
}

func applySpanBoxAttribute(style *PDFTextStyle, attribute html.Attribute) error {
	switch attribute.Key {
	case "background-color", "backgroundColor":
		style.BackgroundColor = strings.TrimSpace(attribute.Val)
	case "border-color", "borderColor":
		style.BorderColor = strings.TrimSpace(attribute.Val)
	case "border-style", "borderStyle":
		style.BorderStyle = strings.ToLower(strings.TrimSpace(attribute.Val))
	case "border-width", "borderWidth":
		value, ok := tmpl.ParseLengthValue(attribute.Val)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("border-width must be a non-negative finite length")
		}
		style.BorderWidth = value
	case "border":
		applySpanBorder(style, attribute.Val)
	}
	return nil
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
