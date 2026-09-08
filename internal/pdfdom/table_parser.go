package pdfdom

import (
	"fmt"
	"strings"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
	"golang.org/x/net/html"
)

func htmlBuildTable(node *html.Node, options ParseOptions) (*ElemTable, error) {
	table := NewElemTable()
	htmlSetAttrs(table, node.Attr)
	for _, child := range tmpl.ElemChildren(node) {
		var element PDFElementNode
		var err error
		switch child.Data {
		case "colgroup":
			element, err = htmlBuildColgroup(child)
		case "thead", "tbody":
			element, err = htmlBuildTableSection(child, options)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := table.Add(element); err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
	}
	return table, nil
}

func htmlBuildColgroup(node *html.Node) (*ElemColgroup, error) {
	group := NewElemColgroup()
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data == "col" {
			column := NewElemCol()
			htmlSetAttrs(column, child.Attr)
			if err := group.Add(column); err != nil {
				return nil, wrapHTMLNodeError(child, err)
			}
		}
	}
	return group, nil
}

func htmlBuildTableSection(node *html.Node, options ParseOptions) (PDFElementNode, error) {
	var section PDFElementNode
	switch node.Data {
	case "thead":
		section = NewElemThead()
	case "tbody":
		section = NewElemTbody()
	default:
		return nil, fmt.Errorf("unsupported table section: <%s>", node.Data)
	}
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data != "tr" {
			continue
		}
		row, err := htmlBuildTr(child, options)
		if err != nil {
			return nil, err
		}
		if err := section.Add(row); err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
	}
	return section, nil
}

func htmlBuildTr(node *html.Node, options ParseOptions) (*ElemTr, error) {
	row := NewElemTr()
	htmlSetAttrs(row, node.Attr)
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data != "td" && child.Data != "th" {
			continue
		}
		cell, err := htmlBuildTableCell(child, options)
		if err != nil {
			return nil, err
		}
		if err := row.Add(cell); err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
	}
	return row, nil
}

func htmlBuildTableCell(node *html.Node, options ParseOptions) (PDFElementNode, error) {
	var cell PDFElementNode
	switch node.Data {
	case "td":
		cell = NewElemTd()
	case "th":
		cell = NewElemTh()
	default:
		return nil, fmt.Errorf("unsupported table cell element: <%s>", node.Data)
	}
	htmlSetAttrs(cell, node.Attr)
	first := true
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		content, err := htmlBuildTableCellChild(child, options)
		if err != nil {
			return nil, err
		}
		if content == nil {
			continue
		}
		if first {
			if err := cell.Add(content); err != nil {
				return nil, wrapHTMLNodeError(child, err)
			}
			first = false
		} else {
			if err := cell.AddLine(content); err != nil {
				return nil, wrapHTMLNodeError(child, err)
			}
		}
	}
	return cell, nil
}

func htmlBuildTableCellChild(node *html.Node, options ParseOptions) (PDFNode, error) {
	if node.Type == html.TextNode {
		text := strings.TrimSpace(node.Data)
		if text == "" {
			return nil, nil
		}
		return &PDFTextNode{Text: text}, nil
	}
	if node.Type != html.ElementNode {
		return nil, nil
	}
	switch node.Data {
	case "br":
		return NewElemBr(), nil
	case "span":
		return htmlBuildSpan(node, options)
	case "ul":
		return htmlBuildUnorderedList(node, options)
	case "currency-value":
		return htmlBuildCurrencyValue(node)
	case "date-value":
		return htmlBuildDateValue(node)
	case "duration-value":
		return htmlBuildDurationValue(node)
	case "man-days-value":
		return htmlBuildManDaysValue(node)
	default:
		return nil, nil
	}
}

func htmlBuildUnorderedList(node *html.Node, options ParseOptions) (*PDFTextNode, error) {
	items := make([]string, 0)
	var itemStyle *PDFTextStyle
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data != "li" {
			continue
		}
		text, style, err := htmlListItem(child, options)
		if err != nil {
			return nil, wrapHTMLNodeError(child, err)
		}
		if text == "" {
			continue
		}
		if itemStyle == nil {
			itemStyle = style
		}
		items = append(items, "• "+text)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &PDFTextNode{Text: strings.Join(items, "\n"), Style: itemStyle}, nil
}

func htmlListItem(node *html.Node, options ParseOptions) (string, *PDFTextStyle, error) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "span" {
			span, err := htmlBuildSpan(child, options)
			if err != nil {
				return "", nil, err
			}
			return strings.TrimSpace(strings.TrimPrefix(span.Text, "- ")), span.Style, nil
		}
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tmpl.CollectText(node)), "- ")), nil, nil
}
