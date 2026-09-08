package pdfdom

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
	"golang.org/x/net/html"
)

// ParseHTMLTableElem parses an HTML fragment containing a <table> element and
// maps it to the same ElemTable / Elem* node tree used by the PDF rendering
// pipeline. cssStyle is applied before parsing (may be empty).
//
// Mapping rules:
//
//	<table>               → ElemTable       (all attributes forwarded)
//	<colgroup>            → ElemColgroup
//	<col>                 → ElemCol         (width attribute forwarded)
//	<thead>               → ElemThead
//	<tbody>               → ElemTbody
//	<tr>                  → ElemTr          (fill, stroke, height forwarded)
//	<td>                  → ElemTd          (align, colspan forwarded)
//	<th>                  → ElemTh          (align, colspan forwarded)
//	<span>                → PDFTextNode     (font-style/face/size/color/align
//	                                         become PDFTextStyle fields)
//	<currency-value>      → ElemCurrencyValue  ("v" attr → Value float64)
//	<date-value>          → ElemDateValue      ("value" attr → Value string;
//	                                             "long" attr forwarded)
//	<duration-value>      → ElemDurationValue  ("v" attr → Value float64)
//	<man-days-value>      → ElemManDaysValue   ("v" attr → Value float64;
//	                                             "unit" attr forwarded)
//
// Attribute name normalisation (HTML kebab-case → Elem* camelCase):
//
//	row-height-min → rowHeightMin
//
// Inside <td>, the first non-empty child is added inline (Add); each
// subsequent child is added with a preceding line break (AddLine), matching
// the main-text / sub-text rendering idiom.
func ParseHTMLTableElem(htmlStr, cssStyle string) (*ElemTable, error) {
	// Wrap in a minimal document to give the parser a well-defined context.
	doc, err := tmpl.ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}

	tableNode := tmpl.FindFirst(doc, "table")
	if tableNode == nil {
		return nil, fmt.Errorf("no <table> element found in HTML fragment")
	}

	return htmlBuildTable(tableNode, ParseOptions{})
}

// ParseHTMLSectionElem parses an HTML fragment containing a
// <div id="intro"> element with <span>, <div> and <br> children and maps it
// to an ElemDiv. cssStyle is applied before parsing (may be empty).
func ParseHTMLSectionElem(htmlStr, cssStyle string) (*ElemDiv, error) {
	doc, err := tmpl.ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}

	introNode := tmpl.FindFirstByID(doc, "div", "intro")
	if introNode == nil {
		return nil, fmt.Errorf("no <div id=\"intro\"> element found in HTML fragment")
	}

	return htmlBuildSectionDiv(introNode, ParseOptions{})
}

// ParseHTMLDocFlow parses an HTML fragment into top-level renderable elements
// in source order. cssStyle is applied before parsing (may be empty).
// Supported root-level tags are <div>, <footer>, <p>, <table>, <img>, and
// headings <h1>..<h3>.
func ParseHTMLDocFlow(htmlStr, cssStyle string) ([]PDFElementNode, error) {
	doc, err := tmpl.ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}
	return parseHTMLDocFlow(doc, ParseOptions{})
}

func htmlBuildRootElement(node *html.Node, options ParseOptions) (PDFElementNode, bool, error) {
	var element PDFElementNode
	var err error
	switch node.Data {
	case "div", "footer":
		element, err = htmlBuildSectionDiv(node, options)
	case "p":
		element, err = htmlBuildParagraph(node, nil, options)
	case "h1", "h2", "h3":
		element, err = htmlBuildHeadingWithInherited(node, nil, options)
	case "table":
		element, err = htmlBuildTable(node, options)
	case "img":
		element = htmlBuildImage(node)
	case "use-template":
		element = htmlBuildUseTemplate(node)
	case "create-template":
		element, err = htmlBuildCreateTemplate(node, options)
	default:
		return nil, false, nil
	}
	return element, true, err
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// htmlSetAttrs copies all attributes from an HTML node to a PDFElementNode,
// normalising hyphenated names to camelCase where needed.
func htmlSetAttrs(dst PDFElementNode, attrs []html.Attribute) {
	for _, a := range attrs {
		dst.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
	}
}

func htmlBuildSectionDiv(n *html.Node, options ParseOptions) (*ElemDiv, error) {
	return htmlBuildSectionDivWithInherited(n, nil, options)
}

func htmlBuildSectionDivWithInherited(n *html.Node, inherited *PDFTextStyle, options ParseOptions) (*ElemDiv, error) {
	div := NewElemDiv()
	htmlSetAttrs(div, n.Attr)
	span, err := htmlBuildSpan(n, options)
	if err != nil {
		return nil, wrapHTMLNodeError(n, err)
	}
	baseStyle := mergeDeclaredTextStyles(inherited, span.Style)

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if err := htmlAppendSectionChild(div, child, baseStyle, options); err != nil {
			return nil, err
		}
	}

	return div, nil
}

