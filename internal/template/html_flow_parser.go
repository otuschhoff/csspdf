package template

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/otuschhoff/invoice-gen/internal/pdfcore"
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
//	<currency-value>      → ElemCurrencyValue  ("value" attr → Value float64)
//	<date-value>          → ElemDateValue      ("value" attr → Value string;
//	                                             "long" attr forwarded)
//	<duration-value>      → ElemDurationValue  ("value" attr → Value float64)
//	<man-days-value>      → ElemManDaysValue   ("value" attr → Value float64;
//	                                             "unit" attr forwarded)
//
// Attribute name normalisation (HTML kebab-case → Elem* camelCase):
//
//	row-height-min → rowHeightMin
//
// Inside <td>, the first non-empty child is added inline (Add); each
// subsequent child is added with a preceding line break (AddLine), matching
// the main-text / sub-text rendering idiom.
func ParseHTMLTableElem(htmlStr, cssStyle string) (*pdfcore.ElemTable, error) {
	// Wrap in a minimal document to give the parser a well-defined context.
	doc, err := ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}

	tableNode := FindFirst(doc, "table")
	if tableNode == nil {
		return nil, fmt.Errorf("no <table> element found in HTML fragment")
	}

	return htmlBuildTable(tableNode)
}

// ParseHTMLIntroElem parses an HTML fragment containing a
// <div id="intro"> element with <span>, <div> and <br> children and maps it
// to an ElemDiv. cssStyle is applied before parsing (may be empty).
func ParseHTMLIntroElem(htmlStr, cssStyle string) (*pdfcore.ElemDiv, error) {
	doc, err := ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}

	introNode := FindFirstByID(doc, "div", "intro")
	if introNode == nil {
		return nil, fmt.Errorf("no <div id=\"intro\"> element found in HTML fragment")
	}

	return htmlBuildIntroDiv(introNode), nil
}

// ParseHTMLDocFlow parses an HTML fragment into top-level renderable elements
// in source order. cssStyle is applied before parsing (may be empty).
// Supported root-level tags are <div>, <table>, <img>, and headings <h1>..<h3>.
func ParseHTMLDocFlow(htmlStr, cssStyle string) ([]pdfcore.PDFElementNode, error) {
	doc, err := ParseStyledFragment(htmlStr, cssStyle)
	if err != nil {
		return nil, err
	}

	body := FindFirst(doc, "body")
	if body == nil {
		return nil, fmt.Errorf("no <body> element found in HTML fragment")
	}

	out := make([]pdfcore.PDFElementNode, 0)
	for _, child := range ElemChildren(body) {
		switch child.Data {
		case "div":
			out = append(out, htmlBuildIntroDiv(child))
		case "h1", "h2", "h3":
			heading, err := htmlBuildHeading(child)
			if err != nil {
				return nil, err
			}
			out = append(out, heading)
		case "table":
			table, err := htmlBuildTable(child)
			if err != nil {
				return nil, err
			}
			out = append(out, table)
		case "img":
			out = append(out, htmlBuildImage(child))
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no supported top-level elements found in HTML fragment")
	}

	return out, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// htmlSetAttrs copies all attributes from an HTML node to a PDFElementNode,
// normalising hyphenated names to camelCase where needed.
func htmlSetAttrs(dst pdfcore.PDFElementNode, attrs []html.Attribute) {
	for _, a := range attrs {
		dst.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
	}
}

// htmlNormaliseAttrKey converts HTML hyphenated attribute names to the
// camelCase keys expected by the PDF element/rendering infrastructure.
func htmlNormaliseAttrKey(key string) string {
	switch key {
	case "row-height-min":
		return "rowHeightMin"
	case "padding-top":
		return "paddingTop"
	case "padding-right":
		return "paddingRight"
	case "padding-bottom":
		return "paddingBottom"
	case "padding-left":
		return "paddingLeft"
	case "background-color":
		return "backgroundColor"
	case "border-width":
		return "borderWidth"
	case "border-style":
		return "borderStyle"
	case "border-color":
		return "borderColor"
	case "margin-top":
		return "marginTop"
	case "margin-bottom":
		return "marginBottom"
	case "break-before":
		return "breakBefore"
	case "break-after":
		return "breakAfter"
	}
	return key
}

func htmlBuildIntroDiv(n *html.Node) *pdfcore.ElemDiv {
	div := pdfcore.NewElemDiv()
	htmlSetAttrs(div, n.Attr)
	baseSpan := htmlBuildSpan(n)
	baseStyle := baseSpan.Style

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			text := NormaliseInlineTextNode(child.Data)
			if text != "" {
				div.Add(&pdfcore.PDFTextNode{Text: text, Style: baseStyle})
			}
			continue
		}
		if child.Type != html.ElementNode {
			continue
		}

		switch child.Data {
		case "span":
			div.Add(htmlBuildSpan(child))
		case "img":
			div.Add(htmlBuildImage(child))
		case "div":
			lineDiv := pdfcore.NewElemDiv()
			htmlSetAttrs(lineDiv, child.Attr)
			lineBase := htmlBuildSpan(child)
			lineStyle := lineBase.Style
			hasInline := false
			for c := child.FirstChild; c != nil; c = c.NextSibling {
				switch c.Type {
				case html.TextNode:
					text := NormaliseInlineTextNode(c.Data)
					if text == "" {
						continue
					}
					lineDiv.Add(&pdfcore.PDFTextNode{Text: text, Style: lineStyle})
					hasInline = true
				case html.ElementNode:
					switch c.Data {
					case "span":
						lineDiv.Add(htmlBuildSpan(c))
						hasInline = true
					case "img":
						lineDiv.Add(htmlBuildImage(c))
						hasInline = true
					case "br":
						lineDiv.Add(pdfcore.NewElemBr())
					}
				}
			}
			if !hasInline {
				lineDiv.Add(lineBase)
			}
			div.AddLine(lineDiv)
		case "br":
			div.Add(pdfcore.NewElemBr())
		}
	}

	return div
}

