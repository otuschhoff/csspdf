package pdfrender

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/otuschhoff/gofpdf"
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

// TableRenderer handles rendering tables in PDF documents.
type TableRenderer struct {
	pdf       *gofpdf.Fpdf
	formatter CellFormatter
}

// NewTableRenderer creates a new table renderer.
func NewTableRenderer(pdf *gofpdf.Fpdf, formatter CellFormatter) *TableRenderer {
	return &TableRenderer{pdf: pdf, formatter: formatter}
}

// TableDef defines a table structure.
type TableDef struct {
	Title           string
	Width           float64
	TableLayout     string
	Padding         float64
	RowHeightMin    float64
	MarginTop       float64
	MarginBottom    float64
	MarginTopSet    bool
	MarginBottomSet bool
	Background      string
	BorderColor     string
	BorderStyle     string
	BorderWidth     float64
	Columns         []ColumnDef
	Rows            []RowDef
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
	Header     bool
	Background string
	Fill       string
	Border     bool
	Stroke     *bool
}

// CellDef defines a cell.
type CellDef struct {
	Text          string
	SubText       string
	FontFace      string
	FontStyle     string
	FontSize      float64
	FontColor     string
	Background    string
	LineHeight    float64
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
	NoWrap        bool
	Colspan       int
}