func htmlAppendSectionChild(div *ElemDiv, child *html.Node, style *PDFTextStyle, options ParseOptions) error {
	if child.Type == html.TextNode {
		if text := tmpl.NormaliseInlineTextNode(child.Data); text != "" {
			return div.Add(&PDFTextNode{Text: text, Style: cloneTextStyle(style)})
		}
		return nil
	}
	if child.Type != html.ElementNode {
		return nil
	}
	if element, supported, err := htmlBuildValueElement(child); supported {
		if err != nil {
			return wrapHTMLNodeError(child, err)
		}
		return div.Add(element)
	}
	switch child.Data {
	case "span":
		span, err := htmlBuildSpanWithInherited(child, style, options)
		if err != nil {
			return wrapHTMLNodeError(child, err)
		}
		return div.Add(span)
	case "img":
		return div.Add(htmlBuildImage(child))
	case "use-template":
		return div.Add(htmlBuildUseTemplate(child))
	case "br":
		return div.Add(NewElemBr())
	default:
		element, supported, err := htmlBuildNestedBlock(child, style, options)
		if err != nil {
			return err
		}
		if supported {
			return div.AddLine(element)
		}
	}
	return nil
}

func htmlBuildNestedBlock(node *html.Node, style *PDFTextStyle, options ParseOptions) (PDFElementNode, bool, error) {
	var element PDFElementNode
	var err error
	switch node.Data {
	case "div":
		element, err = htmlBuildSectionDivWithInherited(node, style, options)
	case "p":
		element, err = htmlBuildParagraph(node, style, options)
	case "h1", "h2", "h3":
		element, err = htmlBuildHeadingWithInherited(node, style, options)
	default:
		return nil, false, nil
	}
	return element, true, err
}

func htmlBuildValueElement(node *html.Node) (PDFElementNode, bool, error) {
	var element PDFElementNode
	var err error
	switch node.Data {
	case "currency-value":
		element, err = htmlBuildCurrencyValue(node)
	case "date-value":
		element, err = htmlBuildDateValue(node)
	case "duration-value":
		element, err = htmlBuildDurationValue(node)
	case "man-days-value":
		element, err = htmlBuildManDaysValue(node)
	default:
		return nil, false, nil
	}
	return element, true, err
}

func htmlBuildImage(n *html.Node) *ElemImg {
	img := NewElemImg()
	htmlSetAttrs(img, n.Attr)
	return img
}

func htmlBuildUseTemplate(n *html.Node) *ElemUseTemplate {
	elem := NewElemUseTemplate()
	htmlSetAttrs(elem, n.Attr)
	return elem
}

// htmlBuildCreateTemplate parses a <create-template> node into an
// ElemCreateTemplate whose children are the renderable content of the template.
func htmlBuildCreateTemplate(n *html.Node, options ParseOptions) (*ElemCreateTemplate, error) {
	elem := NewElemCreateTemplate()
	htmlSetAttrs(elem, n.Attr)
	for _, child := range tmpl.ElemChildren(n) {
		var content PDFElementNode
		var err error
		switch child.Data {
		case "div", "footer":
			content, err = htmlBuildSectionDiv(child, options)
		case "p":
			content, err = htmlBuildParagraph(child, nil, options)
		case "h1", "h2", "h3":
			content, err = htmlBuildHeadingWithInherited(child, nil, options)
		case "table":
			content, err = htmlBuildTable(child, options)
		case "img":
			content = htmlBuildImage(child)
		case "use-template":
			content = htmlBuildUseTemplate(child)
		default:
			continue
		}
		if err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
		if err := elem.Add(content); err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
	}
	return elem, nil
}

