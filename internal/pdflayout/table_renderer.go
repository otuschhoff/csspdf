package pdflayout

import (
	"fmt"
	"math"
	"strings"

	"github.com/otuschhoff/gofpdf"
	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
)

// CellFormatter formats typed cell values into locale-aware strings.
// Implemented by *invoice.Formatter; defined here to avoid an import cycle.
type CellFormatter interface {
	FormatCurrency(value float64) string
	FormatFloat(value float64, decimals int) string
	FormatDate(dateStr string, long bool) string
	FormatWorkWeek(dateStr string) string
	FormatDuration(hours float64) string
}

// TableStyle holds the three style variants used by the table renderer.
// Use pdfdom.StyleVariant values from the host application's style sheet.
type TableStyle struct {
	Title  pdfdom.StyleVariant
	Normal pdfdom.StyleVariant
	Small  pdfdom.StyleVariant
}

// TableRenderer handles rendering tables in PDF documents.
type TableRenderer struct {
	pdf       *gofpdf.Fpdf
	style     *TableStyle
	formatter CellFormatter
}

// NewTableRenderer creates a new table renderer.
func NewTableRenderer(pdf *gofpdf.Fpdf, style *TableStyle, formatter CellFormatter) *TableRenderer {
	return &TableRenderer{pdf: pdf, style: style, formatter: formatter}
}

// TableDef defines a table structure.
type TableDef struct {
	Title        string
	Width        float64
	Padding      float64
	RowHeightMin float64
	Background   string
	BorderColor  string
	BorderStyle  string
	BorderWidth  float64
	Columns      []ColumnDef
	Rows         []RowDef
}

// ColumnDef defines a column.
type ColumnDef struct {
	Width    float64
	MinWidth float64
	MaxWidth float64
	Align    string
}

// RowDef defines a row.
type RowDef struct {
	Cells      []CellDef
	Height     float64
	Background string
	Fill       string
	Border     bool
	Stroke     *bool
}

// CellDef defines a cell.
type CellDef struct {
	Text          string
	SubText       string
	Value         interface{}
	Type          string
	Align         string
	PaddingTop    float64
	PaddingRight  float64
	PaddingBottom float64
	PaddingLeft   float64
	Bold          bool
	Small         bool
	SubFontFace   string
	SubFontColor  string
	SubFontSize   float64
	Colspan       int
}

// RenderTable renders a table at the current PDF cursor position.
func (tr *TableRenderer) RenderTable(table *TableDef) error {
	if table == nil {
		return fmt.Errorf("table cannot be nil")
	}
	if len(table.Columns) == 0 {
		return fmt.Errorf("table must define at least one column")
	}

	startX := tr.pdf.GetX()
	startY := tr.pdf.GetY()
	layout := tr.resolveTableLayout(table)

	if table.Title != "" {
		tr.setStyle(tr.style.Title)
		tr.pdf.Text(startX, startY+float64(tr.style.Title.FontSize), table.Title)
		startY += float64(tr.style.Title.FontSize) + 10
		tr.pdf.SetY(startY)
	}

	rowHeights := make([]float64, len(table.Rows))
	totalTableHeight := 0.0
	for rowIdx, row := range table.Rows {
		rh := row.Height
		if rh == 0 {
			rh = layout.rowHeightMin
		}
		if calc := tr.calculateRowHeight(&row, rh, layout.padding, layout.colWidths); calc > rh {
			rh = calc
		}
		rowHeights[rowIdx] = rh
		totalTableHeight += rh
	}

	tr.drawTableBackgroundAndBorder(startX, startY, layout.tableWidth, totalTableHeight, table)

	currentY := startY
	for rowIdx, row := range table.Rows {
		rh := rowHeights[rowIdx]
		bgColor := row.Fill
		if bgColor == "" {
			bgColor = row.Background
		}
		drawStroke := row.Border
		if row.Stroke != nil {
			drawStroke = *row.Stroke
		}
		if bgColor != "" || drawStroke {
			tr.drawRowBackground(startX, currentY, layout.tableWidth, rh, bgColor, drawStroke)
		}
		currentX := startX
		for colIdx, cell := range row.Cells {
			if colIdx >= len(layout.colWidths) {
				break
			}
			colWidth := tr.getColumnWidth(layout.colWidths, colIdx)
			if cell.Colspan > 1 {
				for i := 1; i < cell.Colspan && (colIdx+i) < len(layout.colWidths); i++ {
					colWidth += tr.getColumnWidth(layout.colWidths, colIdx+i)
				}
			}
			tr.renderCell(&cell, currentX, currentY, colWidth, rh, layout.padding)
			currentX += tr.getColumnWidth(layout.colWidths, colIdx)
		}
		currentY += rh
	}
	tr.pdf.SetY(currentY + 5)
	return nil
}