func htmlBuildImage(n *html.Node) *pdfcore.ElemImg {
	img := pdfcore.NewElemImg()
	htmlSetAttrs(img, n.Attr)
	return img
}

func htmlBuildHeading(n *html.Node) (pdfcore.PDFElementNode, error) {
	var heading pdfcore.PDFElementNode
	switch n.Data {
	case "h1":
		heading = pdfcore.NewElemH1()
	case "h2":
		heading = pdfcore.NewElemH2()
	case "h3":
		heading = pdfcore.NewElemH3()
	default:
		return nil, fmt.Errorf("unsupported heading element: <%s>", n.Data)
	}

	htmlSetAttrs(heading, n.Attr)
	baseSpan := htmlBuildSpan(n)
	baseStyle := baseSpan.Style
	hasInline := false

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			text := NormaliseInlineTextNode(child.Data)
			if text == "" {
				continue
			}
			heading.Add(&pdfcore.PDFTextNode{Text: text, Style: baseStyle})
			hasInline = true
		case html.ElementNode:
			switch child.Data {
			case "span":
				heading.Add(htmlBuildSpan(child))
				hasInline = true
			case "img":
				heading.Add(htmlBuildImage(child))
				hasInline = true
			case "br":
				heading.Add(pdfcore.NewElemBr())
			}
		}
	}

	if !hasInline && strings.TrimSpace(baseSpan.Text) != "" {
		heading.Add(baseSpan)
	}

	return heading, nil
}

// ─── table ───────────────────────────────────────────────────────────────────

func htmlBuildTable(n *html.Node) (*pdfcore.ElemTable, error) {
	table := pdfcore.NewElemTable()
	htmlSetAttrs(table, n.Attr)

	for _, child := range ElemChildren(n) {
		switch child.Data {
		case "colgroup":
			cg, err := htmlBuildColgroup(child)
			if err != nil {
				return nil, err
			}
			table.Add(cg)
		case "thead", "tbody":
			section, err := htmlBuildSection(child)
			if err != nil {
				return nil, err
			}
			table.Add(section)
		}
	}
	return table, nil
}

// ─── colgroup / col ──────────────────────────────────────────────────────────

