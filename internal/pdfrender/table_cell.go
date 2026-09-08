package pdfrender

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func (l *LayoutPDF) TableDefFromElement(table *pdfdom.ElemTable, availableWidth float64) (*TableDef, error) {
	width, err := parseTableWidthAttr(table, availableWidth)
	if err != nil {
		return nil, err
	}
	padding, err := parseLayoutFloatAttr(table, "padding")
	if err != nil {
		return nil, err
	}
	rowHeight, err := parseLayoutFloatAttr(table, "rowHeightMin")
	if err != nil {
		return nil, err
	}
	tableDef := &TableDef{Width: width, Padding: padding, RowHeightMin: rowHeight}
	applyTableStyle(tableDef, table)
	columnsDefined, err := parseTableColumns(tableDef, table)
	if err != nil {
		return nil, err
	}
	if err := l.appendTableChildren(tableDef, table, &columnsDefined); err != nil {
		return nil, err
	}
	return tableDef, nil
}

func applyTableStyle(tableDef *TableDef, table *pdfdom.ElemTable) {
	tableDef.TableLayout = strings.ToLower(firstTrimmedAttribute(table, "tableLayout", "table-layout"))
	applyTableMargins(tableDef, table)
	tableDef.Background = firstTrimmedAttribute(table, "backgroundColor", "background-color")
	applyTableBorder(tableDef, table)
}

func applyTableMargins(tableDef *TableDef, table *pdfdom.ElemTable) {
	if _, ok := table.Attribute("marginTop"); ok {
		tableDef.MarginTop, tableDef.MarginTopSet = htmlLengthToFloat(table, "marginTop", "margin-top"), true
	} else if _, ok := table.Attribute("margin-top"); ok {
		tableDef.MarginTop, tableDef.MarginTopSet = htmlLengthToFloat(table, "marginTop", "margin-top"), true
	}
	if _, ok := table.Attribute("marginBottom"); ok {
		tableDef.MarginBottom, tableDef.MarginBottomSet = htmlLengthToFloat(table, "marginBottom", "margin-bottom"), true
	} else if _, ok := table.Attribute("margin-bottom"); ok {
		tableDef.MarginBottom, tableDef.MarginBottomSet = htmlLengthToFloat(table, "marginBottom", "margin-bottom"), true
	}
}

func applyTableBorder(tableDef *TableDef, table *pdfdom.ElemTable) {
	tableDef.BorderColor = firstTrimmedAttribute(table, "borderColor", "border-color")
	tableDef.BorderStyle = strings.ToLower(firstTrimmedAttribute(table, "borderStyle", "border-style"))
	tableDef.BorderWidth = htmlLengthToFloat(table, "borderWidth", "border-width")
	border, ok := table.Attribute("border")
	if !ok {
		return
	}
	width, style, color := templateload.ParseBorderShorthand(border)
	if tableDef.BorderWidth <= 0 {
		tableDef.BorderWidth = width
	}
	if tableDef.BorderStyle == "" {
		tableDef.BorderStyle = style
	}
	if tableDef.BorderColor == "" {
		tableDef.BorderColor = color
	}
}

func parseTableColumns(tableDef *TableDef, table *pdfdom.ElemTable) (bool, error) {
	defined, groupCount := false, 0
	for _, child := range table.ElementChildren() {
		group, ok := child.(*pdfdom.ElemColgroup)
		if !ok {
			continue
		}
		groupCount++
		if groupCount > 1 {
			return false, fmt.Errorf("table may contain at most one colgroup")
		}
		tableDef.Columns = make([]ColumnDef, 0, len(group.ElementChildren()))
		for _, node := range group.ElementChildren() {
			column, ok := node.(*pdfdom.ElemCol)
			if !ok {
				return false, fmt.Errorf("colgroup contains non-col child")
			}
			width, ok := column.Attribute("width")
			if !ok || strings.TrimSpace(width) == "" {
				tableDef.Columns = append(tableDef.Columns, ColumnDef{})
				continue
			}
			var parsed float64
			if _, err := fmt.Sscanf(strings.TrimSpace(width), "%f", &parsed); err != nil {
				return false, newInvalidColWidthError(column, len(tableDef.Columns)+1, width)
			}
			tableDef.Columns = append(tableDef.Columns, ColumnDef{Width: parsed})
		}
		defined = true
	}
	return defined, nil
}