// MeasureTableHeight returns the height that RenderTable would consume
// (excluding the trailing 5pt cursor gap).
func (tr *TableRenderer) MeasureTableHeight(table *TableDef) (float64, error) {
	if table == nil {
		return 0, fmt.Errorf("table cannot be nil")
	}
	if len(table.Columns) == 0 {
		return 0, fmt.Errorf("table must define at least one column")
	}
	layout := tr.resolveTableLayout(table)
	height := 0.0
	if table.Title != "" {
		height += float64(tr.style.Title.FontSize) + 10
	}
	for _, row := range table.Rows {
		rh := row.Height
		if rh == 0 {
			rh = layout.rowHeightMin
		}
		if calc := tr.calculateRowHeight(&row, rh, layout.padding, layout.colWidths); calc > rh {
			rh = calc
		}
		height += rh
	}
	return height, nil
}

type resolvedTableLayout struct {
	tableWidth   float64
	padding      float64
	rowHeightMin float64
	colWidths    []float64
}

func (tr *TableRenderer) resolveTableLayout(table *TableDef) resolvedTableLayout {
	padding := table.Padding
	if padding <= 0 {
		padding = 5.0
	}
	rowHeightMin := table.RowHeightMin
	if rowHeightMin <= 0 {
		rowHeightMin = 20.0
	}

	fixedTotal, minFlexTotal := 0.0, 0.0
	flexIndices := make([]int, 0, len(table.Columns))
	widths := make([]float64, len(table.Columns))

	for i, col := range table.Columns {
		if col.Width > 0 {
			widths[i] = col.Width
			fixedTotal += col.Width
			continue
		}
		minW := col.MinWidth
		if minW < 0 {
			minW = 0
		}
		widths[i] = minW
		minFlexTotal += minW
		flexIndices = append(flexIndices, i)
	}

	tableWidth := table.Width
	if tableWidth <= 0 {
		if len(flexIndices) == 0 {
			tableWidth = fixedTotal
		} else {
			pageW, _ := tr.pdf.GetPageSize()
			left, _, right, _ := tr.pdf.GetMargins()
			tableWidth = pageW - left - right
		}
	}
	if reqMin := fixedTotal + minFlexTotal; tableWidth < reqMin {
		tableWidth = reqMin
	}

	remaining := tableWidth - fixedTotal - minFlexTotal
	if remaining > 0 && len(flexIndices) > 0 {
		extra := remaining / float64(len(flexIndices))
		for _, idx := range flexIndices {
			widths[idx] += extra
		}
	}

	// Honour MaxWidth by redistributing excess to uncapped flex columns.
	for {
		cappedAny := false
		reclaim := 0.0
		avail := make([]int, 0, len(flexIndices))
		for _, idx := range flexIndices {
			maxW := table.Columns[idx].MaxWidth
			if maxW > 0 && widths[idx] > maxW {
				reclaim += widths[idx] - maxW
				widths[idx] = maxW
				cappedAny = true
				continue
			}
			avail = append(avail, idx)
		}
		if !cappedAny || reclaim <= 0 || len(avail) == 0 {
			break
		}
		extra := reclaim / float64(len(avail))
		for _, idx := range avail {
			widths[idx] += extra
		}
	}

	actual := 0.0
	for _, w := range widths {
		actual += w
	}
	if actual <= 0 {
		actual = tableWidth
	}
	return resolvedTableLayout{tableWidth: actual, padding: padding, rowHeightMin: rowHeightMin, colWidths: widths}
}

func (tr *TableRenderer) calculateRowHeight(row *RowDef, minH, padding float64, colWidths []float64) float64 {
	max := minH
	for colIdx, cell := range row.Cells {
		if colIdx >= len(colWidths) {
			break
		}
		if h := tr.measureCellHeight(&cell, tr.getColumnWidth(colWidths, colIdx), padding); h > max {
			max = h
		}
	}
	return max
}

func (tr *TableRenderer) resolvedCellPadding(cell *CellDef, def float64) (top, right, bottom, left float64) {
	top, right, bottom, left = def, def, def, def
	if cell == nil {
		return
	}
	if cell.PaddingTop > 0 {
		top = cell.PaddingTop
	}
	if cell.PaddingRight > 0 {
		right = cell.PaddingRight
	}
	if cell.PaddingBottom > 0 {
		bottom = cell.PaddingBottom
	}
	if cell.PaddingLeft > 0 {
		left = cell.PaddingLeft
	}
	return
}

