package templating

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	css "github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
	"golang.org/x/net/html"
)

func ParseStyledFragment(htmlStr, cssText string) (*html.Node, error) {
	stylesheet, err := PrepareStylesheet(cssText)
	if err != nil {
		return nil, err
	}
	return ParsePreparedStyledFragment(htmlStr, stylesheet)
}

type PreparedStylesheet struct {
	rules []preparedCSSRule
}

type preparedCSSRule struct {
	selector     cascadia.SelectorGroup
	declarations []*css.Declaration
}

func PrepareStylesheet(cssText string) (*PreparedStylesheet, error) {
	prepared := &PreparedStylesheet{}
	if strings.TrimSpace(cssText) == "" {
		return prepared, nil
	}
	sheet, err := parser.Parse(cssText)
	if err != nil {
		return nil, fmt.Errorf("css parse: %w", err)
	}
	for _, rule := range sheet.Rules {
		if len(rule.Selectors) == 0 || len(rule.Declarations) == 0 {
			continue
		}
		for _, selectorText := range rule.Selectors {
			selector, err := cascadia.ParseGroup(selectorText)
			if err != nil {
				return nil, fmt.Errorf("css selector parse %q: %w", selectorText, err)
			}
			prepared.rules = append(prepared.rules, preparedCSSRule{selector: selector, declarations: rule.Declarations})
		}
	}
	return prepared, nil
}

func ParsePreparedStyledFragment(htmlStr string, stylesheet *PreparedStylesheet) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader("<html><body>" + htmlStr + "</body></html>"))
	if err != nil {
		return nil, fmt.Errorf("html parse: %w", err)
	}
	if err := ApplyPreparedStylesheet(doc, stylesheet); err != nil {
		return nil, err
	}
	return doc, nil
}

func ApplyStylesheet(root *html.Node, cssText string) error {
	stylesheet, err := PrepareStylesheet(cssText)
	if err != nil {
		return err
	}
	return ApplyPreparedStylesheet(root, stylesheet)
}

func ApplyPreparedStylesheet(root *html.Node, stylesheet *PreparedStylesheet) error {
	if stylesheet == nil || len(stylesheet.rules) == 0 {
		return nil
	}
	inlineAttrs := CaptureAttrNames(root)
	for _, rule := range stylesheet.rules {
		for _, node := range cascadia.QueryAll(root, rule.selector) {
			if node.Type != html.ElementNode {
				continue
			}
			for _, declaration := range rule.declarations {
				ApplyCSSDeclaration(node, declaration, inlineAttrs[node])
			}
		}
	}

	return nil
}

func CaptureAttrNames(root *html.Node) map[*html.Node]map[string]struct{} {
	attrs := make(map[*html.Node]map[string]struct{})
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			names := make(map[string]struct{}, len(n.Attr))
			for _, attr := range n.Attr {
				names[attr.Key] = struct{}{}
			}
			attrs[n] = names
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return attrs
}

func ApplyCSSDeclaration(node *html.Node, decl *css.Declaration, inlineAttrs map[string]struct{}) {
	if node == nil || decl == nil {
		return
	}

	attrName, attrValue, ok := CSSDeclarationToAttr(decl)
	if !ok {
		return
	}
	if _, exists := inlineAttrs[attrName]; exists {
		return
	}
	SetOrReplaceAttr(node, attrName, attrValue)
}

func CSSDeclarationToAttr(decl *css.Declaration) (string, string, bool) {
	property := strings.ToLower(strings.TrimSpace(decl.Property))
	value := strings.TrimSpace(decl.Value)
	if mapping, ok := cssAttributeMappings[property]; ok {
		if mapping.lowercase {
			value = strings.ToLower(value)
		}
		return mapping.attribute, value, true
	}
	if property == "font-weight" {
		switch strings.ToLower(value) {
		case "bold", "700":
			return "font-weight", "bold", true
		case "normal", "400":
			return "font-weight", "normal", true
		}
	}
	return "", "", false
}

type cssAttributeMapping struct {
	attribute string
	lowercase bool
}

var cssAttributeMappings = map[string]cssAttributeMapping{
	"text-align": {attribute: "align", lowercase: true},
	"position":   {attribute: "position", lowercase: true},
	"top":        {attribute: "top"}, "right": {attribute: "right"}, "bottom": {attribute: "bottom"},
	"left": {attribute: "left"}, "width": {attribute: "width"}, "height": {attribute: "height"},
	"fill": {attribute: "fill"}, "border": {attribute: "border"}, "border-width": {attribute: "border-width"},
	"border-style": {attribute: "border-style"}, "border-color": {attribute: "border-color"},
	"padding": {attribute: "padding"}, "padding-top": {attribute: "padding-top"},
	"padding-right": {attribute: "padding-right"}, "padding-bottom": {attribute: "padding-bottom"},
	"padding-left": {attribute: "padding-left"}, "font-family": {attribute: "font-face"},
	"font-size": {attribute: "font-size"}, "color": {attribute: "font-color"},
	"font-style": {attribute: "font-style"}, "background-color": {attribute: "background-color"},
	"margin-top": {attribute: "margin-top"}, "margin-bottom": {attribute: "margin-bottom"},
	"break-before": {attribute: "break-before", lowercase: true},
	"break-after":  {attribute: "break-after", lowercase: true},
	"white-space":  {attribute: "white-space", lowercase: true},
	"table-layout": {attribute: "table-layout", lowercase: true},
}

func SetOrReplaceAttr(node *html.Node, key, value string) {
	for idx := range node.Attr {
		if node.Attr[idx].Key == key {
			node.Attr[idx].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func FindFirst(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := FindFirst(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func FindFirstByID(n *html.Node, tag, id string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag && AttrVal(n, "id") == id {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := FindFirstByID(c, tag, id); found != nil {
			return found
		}
	}
	return nil
}

func AttrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func ElemChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

func CollectText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return strings.TrimSpace(b.String())
}

func NormaliseInlineTextNode(raw string) string {
	replaced := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(raw)
	hasLeading := strings.HasPrefix(replaced, " ")
	hasTrailing := strings.HasSuffix(replaced, " ")
	parts := strings.Fields(replaced)
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, " ")
	if hasLeading {
		text = " " + text
	}
	if hasTrailing {
		text += " "
	}
	return text
}

func ParseLengthValue(raw string) (float64, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.TrimSuffix(v, "px")
	v = strings.TrimSuffix(v, "pt")
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func ParseBorderShorthand(value string) (width float64, style, color string) {
	for _, part := range strings.Fields(strings.ToLower(strings.TrimSpace(value))) {
		switch part {
		case "none", "solid", "dashed", "dotted", "double":
			style = part
		default:
			if f, ok := ParseLengthValue(part); ok {
				width = f
				continue
			}
			color = part
		}
	}
	return width, style, color
}
