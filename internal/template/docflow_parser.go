package template

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
	doc, err := html.Parse(strings.NewReader("<html><body>" + htmlStr + "</body></html>"))
	if err != nil {
		return nil, fmt.Errorf("html parse: %w", err)
	}
	if err := ApplyStylesheet(doc, cssText); err != nil {
		return nil, err
	}
	return doc, nil
}

func ApplyStylesheet(root *html.Node, cssText string) error {
	if strings.TrimSpace(cssText) == "" {
		return nil
	}

	inlineAttrs := CaptureAttrNames(root)
	sheet, err := parser.Parse(cssText)
	if err != nil {
		return fmt.Errorf("css parse: %w", err)
	}

	for _, rule := range sheet.Rules {
		if len(rule.Selectors) == 0 || len(rule.Declarations) == 0 {
			continue
		}
		for _, selectorText := range rule.Selectors {
			selector, err := cascadia.ParseGroup(selectorText)
			if err != nil {
				return fmt.Errorf("css selector parse %q: %w", selectorText, err)
			}
			for _, node := range cascadia.QueryAll(root, selector) {
				if node.Type != html.ElementNode {
					continue
				}
				for _, decl := range rule.Declarations {
					ApplyCSSDeclaration(node, decl, inlineAttrs[node])
				}
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

	switch property {
	case "text-align":
		return "align", strings.ToLower(value), true
	case "position":
		return "position", strings.ToLower(value), true
	case "top":
		return "top", value, true
	case "right":
		return "right", value, true
	case "bottom":
		return "bottom", value, true
	case "left":
		return "left", value, true
	case "width":
		return "width", value, true
	case "height":
		return "height", value, true
	case "fill":
		return "fill", value, true
	case "border":
		return "border", value, true
	case "border-width":
		return "border-width", value, true
	case "border-style":
		return "border-style", value, true
	case "border-color":
		return "border-color", value, true
	case "padding":
		return "padding", value, true
	case "padding-top":
		return "padding-top", value, true
	case "padding-right":
		return "padding-right", value, true
	case "padding-bottom":
		return "padding-bottom", value, true
	case "padding-left":
		return "padding-left", value, true
	case "font-family":
		return "font-face", value, true
	case "font-size":
		return "font-size", value, true
	case "color":
		return "font-color", value, true
	case "font-style":
		return "font-style", value, true
	case "font-weight":
		if strings.EqualFold(value, "bold") || value == "700" {
			return "font-style", "bold", true
		}
		if strings.EqualFold(value, "normal") || value == "400" {
			return "font-style", "normal", true
		}
	case "background-color":
		return "background-color", value, true
	case "margin-top":
		return "margin-top", value, true
	case "margin-bottom":
		return "margin-bottom", value, true
	case "break-before":
		return "break-before", strings.ToLower(value), true
	case "break-after":
		return "break-after", strings.ToLower(value), true
	}

	return "", "", false
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