func htmlBuildColgroup(n *html.Node) (*pdfcore.ElemColgroup, error) {
	cg := pdfcore.NewElemColgroup()
	for _, child := range ElemChildren(n) {
		if child.Data != "col" {
			continue
		}
		col := pdfcore.NewElemCol()
		htmlSetAttrs(col, child.Attr)
		cg.Add(col)
	}
	return cg, nil
}

// ─── thead / tbody ───────────────────────────────────────────────────────────

func htmlBuildSection(n *html.Node) (pdfcore.PDFElementNode, error) {
	var section pdfcore.PDFElementNode
	switch n.Data {
	case "thead":
		section = pdfcore.NewElemThead()
	case "tbody":
		section = pdfcore.NewElemTbody()
	default:
		return nil, fmt.Errorf("unsupported table section: <%s>", n.Data)
	}

	for _, child := range ElemChildren(n) {
		if child.Data != "tr" {
			continue
		}
		tr, err := htmlBuildTr(child)
		if err != nil {
			return nil, err
		}
		section.Add(tr)
	}
	return section, nil
}

// ─── tr ──────────────────────────────────────────────────────────────────────

func htmlBuildTr(n *html.Node) (*pdfcore.ElemTr, error) {
	tr := pdfcore.NewElemTr()
	htmlSetAttrs(tr, n.Attr)

	for _, child := range ElemChildren(n) {
		if child.Data != "td" && child.Data != "th" {
			continue
		}
		td, err := htmlBuildTableCell(child)
		if err != nil {
			return nil, err
		}
		tr.Add(td)
	}
	return tr, nil
}

// ─── td / th ─────────────────────────────────────────────────────────────────

// htmlBuildTableCell converts a <td> or <th> HTML element into an ElemTd/ElemTh.
// Children are processed in source order: the first non-empty child uses Add
// (inline), every subsequent child uses AddLine (line break before it).
func htmlBuildTableCell(n *html.Node) (pdfcore.PDFElementNode, error) {
	var cell pdfcore.PDFElementNode
	switch n.Data {
	case "td":
		cell = pdfcore.NewElemTd()
	case "th":
		cell = pdfcore.NewElemTh()
	default:
		return nil, fmt.Errorf("unsupported table cell element: <%s>", n.Data)
	}
	htmlSetAttrs(cell, n.Attr)

	first := true
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		node, err := htmlBuildTdChild(c)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		if first {
			cell.Add(node)
			first = false
		} else {
			cell.AddLine(node)
		}
	}
	return cell, nil
}

// htmlBuildTdChild maps a single child node of a <td> to a PDFNode.
// Whitespace-only text nodes and unrecognised elements return nil.
func htmlBuildTdChild(c *html.Node) (pdfcore.PDFNode, error) {
	switch c.Type {
	case html.TextNode:
		text := strings.TrimSpace(c.Data)
		if text == "" {
			return nil, nil
		}
		return &pdfcore.PDFTextNode{Text: text}, nil

	case html.ElementNode:
		switch c.Data {
		case "br":
			return pdfcore.NewElemBr(), nil
		case "span":
			return htmlBuildSpan(c), nil
		case "ul":
			return htmlBuildUnorderedList(c), nil
		case "currency-value":
			return htmlBuildCurrencyValue(c)
		case "date-value":
			return htmlBuildDateValue(c)
		case "duration-value":
			return htmlBuildDurationValue(c)
		case "man-days-value":
			return htmlBuildManDaysValue(c)
		}
	}
	return nil, nil
}

func htmlBuildUnorderedList(n *html.Node) *pdfcore.PDFTextNode {
	items := make([]string, 0)
	var itemStyle *pdfcore.PDFTextStyle

	for _, child := range ElemChildren(n) {
		if child.Data != "li" {
			continue
		}

		itemText := ""
		for c := child.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.Data == "span" {
				span := htmlBuildSpan(c)
				itemText = strings.TrimSpace(span.Text)
				if itemStyle == nil && span.Style != nil {
					itemStyle = span.Style
				}
				break
			}
		}

		if itemText == "" {
			itemText = strings.TrimSpace(CollectText(child))
		}
		itemText = strings.TrimPrefix(itemText, "- ")
		itemText = strings.TrimSpace(itemText)
		if itemText == "" {
			continue
		}

		items = append(items, "• "+itemText)
	}

	if len(items) == 0 {
		return nil
	}

	return &pdfcore.PDFTextNode{Text: strings.Join(items, "\n"), Style: itemStyle}
}