func (l *LayoutPDF) appendTableChildren(tableDef *TableDef, table *pdfdom.ElemTable, columnsDefined *bool) error {
	for _, child := range table.ElementChildren() {
		switch element := child.(type) {
		case *pdfdom.ElemColgroup:
			continue
		case *pdfdom.ElemThead:
			if err := l.appendTableSectionRows(tableDef, element.ElementChildren(), true, columnsDefined, "thead"); err != nil {
				return err
			}
		case *pdfdom.ElemTbody:
			if err := l.appendTableSectionRows(tableDef, element.ElementChildren(), false, columnsDefined, "tbody"); err != nil {
				return err
			}
		case *pdfdom.ElemTr:
			if err := l.appendTableRow(tableDef, element, false, columnsDefined); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported table child element %T", child)
		}
	}
	return nil
}

func (l *LayoutPDF) appendTableSectionRows(table *TableDef, nodes []pdfdom.PDFNode, header bool, columnsDefined *bool, section string) error {
	for _, node := range nodes {
		row, ok := node.(*pdfdom.ElemTr)
		if !ok {
			return fmt.Errorf("%s contains non-tr child", section)
		}
		if err := l.appendTableRow(table, row, header, columnsDefined); err != nil {
			return err
		}
	}
	return nil
}

func (l *LayoutPDF) appendTableRow(tableDef *TableDef, row *pdfdom.ElemTr, isHeader bool, columnsDefined *bool) error {
	cells := row.ElementChildren()
	if !*columnsDefined {
		columns, err := columnsFromCells(cells)
		if err != nil {
			return err
		}
		tableDef.Columns, *columnsDefined = columns, true
	}
	rowDef := RowDef{Header: isHeader}
	if fill, ok := row.Attribute("fill"); ok {
		rowDef.Fill = fill
	}
	if stroke, ok := row.Attribute("stroke"); ok {
		value := stroke == "true"
		rowDef.Stroke = &value
	}
	if height, ok := row.Attribute("height"); ok {
		scanFloatAttribute(height, &rowDef.Height)
	}
	for _, child := range cells {
		cell, err := tableCellFromNode(child)
		if err != nil {
			return err
		}
		rowDef.Cells = append(rowDef.Cells, l.CellDefFromTableCell(cell, isHeader))
	}
	tableDef.Rows = append(tableDef.Rows, rowDef)
	return nil
}

func columnsFromCells(cells []pdfdom.PDFNode) ([]ColumnDef, error) {
	columns := make([]ColumnDef, 0, len(cells))
	for _, child := range cells {
		cell, err := tableCellFromNode(child)
		if err != nil {
			return nil, err
		}
		column := columnDefFromCell(cell)
		span := 1
		if raw, ok := cell.Attribute("colspan"); ok {
			scanIntAttribute(raw, &span)
		}
		span = normalizedColspan(span)
		if span > 1 {
			column.Width /= float64(span)
			column.MinWidth /= float64(span)
			column.MaxWidth /= float64(span)
		}
		for range span {
			columns = append(columns, column)
		}
	}
	return columns, nil
}

// CellDefFromTableCell builds a CellDef from an HTML table cell element.
func (l *LayoutPDF) CellDefFromTableCell(cellElem pdfdom.PDFElementNode, isHeader bool) CellDef {
	cell := CellDef{}
	applyCellFontAndColors(&cell, cellElem)
	applyCellDimensions(&cell, cellElem)
	explicitAlign := applyCellAlignment(&cell, cellElem, isHeader)
	if colspan, ok := cellElem.Attribute("colspan"); ok {
		scanIntAttribute(colspan, &cell.Colspan)
	}
	if _, ok := cellElem.(*pdfdom.ElemTh); ok {
		cell.Bold = true
	}
	if l.applyCellValue(&cell, cellElem, isHeader) {
		return cell
	}
	l.applyCellText(&cell, cellElem, explicitAlign)
	return cell
}

func applyCellFontAndColors(cell *CellDef, elem pdfdom.PDFElementNode) {
	cell.FontFace = firstTrimmedAttribute(elem, "fontFace", "font-face")
	cell.FontStyle = firstTrimmedAttribute(elem, "fontStyle", "font-style")
	if weight := firstTrimmedAttribute(elem, "font-weight"); weight != "" && !strings.EqualFold(weight, "normal") {
		cell.FontStyle = strings.TrimSpace(cell.FontStyle + " " + weight)
	}
	cell.FontColor = firstTrimmedAttribute(elem, "fontColor", "font-color")
	cell.Background = firstTrimmedAttribute(elem, "fill", "backgroundColor", "background-color")
}

