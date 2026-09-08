package pdfdom

import (
	"fmt"
	"strings"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
	"golang.org/x/net/html"
)

func htmlBuildTable(node *html.Node) (*ElemTable, error) {
	table := NewElemTable()
	htmlSetAttrs(table, node.Attr)
	for _, child := range tmpl.ElemChildren(node) {
		var element PDFElementNode
		var err error
		switch child.Data {
		case "colgroup":
			element, err = htmlBuildColgroup(child)
		case "thead", "tbody":
			element, err = htmlBuildTableSection(child)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		table.Add(element)
	}
	return table, nil
}

func htmlBuildColgroup(node *html.Node) (*ElemColgroup, error) {
	group := NewElemColgroup()
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data == "col" {
			column := NewElemCol()
			htmlSetAttrs(column, child.Attr)
			group.Add(column)
		}
	}
	return group, nil
}

func htmlBuildTableSection(node *html.Node) (PDFElementNode, error) {
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
		row, err := htmlBuildTr(child)
		if err != nil {
			return nil, err
		}
		section.Add(row)
	}
	return section, nil
}

func htmlBuildTr(node *html.Node) (*ElemTr, error) {
	row := NewElemTr()
	htmlSetAttrs(row, node.Attr)
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data != "td" && child.Data != "th" {
			continue
		}
		cell, err := htmlBuildTableCell(child)
		if err != nil {
			return nil, err
		}
		row.Add(cell)
	}
	return row, nil
}

func htmlBuildTableCell(node *html.Node) (PDFElementNode, error) {
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
		content, err := htmlBuildTableCellChild(child)
		if err != nil {
			return nil, err
		}
		if content == nil {
			continue
		}
		if first {
			cell.Add(content)
			first = false
		} else {
			cell.AddLine(content)
		}
	}
	return cell, nil
}

func htmlBuildTableCellChild(node *html.Node) (PDFNode, error) {
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
		return htmlBuildSpan(node), nil
	case "ul":
		return htmlBuildUnorderedList(node), nil
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

func htmlBuildUnorderedList(node *html.Node) *PDFTextNode {
	items := make([]string, 0)
	var itemStyle *PDFTextStyle
	for _, child := range tmpl.ElemChildren(node) {
		if child.Data != "li" {
			continue
		}
		text, style := htmlListItem(child)
		if text == "" {
			continue
		}
		if itemStyle == nil {
			itemStyle = style
		}
		items = append(items, "• "+text)
	}
	if len(items) == 0 {
		return nil
	}
	return &PDFTextNode{Text: strings.Join(items, "\n"), Style: itemStyle}
}

func htmlListItem(node *html.Node) (string, *PDFTextStyle) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "span" {
			span := htmlBuildSpan(child)
			return strings.TrimSpace(strings.TrimPrefix(span.Text, "- ")), span.Style
		}
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tmpl.CollectText(node)), "- ")), nil
}