func htmlBuildHeadingWithInherited(n *html.Node, inherited *PDFTextStyle, options ParseOptions) (PDFElementNode, error) {
	var heading PDFElementNode
	switch n.Data {
	case "h1":
		heading = NewElemH1()
	case "h2":
		heading = NewElemH2()
	case "h3":
		heading = NewElemH3()
	default:
		return nil, fmt.Errorf("unsupported heading element: <%s>", n.Data)
	}

	htmlSetAttrs(heading, n.Attr)
	span, err := htmlBuildSpan(n, options)
	if err != nil {
		return nil, wrapHTMLNodeError(n, err)
	}
	baseStyle := mergeDeclaredTextStyles(inherited, span.Style)
	hasInline := false

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		added, err := htmlAppendHeadingChild(heading, child, baseStyle, options)
		if err != nil {
			return nil, err
		}
		hasInline = hasInline || added
	}

	if !hasInline {
		baseText := strings.TrimSpace(tmpl.CollectText(n))
		if baseText != "" {
			if err := heading.Add(&PDFTextNode{Text: baseText, Style: cloneTextStyle(baseStyle)}); err != nil {
				return nil, err
			}
		}
	}

	return heading, nil
}

func htmlAppendHeadingChild(heading PDFElementNode, child *html.Node, style *PDFTextStyle, options ParseOptions) (bool, error) {
	if child.Type == html.TextNode {
		text := tmpl.NormaliseInlineTextNode(child.Data)
		if text == "" {
			return false, nil
		}
		return true, heading.Add(&PDFTextNode{Text: text, Style: cloneTextStyle(style)})
	}
	if child.Type != html.ElementNode {
		return false, nil
	}
	if element, supported, err := htmlBuildValueElement(child); supported {
		if err != nil {
			return false, err
		}
		return true, heading.Add(element)
	}
	switch child.Data {
	case "span":
		span, err := htmlBuildSpanWithInherited(child, style, options)
		if err != nil {
			return false, wrapHTMLNodeError(child, err)
		}
		return true, heading.Add(span)
	case "img":
		return true, heading.Add(htmlBuildImage(child))
	case "br":
		return true, heading.Add(NewElemBr())
	}
	return false, nil
}

func htmlBuildParagraph(n *html.Node, inherited *PDFTextStyle, options ParseOptions) (*ElemDiv, error) {
	paragraph := NewElemDiv()
	htmlSetAttrs(paragraph, n.Attr)

	span, err := htmlBuildSpan(n, options)
	if err != nil {
		return nil, wrapHTMLNodeError(n, err)
	}
	baseStyle := mergeDeclaredTextStyles(inherited, span.Style)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if err := htmlAppendParagraphChild(paragraph, child, baseStyle, options); err != nil {
			return nil, err
		}
	}

	if len(paragraph.ElementChildren()) == 0 {
		baseText := strings.TrimSpace(tmpl.CollectText(n))
		if baseText != "" {
			if err := paragraph.Add(&PDFTextNode{Text: baseText, Style: cloneTextStyle(baseStyle)}); err != nil {
				return nil, err
			}
		}
	}

	return paragraph, nil
}

func htmlAppendParagraphChild(paragraph *ElemDiv, child *html.Node, style *PDFTextStyle, options ParseOptions) error {
	if child.Type == html.TextNode {
		if text := tmpl.NormaliseInlineTextNode(child.Data); text != "" {
			return paragraph.Add(&PDFTextNode{Text: text, Style: cloneTextStyle(style)})
		}
		return nil
	}
	if child.Type != html.ElementNode {
		return nil
	}
	if element, supported, err := htmlBuildValueElement(child); supported {
		if err != nil {
			return wrapHTMLNodeError(child, err)
		}
		return paragraph.Add(element)
	}
	switch child.Data {
	case "span":
		span, err := htmlBuildSpanWithInherited(child, style, options)
		if err != nil {
			return wrapHTMLNodeError(child, err)
		}
		return paragraph.Add(span)
	case "img":
		return paragraph.Add(htmlBuildImage(child))
	case "br":
		return paragraph.Add(NewElemBr())
	case "use-template":
		return paragraph.Add(htmlBuildUseTemplate(child))
	}
	return nil
}

func wrapHTMLNodeError(n *html.Node, err error) error {
	return fmt.Errorf("%w in %s", err, htmlNodeSnippet(n))
}