func applyCellDimensions(cell *CellDef, elem pdfdom.PDFElementNode) {
	cell.FontSize = parseTableFloat(firstTrimmedAttribute(elem, "fontSize", "font-size"))
	cell.LineHeight = parseTableFloat(firstTrimmedAttribute(elem, "lineHeight", "line-height"))
	if padding := htmlLengthToFloat(elem, "padding"); padding > 0 {
		cell.PaddingTop, cell.PaddingRight, cell.PaddingBottom, cell.PaddingLeft = padding, padding, padding, padding
	}
	if padding := htmlLengthToFloat(elem, "paddingTop", "padding-top"); padding > 0 {
		cell.PaddingTop = padding
	}
	if padding := htmlLengthToFloat(elem, "paddingRight", "padding-right"); padding > 0 {
		cell.PaddingRight = padding
	}
	if padding := htmlLengthToFloat(elem, "paddingBottom", "padding-bottom"); padding > 0 {
		cell.PaddingBottom = padding
	}
	if padding := htmlLengthToFloat(elem, "paddingLeft", "padding-left"); padding > 0 {
		cell.PaddingLeft = padding
	}
}

func applyCellAlignment(cell *CellDef, elem pdfdom.PDFElementNode, isHeader bool) bool {
	if align, ok := elem.Attribute("align"); ok {
		cell.Align = tableAlignFromAttr(align)
		return true
	}
	if isHeader {
		cell.Align = "C"
	}
	whiteSpace := firstTrimmedAttribute(elem, "whiteSpace", "white-space")
	cell.NoWrap = strings.EqualFold(whiteSpace, "nowrap")
	return false
}

func (l *LayoutPDF) applyCellValue(cell *CellDef, elem pdfdom.PDFElementNode, isHeader bool) bool {
	for _, child := range elem.ElementChildren() {
		switch value := child.(type) {
		case *pdfdom.ElemCurrencyValue:
			cell.Text = value.Format(l.Formatter)
		case *pdfdom.ElemDateValue:
			cell.Text = value.Format(l.Formatter)
		case *pdfdom.ElemDurationValue:
			cell.Text = value.Format(l.Formatter)
		case *pdfdom.ElemManDaysValue:
			cell.Text = value.Format(l.Formatter)
		default:
			continue
		}
		cell.Bold = cell.Bold || isHeader
		return true
	}
	return false
}

func (l *LayoutPDF) applyCellText(cell *CellDef, elem pdfdom.PDFElementNode, explicitAlign bool) {
	var textChildren []*pdfdom.PDFTextNode
	for _, child := range elem.ElementChildren() {
		if textNode, ok := child.(*pdfdom.PDFTextNode); ok {
			textChildren = append(textChildren, textNode)
		}
	}
	if len(textChildren) == 0 {
		return
	}
	cell.Text = l.ResolvePDFTextNode(textChildren[0])
	applyMainCellTextStyle(cell, textChildren[0].Style, explicitAlign)
	if len(textChildren) > 1 {
		cell.SubText = l.ResolvePDFTextNode(textChildren[1])
		applySubCellTextStyle(cell, textChildren[1].Style)
	}
}

func applyMainCellTextStyle(cell *CellDef, style *pdfdom.PDFTextStyle, explicitAlign bool) {
	if style == nil {
		return
	}
	if style.FontFace != "" {
		cell.FontFace = style.FontFace
	}
	if style.FontStyle != "" {
		cell.FontStyle = style.FontStyle
	}
	if style.FontColor != "" {
		cell.FontColor = style.FontColor
	}
	if style.FontSize > 0 {
		cell.FontSize = style.FontSize
	}
	if style.LineHeight > 0 {
		cell.LineHeight = style.LineHeight
	}
	if style.Align != "" && !explicitAlign {
		cell.Align = textAlignToTableAlign(style.Align)
	}
	cell.Bold = cell.Bold || strings.Contains(style.FontStyle, "B")
	cell.Small = style.FontSize > 0 && style.FontSize <= 7
}

func applySubCellTextStyle(cell *CellDef, style *pdfdom.PDFTextStyle) {
	if style != nil {
		cell.SubFontFace, cell.SubFontColor, cell.SubFontSize = style.FontFace, style.FontColor, style.FontSize
	}
}

func firstTrimmedAttribute(elem pdfdom.PDFElementNode, names ...string) string {
	for _, name := range names {
		if value, ok := elem.Attribute(name); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseTableWidthAttr(node pdfdom.PDFElementNode, availableWidth float64) (float64, error) {
	value, ok := node.Attribute("width")
	if !ok {
		return 0, fmt.Errorf("missing required width attribute on %s", node.ElementType())
	}
	raw := strings.TrimSpace(value)
	if strings.HasSuffix(raw, "%") {
		percentage, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(raw, "%")), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid width percentage on %s: %q", node.ElementType(), value)
		}
		if percentage <= 0 {
			return 0, fmt.Errorf("width percentage must be greater than zero on %s: %q", node.ElementType(), value)
		}
		if availableWidth <= 0 {
			return 0, fmt.Errorf("cannot resolve width %q on %s without positive available width", value, node.ElementType())
		}
		return availableWidth * percentage / 100, nil
	}
	var parsed float64
	if _, err := fmt.Sscanf(raw, "%f", &parsed); err != nil {
		return 0, fmt.Errorf("invalid width attribute on %s: %q", node.ElementType(), value)
	}
	return parsed, nil
}