const (
	defaultTableFontFace      = "Helvetica"
	defaultTableFontSize      = 10.0
	defaultTableLineHeightMul = 1.2
)

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
	if err := validateTableSpans(table, len(layout.colWidths)); err != nil {
		return err
	}

	if table.Title != "" {
		tr.pdf.SetFont(defaultTableFontFace, "", defaultTableFontSize)
		tr.pdf.SetTextColor(0, 0, 0)
		tr.pdf.Text(startX, startY+defaultTableFontSize, table.Title)
		startY += defaultTableFontSize + 10
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
		for _, placement := range tableCellPlacements(&row, layout.colWidths) {
			cell := &row.Cells[placement.cellIndex]
			tr.renderCell(cell, startX+placement.offset, currentY, placement.width, rh, layout.padding)
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
	rowHeights, err := tr.resolvedRowHeights(table)
	if err != nil {
		return 0, err
	}
	height := 0.0
	if table.Title != "" {
		height += defaultTableFontSize + 10
	}
	for _, rowHeight := range rowHeights {
		height += rowHeight
	}
	return height, nil
}

func (tr *TableRenderer) wrapTextLines(text string, width float64, noWrap bool) []string {
	normalized := tr.normalizeTableTextForCurrentFont(text)
	if normalized == "" {
		return nil
	}
	if width <= 0 {
		return []string{normalized}
	}
	segments := strings.Split(normalized, "\n")
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			out = append(out, "")
			continue
		}
		if noWrap {
			out = append(out, seg)
			continue
		}
		if tr.pdf.CurrentFontIsUTF8() {
			wrapped := tr.pdf.SplitText(seg, width)
			if len(wrapped) == 0 {
				out = append(out, "")
				continue
			}
			out = append(out, wrapped...)
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

func (tr *TableRenderer) normalizeTableTextForCurrentFont(text string) string {
	if text == "" {
		return ""
	}
	if tr.pdf.CurrentFontIsUTF8() {
		return text
	}
	return tableEncodePDFTextLatin1(text)
}

func (tr *TableRenderer) renderCell(cell *CellDef, x, y, width, height, padding float64) {
	if cell == nil {
		return
	}
	if strings.TrimSpace(cell.Background) != "" {
		r, g, b := tableHexToRGB(cell.Background)
		tr.pdf.SetFillColor(r, g, b)
		tr.pdf.Rect(x, y, width, height, "F")
	}

	fontSize, lineHeightMul := tr.applyCellStyle(cell)

	align := resolvedCellTextAlign(cell)
	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	topP, rightP, _, leftP := tr.resolvedCellPadding(cell, padding)
	contentWidth := math.Max(width-leftP-rightP, 1)
	baseline := y + topP + fontSize
	baseline = tr.renderCellTextLines(text, align, cell.NoWrap, x, width, leftP, rightP, contentWidth, baseline, fontSize*lineHeightMul)
	if cell.SubText != "" {
		tr.renderCellSubText(cell, align, x, width, leftP, rightP, contentWidth, baseline, fontSize)
	}
}

func resolvedCellTextAlign(cell *CellDef) string {
	if cell.Align == "" && cell.Text != "" {
		return "L"
	}
	return cell.Align
}

func (tr *TableRenderer) renderCellTextLines(text, align string, noWrap bool, x, width, left, right, contentWidth, baseline, lineHeight float64) float64 {
	for _, line := range tr.wrapTextLines(text, contentWidth, noWrap) {
		tr.pdf.Text(tr.alignedCellTextX(line, align, x, width, left, right, contentWidth), baseline, line)
		baseline += lineHeight
	}
	return baseline
}

func (tr *TableRenderer) alignedCellTextX(text, align string, x, width, left, right, contentWidth float64) float64 {
	switch align {
	case "R":
		return x + width - right - tr.pdf.GetStringWidth(text)
	case "C":
		return x + left + (contentWidth-tr.pdf.GetStringWidth(text))/2
	default:
		return x + left
	}
}

func (tr *TableRenderer) renderCellSubText(cell *CellDef, align string, x, width, left, right, contentWidth, baseline, mainSize float64) {
	face := firstNonEmpty(cell.SubFontFace, cell.FontFace, defaultTableFontFace)
	color := firstNonEmpty(cell.SubFontColor, cell.FontColor, "#666")
	size := cell.SubFontSize
	if size <= 0 {
		size = math.Max(7, mainSize-3)
	}
	tr.pdf.SetFont(normalizeTableFontFace(face), "", size)
	if len(color) >= 4 && color[0] == '#' {
		red, green, blue := tableHexToRGB(color)
		tr.pdf.SetTextColor(red, green, blue)
	}
	tr.renderCellTextLines(cell.SubText, align, cell.NoWrap, x, width, left, right, contentWidth, baseline, size*1.2)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (tr *TableRenderer) applyCellStyle(cell *CellDef) (fontSize float64, lineHeightMul float64) {
	fontFace := defaultTableFontFace
	fontStyle := ""
	fontSize = defaultTableFontSize
	fontColor := "#000"
	lineHeightMul = defaultTableLineHeightMul

	if cell != nil {
		if strings.TrimSpace(cell.FontFace) != "" {
			fontFace = cell.FontFace
		}
		if strings.TrimSpace(cell.FontStyle) != "" {
			fontStyle = cell.FontStyle
		}
		if cell.FontSize > 0 {
			fontSize = cell.FontSize
		}
		if strings.TrimSpace(cell.FontColor) != "" {
			fontColor = cell.FontColor
		}
		if cell.LineHeight > 0 {
			lineHeightMul = cell.LineHeight
		}
		if cell.Small && cell.FontSize <= 0 {
			fontSize = 7
		}
		if cell.Bold {
			fontStyle = mergeTableBoldFontStyle(fontStyle)
		}
	}

	tr.pdf.SetFont(normalizeTableFontFace(fontFace), normalizeTableFontStyle(fontStyle), fontSize)
	if len(fontColor) >= 4 && fontColor[0] == '#' {
		r, g, b := tableHexToRGB(fontColor)
		tr.pdf.SetTextColor(r, g, b)
	} else {
		tr.pdf.SetTextColor(0, 0, 0)
	}
	return fontSize, lineHeightMul
}

func mergeTableBoldFontStyle(style string) string {
	n := normalizeTableFontStyle(style)
	if strings.Contains(n, "B") {
		return n
	}
	return n + "B"
}

func normalizeTableFontFace(face string) string {
	f := strings.TrimSpace(face)
	if f == "" {
		return defaultTableFontFace
	}
	if f == "Futura-Medium" || f == "Futura" {
		return "Helvetica"
	}
	return f
}

func normalizeTableFontStyle(style string) string {
	s := strings.TrimSpace(style)
	if s == "" {
		return ""
	}
	up := strings.ToUpper(s)
	if up == "NORMAL" {
		return ""
	}
	fields := strings.FieldsFunc(up, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '|'
	})
	if len(fields) == 0 {
		fields = []string{up}
	}
	hasB := strings.Contains(up, "B")
	hasI := strings.Contains(up, "I")
	hasU := strings.Contains(up, "U")
	for _, f := range fields {
		switch f {
		case "B", "BOLD", "700", "800", "900":
			hasB = true
		case "I", "ITALIC", "OBLIQUE":
			hasI = true
		case "U", "UNDERLINE":
			hasU = true
		case "NORMAL", "400":
			// no-op
		}
	}
	var b strings.Builder
	if hasB {
		b.WriteByte('B')
	}
	if hasI {
		b.WriteByte('I')
	}
	if hasU {
		b.WriteByte('U')
	}
	return b.String()
}

func parseTableFloat(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return v
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