func htmlNodeSnippet(n *html.Node) string {
	if n == nil {
		return "<unknown-node>"
	}
	var buf bytes.Buffer
	if err := html.Render(&buf, n); err != nil {
		return fmt.Sprintf("<%s>", n.Data)
	}
	snippet := strings.TrimSpace(buf.String())
	if len(snippet) > 160 {
		snippet = snippet[:157] + "..."
	}
	return snippet
}

func htmlBuildSpanWithInherited(n *html.Node, inherited *PDFTextStyle, options ParseOptions) (*PDFTextNode, error) {
	span, err := htmlBuildSpan(n, options)
	if err != nil {
		return nil, err
	}
	span.Style = mergeDeclaredTextStyles(inherited, span.Style)
	return span, nil
}

func mergeDeclaredTextStyles(base, override *PDFTextStyle) *PDFTextStyle {
	if base == nil && override == nil {
		return nil
	}

	merged := PDFTextStyle{}
	if base != nil {
		applyBaseTextStyle(&merged, base)
	}
	if override != nil {
		applyOverrideTextStyle(&merged, override)
	}

	if emptyTextStyle(&merged) {
		return nil
	}

	return &merged
}

func applyBaseTextStyle(target, source *PDFTextStyle) {
	target.FontFace, target.FontStyle, target.FontStyleSet = source.FontFace, source.FontStyle, source.FontStyleSet
	target.FontSize, target.FontColor, target.Align, target.LineHeight = source.FontSize, source.FontColor, source.Align, source.LineHeight
}

func applyOverrideTextStyle(target, source *PDFTextStyle) {
	if source.FontFace != "" {
		target.FontFace = source.FontFace
	}
	if source.FontStyleSet {
		target.FontStyle, target.FontStyleSet = source.FontStyle, true
	}
	if source.FontSize > 0 {
		target.FontSize = source.FontSize
	}
	if source.FontColor != "" {
		target.FontColor = source.FontColor
	}
	if source.Align != "" {
		target.Align = source.Align
	}
	if source.LineHeight > 0 {
		target.LineHeight = source.LineHeight
	}
	if source.BackgroundColor != "" {
		target.BackgroundColor = source.BackgroundColor
	}
	if source.BorderColor != "" {
		target.BorderColor = source.BorderColor
	}
	if source.BorderStyle != "" {
		target.BorderStyle = source.BorderStyle
	}
	if source.BorderWidth > 0 {
		target.BorderWidth = source.BorderWidth
	}
}

func cloneTextStyle(style *PDFTextStyle) *PDFTextStyle {
	if style == nil {
		return nil
	}
	cloned := *style
	return &cloned
}

func htmlNormaliseFontStyle(value string) string {
	parts := strings.Fields(strings.ToLower(value))
	if len(parts) == 0 {
		return ""
	}

	var out strings.Builder
	for _, part := range parts {
		switch part {
		case "normal":
			continue
		case "bold", "b":
			out.WriteString("B")
		case "italic", "i":
			out.WriteString("I")
		case "underline", "u":
			out.WriteString("U")
		default:
			out.WriteString(strings.ToUpper(part))
		}
	}
	return out.String()
}

// ─── value elements ──────────────────────────────────────────────────────────

func htmlBuildCurrencyValue(n *html.Node) (*ElemCurrencyValue, error) {
	raw := htmlAttrValWithFallback(n, "v", "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<currency-value> invalid v=%q: %w", raw, err)
	}
	elem := NewElemCurrencyValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" && a.Key != "v" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildDateValue(n *html.Node) (*ElemDateValue, error) {
	elem := NewElemDateValue(tmpl.AttrVal(n, "value"))
	for _, a := range n.Attr {
		if a.Key != "value" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildDurationValue(n *html.Node) (*ElemDurationValue, error) {
	raw := htmlAttrValWithFallback(n, "v", "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<duration-value> invalid v=%q: %w", raw, err)
	}
	elem := NewElemDurationValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" && a.Key != "v" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildManDaysValue(n *html.Node) (*ElemManDaysValue, error) {
	raw := htmlAttrValWithFallback(n, "v", "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<man-days-value> invalid v=%q: %w", raw, err)
	}
	elem := NewElemManDaysValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" && a.Key != "v" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlAttrValWithFallback(n *html.Node, primary, fallback string) string {
	if v := tmpl.AttrVal(n, primary); v != "" {
		return v
	}
	return tmpl.AttrVal(n, fallback)
}