func newInvalidColWidthError(column *pdfdom.ElemCol, columnIndex int, widthValue string) error {
	line, marker := highlightHTMLAttribute(column, "width")
	if marker == "" {
		marker = "       ^"
	}
	return fmt.Errorf("invalid <col> width in table colgroup at column %d: got %q (expected a numeric width in points, or omit width for a flexible column)\n%s\n%s", columnIndex, widthValue, line, marker)
}

func highlightHTMLAttribute(node pdfdom.PDFElementNode, attributeName string) (string, string) {
	const reset, red = "\x1b[0m", "\x1b[31;1m"
	base := renderElementOpenTag(node)
	if strings.TrimSpace(attributeName) == "" {
		return "  " + base, ""
	}
	needle := attributeName + "=\""
	index := strings.Index(base, needle)
	if index < 0 {
		return "  " + base, ""
	}
	start := index + len(needle)
	endOffset := strings.Index(base[start:], "\"")
	if endOffset < 0 {
		return "  " + base, ""
	}
	end := start + endOffset
	value := base[start:end]
	length := len(value)
	if length == 0 {
		length = 1
	}
	line := "  " + base[:start] + red + value + reset + base[end:]
	marker := "  " + strings.Repeat(" ", start) + red + strings.Repeat("^", length) + reset
	return line, marker
}

func renderElementOpenTag(node pdfdom.PDFElementNode) string {
	if node == nil {
		return "<unknown>"
	}
	var builder strings.Builder
	builder.WriteByte('<')
	builder.WriteString(node.ElementType())
	for _, attribute := range node.ElementAttributes() {
		fmt.Fprintf(&builder, " %s=\"%s\"", attribute.Name, attribute.Value)
	}
	builder.WriteString(">")
	return builder.String()
}

func parseLayoutFloatAttr(node pdfdom.PDFElementNode, attribute string) (float64, error) {
	value, ok := node.Attribute(attribute)
	if !ok {
		return 0, fmt.Errorf("missing required %s attribute on %s", attribute, node.ElementType())
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return 0, fmt.Errorf("invalid %s attribute on %s: %q", attribute, node.ElementType(), value)
	}
	return parsed, nil
}

func tableCellFromNode(node pdfdom.PDFNode) (pdfdom.PDFElementNode, error) {
	switch cell := node.(type) {
	case *pdfdom.ElemTd:
		return cell, nil
	case *pdfdom.ElemTh:
		return cell, nil
	default:
		return nil, fmt.Errorf("table row contains non-cell child %T", node)
	}
}

func columnDefFromCell(cell pdfdom.PDFElementNode) ColumnDef {
	column := ColumnDef{}
	if width, ok := cell.Attribute("width"); ok {
		scanFloatAttribute(width, &column.Width)
	}
	if align, ok := cell.Attribute("align"); ok {
		column.Align = tableAlignFromAttr(align)
	}
	return column
}

// scanFloatAttribute leaves target unchanged when raw has no leading number.
func scanFloatAttribute(raw string, target *float64) {
	var parsed float64
	if _, err := fmt.Sscanf(raw, "%f", &parsed); err == nil {
		*target = parsed
	}
}

// scanIntAttribute leaves target unchanged when raw has no leading integer.
func scanIntAttribute(raw string, target *int) {
	var parsed int
	if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil {
		*target = parsed
	}
}

func (l *LayoutPDF) ResolvePDFTextNode(node *pdfdom.PDFTextNode) string {
	if node == nil {
		return ""
	}
	if node.I18nKey == "" {
		return node.Text
	}
	if len(node.I18nVars) > 0 {
		return l.I18n.TWithVars(node.I18nKey, node.I18nVars)
	}
	return l.I18n.T(node.I18nKey)
}

func textAlignToTableAlign(align pdfdom.TextAlign) string {
	switch align {
	case pdfdom.TextAlignCenter:
		return "C"
	case pdfdom.TextAlignRight:
		return "R"
	default:
		return "L"
	}
}

func tableAlignFromAttr(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "center", "c":
		return "C"
	case "right", "r":
		return "R"
	default:
		return "L"
	}
}
