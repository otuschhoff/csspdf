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
	Title        string
	Width        float64
	TableLayout  string
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
	FontFace      string
	FontStyle     string
	FontSize      float64
	FontColor     string
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
		height += defaultTableFontSize + 10
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

	requestedWidth := table.Width
	tableWidth := requestedWidth
	if tableWidth <= 0 {
		if len(flexIndices) == 0 {
			tableWidth = fixedTotal
		} else {
			pageW, _ := tr.pdf.GetPageSize()
			left, _, right, _ := tr.pdf.GetMargins()
			tableWidth = pageW - left - right
		}
	}
	if reqMin := fixedTotal + minFlexTotal; requestedWidth <= 0 && tableWidth < reqMin {
		tableWidth = reqMin
	}

	if strings.EqualFold(strings.TrimSpace(table.TableLayout), "auto") && len(flexIndices) > 0 {
		preferred := tr.preferredColumnWidths(table, widths, padding)
		availFlex := tableWidth - fixedTotal
		if availFlex < 0 {
			availFlex = 0
		}
		minTotal := 0.0
		desiredExtra := 0.0
		for _, idx := range flexIndices {
			minW := widths[idx]
			minTotal += minW
			target := preferred[idx]
			if target < minW {
				target = minW
			}
			desiredExtra += target - minW
		}

		extraAvail := availFlex - minTotal
		if extraAvail < 0 {
			extraAvail = 0
		}

		for _, idx := range flexIndices {
			minW := widths[idx]
			target := preferred[idx]
			if target < minW {
				target = minW
			}
			extra := target - minW
			if desiredExtra > 0 && extraAvail > 0 {
				widths[idx] = minW + extra*(extraAvail/desiredExtra)
			} else {
				widths[idx] = minW
			}
		}
	}

	remaining := tableWidth - fixedTotal - minFlexTotal
	if remaining > 0 && len(flexIndices) > 0 {
		if !strings.EqualFold(strings.TrimSpace(table.TableLayout), "auto") {
			extra := remaining / float64(len(flexIndices))
			for _, idx := range flexIndices {
				widths[idx] += extra
			}
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

	tr.enforceNoWrapColumnWidths(table, widths, padding)

	actual := 0.0
	for _, w := range widths {
		actual += w
	}
	if actual <= 0 {
		actual = tableWidth
	}

	// If columns don't fill the table width, scale them all up proportionally
	// so the table is always greedy and uses the full available width.
	if actual < tableWidth && actual > 0 {
		scale := tableWidth / actual
		for i := range widths {
			widths[i] *= scale
		}
		actual = tableWidth
	}

	return resolvedTableLayout{tableWidth: actual, padding: padding, rowHeightMin: rowHeightMin, colWidths: widths}
}

func (tr *TableRenderer) preferredColumnWidths(table *TableDef, currentWidths []float64, defaultPadding float64) []float64 {
	preferred := append([]float64(nil), currentWidths...)
	for _, row := range table.Rows {
		colIdx := 0
		for _, cell := range row.Cells {
			if colIdx >= len(preferred) {
				break
			}
			span := cell.Colspan
			if span < 1 {
				span = 1
			}
			if span == 1 {
				required := tr.measureNoWrapCellRequiredWidth(&cell, defaultPadding)
				if cell.NoWrap {
					if required > preferred[colIdx] {
						preferred[colIdx] = required
					}
				} else {
					// For wrapping cells, prefer larger width to reduce wraps,
					// while avoiding hard no-wrap behavior.
					target := required * 0.65
					if target < preferred[colIdx] {
						target = preferred[colIdx]
					}
					if target > 420 {
						target = 420
					}
					if target > preferred[colIdx] {
						preferred[colIdx] = target
					}
				}
			}
			colIdx += span
		}
	}
	return preferred
}

func (tr *TableRenderer) enforceNoWrapColumnWidths(table *TableDef, widths []float64, defaultPadding float64) {
	if table == nil || len(widths) == 0 {
		return
	}

	// First pass: calculate required widths for each column and identify nowrap columns
	requiredWidths := make([]float64, len(widths))
	hasNoWrap := make([]bool, len(widths))

	for _, row := range table.Rows {
		colIdx := 0
		for _, cell := range row.Cells {
			if colIdx >= len(widths) {
				break
			}
			span := cell.Colspan
			if span < 1 {
				span = 1
			}
			if span == 1 {
				required := tr.measureNoWrapCellRequiredWidth(&cell, defaultPadding)
				if required > requiredWidths[colIdx] {
					requiredWidths[colIdx] = required
				}
				if cell.NoWrap {
					hasNoWrap[colIdx] = true
				}
			}
			colIdx += span
		}
	}

	// Second pass: separate nowrap and wrapping columns
	nowrapRequired := 0.0
	nowrapIndices := []int{}
	wrappingIndices := []int{}

	for i := range widths {
		if hasNoWrap[i] {
			nowrapRequired += requiredWidths[i]
			nowrapIndices = append(nowrapIndices, i)
		} else {
			wrappingIndices = append(wrappingIndices, i)
		}
	}

	totalOriginal := 0.0
	for _, w := range widths {
		totalOriginal += w
	}

	if nowrapRequired <= totalOriginal {
		remainingForWrapping := totalOriginal - nowrapRequired
		for _, i := range nowrapIndices {
			widths[i] = requiredWidths[i]
		}
		if len(wrappingIndices) > 0 {
			wrappingOriginal := 0.0
			for _, i := range wrappingIndices {
				wrappingOriginal += widths[i]
			}
			if wrappingOriginal > 0 {
				scale := remainingForWrapping / wrappingOriginal
				for _, i := range wrappingIndices {
					widths[i] *= scale
				}
			} else {
				share := remainingForWrapping / float64(len(wrappingIndices))
				for _, i := range wrappingIndices {
					widths[i] = share
				}
			}
		}
		return
	}

	// If nowrap requirements exceed available width, prioritise nowrap columns
	// proportionally and collapse wrapping columns.
	if len(nowrapIndices) == 0 {
		return
	}
	scale := 0.0
	if nowrapRequired > 0 {
		scale = totalOriginal / nowrapRequired
	}
	for _, i := range nowrapIndices {
		widths[i] = requiredWidths[i] * scale
	}
	for _, i := range wrappingIndices {
		widths[i] = 0
	}
}

func (tr *TableRenderer) measureNoWrapCellRequiredWidth(cell *CellDef, defaultPadding float64) float64 {
	if cell == nil {
		return 0
	}
	topP, rightP, _, leftP := tr.resolvedCellPadding(cell, defaultPadding)
	_ = topP
	_, _ = tr.applyCellStyle(cell)

	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	maxContent := tr.maxLineWidthNoWrap(text)

	if cell.SubText != "" {
		subFace := cell.SubFontFace
		if subFace == "" {
			subFace = cell.FontFace
		}
		if subFace == "" {
			subFace = defaultTableFontFace
		}
		subSize := cell.SubFontSize
		if subSize <= 0 {
			subSize = 7
		}
		tr.pdf.SetFont(normalizeTableFontFace(subFace), "", subSize)
		if w := tr.maxLineWidthNoWrap(cell.SubText); w > maxContent {
			maxContent = w
		}
	}

	return maxContent + leftP + rightP
}

func (tr *TableRenderer) maxLineWidthNoWrap(text string) float64 {
	encoded := tableEncodePDFTextLatin1(text)
	if encoded == "" {
		return 0
	}
	maxWidth := 0.0
	for _, line := range strings.Split(encoded, "\n") {
		if w := tr.pdf.GetStringWidth(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
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
	fontSize, lineHeightMul := tr.applyCellStyle(cell)

	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	lines := tr.wrapTextLines(text, contentWidth, cell.NoWrap)
	lineH := fontSize * lineHeightMul

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
		subLines := tr.wrapTextLines(cell.SubText, contentWidth, cell.NoWrap)
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

func (tr *TableRenderer) wrapTextLines(text string, width float64, noWrap bool) []string {
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
		if noWrap {
			out = append(out, seg)
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
	fontSize, lineHeightMul := tr.applyCellStyle(cell)

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
	lines := tr.wrapTextLines(text, contentWidth, cell.NoWrap)
	lineH := fontSize * lineHeightMul
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
			subFace = cell.FontFace
		}
		if subFace == "" {
			subFace = defaultTableFontFace
		}
		subColor := cell.SubFontColor
		if subColor == "" {
			subColor = cell.FontColor
		}
		if subColor == "" {
			subColor = "#666"
		}
		subSize := cell.SubFontSize
		if subSize <= 0 {
			subSize = math.Max(7, fontSize-3)
		}
		tr.pdf.SetFont(normalizeTableFontFace(subFace), "", subSize)
		if len(subColor) >= 4 && subColor[0] == '#' {
			r, g, b := tableHexToRGB(subColor)
			tr.pdf.SetTextColor(r, g, b)
		}
		subLines := tr.wrapTextLines(cell.SubText, contentWidth, cell.NoWrap)
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