func (tr *TableRenderer) measureCellHeight(cell *CellDef, width, padding float64) float64 {
	topP, rightP, bottomP, leftP := tr.resolvedCellPadding(cell, padding)
	contentWidth := math.Max(width-leftP-rightP, 1)

	if cell.Bold {
		tr.pdf.SetFont(tr.style.Normal.FontFace, "B", float64(tr.style.Normal.FontSize))
		tr.pdf.SetTextColor(0, 0, 0)
	} else if cell.Small {
		tr.setStyle(tr.style.Small)
	} else {
		tr.setStyle(tr.style.Normal)
	}

	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	lines := tr.wrapTextLines(text, contentWidth)
	_, fontSize := tr.pdf.GetFontSize()
	lineH := fontSize * 1.2

	height := topP
	if len(lines) > 0 {
		height += float64(len(lines)) * lineH
	}

	if cell.SubText != "" {
		subFace := cell.SubFontFace
		if subFace == "" {
			subFace = "Helvetica"
		}
		subSize := cell.SubFontSize
		if subSize <= 0 {
			subSize = 7
		}
		tr.pdf.SetFont(subFace, "", subSize)
		subLines := tr.wrapTextLines(cell.SubText, contentWidth)
		subLineH := subSize * 1.2
		if len(subLines) > 0 {
			if len(lines) > 0 {
				height += lineH
			}
			height += float64(len(subLines)) * subLineH
		}
	}

	return height + bottomP
}

func (tr *TableRenderer) wrapTextLines(text string, width float64) []string {
	encoded := tableEncodePDFTextLatin1(text)
	if encoded == "" {
		return nil
	}
	if width <= 0 {
		return []string{encoded}
	}
	segments := strings.Split(encoded, "\n")
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			out = append(out, "")
			continue
		}
		wrapped := tr.pdf.SplitLines([]byte(seg), width)
		if len(wrapped) == 0 {
			out = append(out, "")
			continue
		}
		for _, l := range wrapped {
			out = append(out, string(l))
		}
	}
	return out
}

func (tr *TableRenderer) renderCell(cell *CellDef, x, y, width, height, padding float64) {
	if cell.Bold {
		tr.pdf.SetFont(tr.style.Normal.FontFace, "B", float64(tr.style.Normal.FontSize))
		tr.pdf.SetTextColor(0, 0, 0)
	} else if cell.Small {
		tr.setStyle(tr.style.Small)
	} else {
		tr.setStyle(tr.style.Normal)
	}

	align := cell.Align
	if align == "" && len(cell.Text) > 0 {
		align = "L"
	}

	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	topP, rightP, _, leftP := tr.resolvedCellPadding(cell, padding)
	contentWidth := math.Max(width-leftP-rightP, 1)
	lines := tr.wrapTextLines(text, contentWidth)

	_, fontSize := tr.pdf.GetFontSize()
	lineH := fontSize * 1.2
	baseline := y + topP + fontSize

	for _, line := range lines {
		lx := x + leftP
		if align == "R" {
			lx = x + width - rightP - tr.pdf.GetStringWidth(line)
		} else if align == "C" {
			lx = x + leftP + (contentWidth-tr.pdf.GetStringWidth(line))/2
		}
		tr.pdf.Text(lx, baseline, line)
		baseline += lineH
	}

	if cell.SubText != "" {
		subFace := cell.SubFontFace
		if subFace == "" {
			subFace = "Helvetica"
		}
		subColor := cell.SubFontColor
		if subColor == "" {
			subColor = "#666"
		}
		subSize := cell.SubFontSize
		if subSize <= 0 {
			subSize = 7
		}
		tr.pdf.SetFont(subFace, "", subSize)
		if len(subColor) >= 4 && subColor[0] == '#' {
			r, g, b := tableHexToRGB(subColor)
			tr.pdf.SetTextColor(r, g, b)
		}
		subLines := tr.wrapTextLines(cell.SubText, contentWidth)
		subLineH := subSize * 1.2
		for _, sl := range subLines {
			sx := x + leftP
			if align == "R" {
				sx = x + width - rightP - tr.pdf.GetStringWidth(sl)
			} else if align == "C" {
				sx = x + leftP + (contentWidth-tr.pdf.GetStringWidth(sl))/2
			}
			tr.pdf.Text(sx, baseline, sl)
			baseline += subLineH
		}
	}
}

func (tr *TableRenderer) formatCellValue(cell *CellDef) string {
	switch cell.Type {
	case "currency":
		if v, ok := cell.Value.(float64); ok {
			return tr.formatter.FormatCurrency(v)
		}
	case "float":
		if v, ok := cell.Value.(float64); ok {
			return tr.formatter.FormatFloat(v, 2)
		}
	case "date":
		if v, ok := cell.Value.(string); ok {
			return tr.formatter.FormatDate(v, false)
		}
	case "workweek":
		if v, ok := cell.Value.(string); ok {
			return tr.formatter.FormatWorkWeek(v)
		}
	case "duration":
		if v, ok := cell.Value.(float64); ok {
			return tr.formatter.FormatDuration(v)
		}
	}
	return fmt.Sprintf("%v", cell.Value)
}