// ─── <span> → PDFTextNode ────────────────────────────────────────────────────

// htmlBuildSpan converts a <span> to a PDFTextNode, reading font-style,
// font-face, font-size, font-color, and align attributes into PDFTextStyle.
// If none of the style attributes are present the style is left nil.
func htmlBuildSpan(n *html.Node) *pdfcore.PDFTextNode {
	style := &pdfcore.PDFTextStyle{}
	for _, a := range n.Attr {
		switch a.Key {
		case "font-style":
			style.FontStyle = htmlNormaliseFontStyle(a.Val)
			style.FontStyleSet = true
		case "font-face":
			style.FontFace = a.Val
		case "font-size":
			if f, err := strconv.ParseFloat(a.Val, 64); err == nil {
				style.FontSize = f
			}
		case "font-color":
			style.FontColor = a.Val
		case "align":
			style.Align = htmlNormaliseTextAlign(a.Val)
		case "background-color", "backgroundColor":
			style.BackgroundColor = strings.TrimSpace(a.Val)
		case "border-color", "borderColor":
			style.BorderColor = strings.TrimSpace(a.Val)
		case "border-style", "borderStyle":
			style.BorderStyle = strings.ToLower(strings.TrimSpace(a.Val))
		case "border-width", "borderWidth":
			if f, ok := ParseLengthValue(a.Val); ok {
				style.BorderWidth = f
			}
		case "border":
			bw, bs, bc := ParseBorderShorthand(a.Val)
			if bw > 0 {
				style.BorderWidth = bw
			}
			if bs != "" {
				style.BorderStyle = bs
			}
			if bc != "" {
				style.BorderColor = bc
			}
		}
	}
	if style.FontFace == "" && style.FontStyle == "" && style.FontSize == 0 &&
		style.FontColor == "" && style.Align == "" && style.BackgroundColor == "" &&
		style.BorderColor == "" && style.BorderStyle == "" && style.BorderWidth == 0 && !style.FontStyleSet {
		style = nil
	}
	return &pdfcore.PDFTextNode{Text: CollectText(n), Style: style}
}

func htmlNormaliseTextAlign(value string) pdfcore.TextAlign {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "left", "l":
		return pdfcore.TextAlignLeft
	case "center", "c":
		return pdfcore.TextAlignCenter
	case "right", "r":
		return pdfcore.TextAlignRight
	default:
		return pdfcore.TextAlign(value)
	}
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

func htmlBuildCurrencyValue(n *html.Node) (*pdfcore.ElemCurrencyValue, error) {
	raw := AttrVal(n, "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<currency-value> invalid value=%q: %w", raw, err)
	}
	elem := pdfcore.NewElemCurrencyValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildDateValue(n *html.Node) (*pdfcore.ElemDateValue, error) {
	elem := pdfcore.NewElemDateValue(AttrVal(n, "value"))
	for _, a := range n.Attr {
		if a.Key != "value" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildDurationValue(n *html.Node) (*pdfcore.ElemDurationValue, error) {
	raw := AttrVal(n, "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<duration-value> invalid value=%q: %w", raw, err)
	}
	elem := pdfcore.NewElemDurationValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}

func htmlBuildManDaysValue(n *html.Node) (*pdfcore.ElemManDaysValue, error) {
	raw := AttrVal(n, "value")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<man-days-value> invalid value=%q: %w", raw, err)
	}
	elem := pdfcore.NewElemManDaysValue(f)
	for _, a := range n.Attr {
		if a.Key != "value" {
			elem.SetAttribute(htmlNormaliseAttrKey(a.Key), a.Val)
		}
	}
	return elem, nil
}