func (tr *TableRenderer) getColumnWidth(colWidths []float64, colIdx int) float64 {
	if colIdx >= len(colWidths) {
		return 0
	}
	return colWidths[colIdx]
}

func (tr *TableRenderer) drawRowBackground(x, y, width, height float64, bgColor string, drawStroke bool) {
	style := ""
	if strings.TrimSpace(bgColor) != "" {
		r, g, b := tableHexToRGB(bgColor)
		tr.pdf.SetFillColor(r, g, b)
		style = "F"
	}
	if drawStroke {
		tr.pdf.SetDrawColor(153, 153, 153)
		tr.pdf.SetLineWidth(0.5)
		if style == "F" {
			style = "FD"
		} else {
			style = "D"
		}
	}
	if style != "" {
		tr.pdf.Rect(x, y, width, height, style)
	}
}

func (tr *TableRenderer) drawTableBackgroundAndBorder(x, y, width, height float64, table *TableDef) {
	if table == nil || width <= 0 || height <= 0 {
		return
	}
	hasFill := strings.TrimSpace(table.Background) != ""
	hasBorder := strings.TrimSpace(table.BorderStyle) != "" &&
		strings.ToLower(strings.TrimSpace(table.BorderStyle)) != "none"
	if !hasFill && !hasBorder {
		return
	}
	style := ""
	if hasFill {
		r, g, b := tableHexToRGB(table.Background)
		tr.pdf.SetFillColor(r, g, b)
		style = "F"
	}
	if hasBorder {
		bc := table.BorderColor
		if strings.TrimSpace(bc) == "" {
			bc = "#000"
		}
		r, g, b := tableHexToRGB(bc)
		tr.pdf.SetDrawColor(r, g, b)
		bw := table.BorderWidth
		if bw <= 0 {
			bw = 1
		}
		tr.pdf.SetLineWidth(bw)
		if style == "F" {
			style = "FD"
		} else {
			style = "D"
		}
	}
	tr.pdf.Rect(x, y, width, height, style)
}

func (tr *TableRenderer) setStyle(style pdfdom.StyleVariant) {
	face := style.FontFace
	if face == "Futura-Medium" || face == "Futura" {
		face = "Helvetica"
	}
	tr.pdf.SetFont(face, "", float64(style.FontSize))
	if len(style.FontColor) >= 4 && style.FontColor[0] == '#' {
		r, g, b := tableHexToRGB(style.FontColor)
		tr.pdf.SetTextColor(r, g, b)
	}
}

// tableHexToRGB parses a CSS hex color string and returns r, g, b in [0,255].
func tableHexToRGB(hex string) (int, int, int) {
	hex = strings.ToLower(strings.TrimSpace(hex))
	if hex == "" {
		return 0, 0, 0
	}
	named := map[string][3]int{
		"black": {0, 0, 0}, "white": {255, 255, 255},
		"gray": {128, 128, 128}, "grey": {128, 128, 128},
		"lightgray": {211, 211, 211}, "lightgrey": {211, 211, 211},
		"darkgray": {169, 169, 169}, "darkgrey": {169, 169, 169},
		"red": {255, 0, 0}, "green": {0, 128, 0}, "blue": {0, 0, 255},
	}
	if v, ok := named[hex]; ok {
		return v[0], v[1], v[2]
	}
	h := hex
	if len(h) > 0 && h[0] == '#' {
		h = h[1:]
	}
	var r, g, b int
	if len(h) == 3 {
		fmt.Sscanf(h, "%1x%1x%1x", &r, &g, &b)
		r, g, b = r*17, g*17, b*17
	} else if len(h) == 6 {
		fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b)
	}
	return r, g, b
}

// tableEncodePDFTextLatin1 converts a UTF-8 string to a CP-1252/Latin-1 byte
// string suitable for gofpdf's standard fonts.
func tableEncodePDFTextLatin1(text string) string {
	if text == "" {
		return ""
	}
	cp1252 := map[rune]byte{
		8364: 0x80, 8218: 0x82, 8222: 0x84, 8230: 0x85, 8224: 0x86, 8225: 0x87,
		710: 0x88, 8240: 0x89, 352: 0x8A, 8249: 0x8B, 338: 0x8C, 381: 0x8E,
		8216: 0x91, 8217: 0x92, 8220: 0x93, 8221: 0x94,
		8226: 0x95, 8211: 0x96, 8212: 0x97, 732: 0x98, 8482: 0x99,
		353: 0x9A, 8250: 0x9B, 339: 0x9C, 382: 0x9E, 376: 0x9F,
	}
	var sb strings.Builder
	sb.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			sb.WriteRune(r)
		case r <= 0xFF:
			sb.WriteByte(byte(r))
		default:
			if b, ok := cp1252[r]; ok {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('?')
			}
		}
	}
	return sb.String()
}
