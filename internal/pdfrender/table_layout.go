package pdfrender

import (
	"math"
	"strings"
)

type resolvedTableLayout struct {
	tableWidth, padding, rowHeightMin float64
	colWidths                         []float64
}

func (tr *TableRenderer) resolveTableLayout(table *TableDef) resolvedTableLayout {
	padding, rowHeight := tableLayoutDefaults(table)
	widths, flexible, fixed, minimumFlexible := initialColumnWidths(table.Columns)
	tableWidth := tr.resolveRequestedTableWidth(table.Width, fixed, minimumFlexible, len(flexible))
	if strings.EqualFold(strings.TrimSpace(table.TableLayout), "auto") {
		tr.allocatePreferredFlexWidths(table, widths, flexible, padding, tableWidth-fixed)
	} else {
		distributeRemainingWidth(widths, flexible, tableWidth-fixed-minimumFlexible)
	}
	capFlexibleWidths(table.Columns, widths, flexible)
	tr.enforceNoWrapColumnWidths(table, widths, padding)
	return resolvedTableLayout{tableWidth: scaleColumnsToTableWidth(widths, tableWidth), padding: padding, rowHeightMin: rowHeight, colWidths: widths}
}

func tableLayoutDefaults(table *TableDef) (float64, float64) {
	padding, rowHeight := table.Padding, table.RowHeightMin
	if padding <= 0 {
		padding = 5
	}
	if rowHeight <= 0 {
		rowHeight = 20
	}
	return padding, rowHeight
}

func initialColumnWidths(columns []ColumnDef) ([]float64, []int, float64, float64) {
	widths, flexible := make([]float64, len(columns)), make([]int, 0, len(columns))
	fixed, minimumFlexible := 0.0, 0.0
	for index, column := range columns {
		if column.Width > 0 {
			widths[index], fixed = column.Width, fixed+column.Width
			continue
		}
		widths[index] = math.Max(column.MinWidth, 0)
		minimumFlexible += widths[index]
		flexible = append(flexible, index)
	}
	return widths, flexible, fixed, minimumFlexible
}

func (tr *TableRenderer) resolveRequestedTableWidth(requested, fixed, minimumFlexible float64, flexibleCount int) float64 {
	width := requested
	if width <= 0 && flexibleCount == 0 {
		width = fixed
	}
	if width <= 0 {
		pageWidth, _ := tr.pdf.GetPageSize()
		left, _, right, _ := tr.pdf.GetMargins()
		width = pageWidth - left - right
	}
	if requested <= 0 {
		width = math.Max(width, fixed+minimumFlexible)
	}
	return width
}

func (tr *TableRenderer) allocatePreferredFlexWidths(table *TableDef, widths []float64, indices []int, padding, available float64) {
	if len(indices) == 0 {
		return
	}
	preferred := tr.preferredColumnWidths(table, widths, padding)
	minimum, desiredExtra := 0.0, 0.0
	for _, index := range indices {
		minimum += widths[index]
		desiredExtra += math.Max(preferred[index]-widths[index], 0)
	}
	extraAvailable := math.Max(available-minimum, 0)
	for _, index := range indices {
		extra := math.Max(preferred[index]-widths[index], 0)
		if desiredExtra > 0 && extraAvailable > 0 {
			widths[index] += extra * extraAvailable / desiredExtra
		}
	}
}

func distributeRemainingWidth(widths []float64, indices []int, remaining float64) {
	if remaining <= 0 || len(indices) == 0 {
		return
	}
	share := remaining / float64(len(indices))
	for _, index := range indices {
		widths[index] += share
	}
}

func capFlexibleWidths(columns []ColumnDef, widths []float64, indices []int) {
	for {
		reclaim, available := capFlexibleWidthPass(columns, widths, indices)
		if reclaim <= 0 || len(available) == 0 {
			return
		}
		distributeRemainingWidth(widths, available, reclaim)
	}
}

func capFlexibleWidthPass(columns []ColumnDef, widths []float64, indices []int) (float64, []int) {
	reclaim, available := 0.0, make([]int, 0, len(indices))
	for _, index := range indices {
		maximum := columns[index].MaxWidth
		if maximum > 0 && widths[index] > maximum {
			reclaim += widths[index] - maximum
			widths[index] = maximum
		} else {
			available = append(available, index)
		}
	}
	return reclaim, available
}

func scaleColumnsToTableWidth(widths []float64, tableWidth float64) float64 {
	actual := sumWidths(widths)
	if actual <= 0 {
		return tableWidth
	}
	if actual < tableWidth {
		scale := tableWidth / actual
		for index := range widths {
			widths[index] *= scale
		}
		return tableWidth
	}
	return actual
}

func (tr *TableRenderer) preferredColumnWidths(table *TableDef, widths []float64, padding float64) []float64 {
	preferred := append([]float64(nil), widths...)
	for _, row := range table.Rows {
		columnIndex := 0
		for _, cell := range row.Cells {
			span := normalizedColspan(cell.Colspan)
			if columnIndex >= len(preferred) || columnIndex+span > len(preferred) {
				break
			}
			required := tr.measureNoWrapCellRequiredWidth(&cell, padding)
			if !cell.NoWrap {
				required = math.Min(required*0.65, 420)
			}
			current := sumWidths(preferred[columnIndex : columnIndex+span])
			if required > current {
				extra := (required - current) / float64(span)
				for index := columnIndex; index < columnIndex+span; index++ {
					preferred[index] += extra
				}
			}
			columnIndex += span
		}
	}
	return preferred
}

func (tr *TableRenderer) enforceNoWrapColumnWidths(table *TableDef, widths []float64, padding float64) {
	if table == nil || len(widths) == 0 {
		return
	}
	required, selected := tr.noWrapRequirements(table, len(widths), padding)
	nowrap, wrapping := partitionColumnIndices(selected)
	requiredTotal, total := sumIndexedWidths(required, nowrap), sumWidths(widths)
	if requiredTotal <= total {
		allocateNoWrapWithinWidth(widths, required, nowrap, wrapping, total-requiredTotal)
		return
	}
	if len(nowrap) > 0 {
		allocateOverflowingNoWrap(widths, required, nowrap, wrapping, total, requiredTotal)
	}
}

func (tr *TableRenderer) noWrapRequirements(table *TableDef, count int, padding float64) ([]float64, []bool) {
	required, selected := make([]float64, count), make([]bool, count)
	for _, row := range table.Rows {
		columnIndex := 0
		for _, cell := range row.Cells {
			if columnIndex >= count {
				break
			}
			span := normalizedColspan(cell.Colspan)
			if span == 1 {
				required[columnIndex] = math.Max(required[columnIndex], tr.measureNoWrapCellRequiredWidth(&cell, padding))
				selected[columnIndex] = selected[columnIndex] || cell.NoWrap
			}
			columnIndex += span
		}
	}
	return required, selected
}

func partitionColumnIndices(selected []bool) ([]int, []int) {
	chosen, others := []int{}, []int{}
	for index, value := range selected {
		if value {
			chosen = append(chosen, index)
		} else {
			others = append(others, index)
		}
	}
	return chosen, others
}

func sumWidths(widths []float64) float64 {
	total := 0.0
	for _, width := range widths {
		total += width
	}
	return total
}
func sumIndexedWidths(widths []float64, indices []int) float64 {
	total := 0.0
	for _, index := range indices {
		total += widths[index]
	}
	return total
}

func allocateNoWrapWithinWidth(widths, required []float64, nowrap, wrapping []int, remaining float64) {
	for _, index := range nowrap {
		widths[index] = required[index]
	}
	if len(wrapping) == 0 {
		return
	}
	original := sumIndexedWidths(widths, wrapping)
	if original > 0 {
		scale := remaining / original
		for _, index := range wrapping {
			widths[index] *= scale
		}
		return
	}
	share := remaining / float64(len(wrapping))
	for _, index := range wrapping {
		widths[index] = share
	}
}

func allocateOverflowingNoWrap(widths, required []float64, nowrap, wrapping []int, total, requiredTotal float64) {
	scale := total / requiredTotal
	for _, index := range nowrap {
		widths[index] = required[index] * scale
	}
	for _, index := range wrapping {
		widths[index] = 0
	}
}

func (tr *TableRenderer) measureNoWrapCellRequiredWidth(cell *CellDef, padding float64) float64 {
	if cell == nil {
		return 0
	}
	_, right, _, left := tr.resolvedCellPadding(cell, padding)
	tr.applyCellStyle(cell)
	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	maximum := tr.maxLineWidthNoWrap(text)
	if cell.SubText != "" {
		face := firstNonEmpty(cell.SubFontFace, cell.FontFace, defaultTableFontFace)
		size := cell.SubFontSize
		if size <= 0 {
			size = 7
		}
		tr.pdf.SetFont(normalizeTableFontFace(face), "", size)
		maximum = math.Max(maximum, tr.maxLineWidthNoWrap(cell.SubText))
	}
	return maximum + left + right
}

func (tr *TableRenderer) maxLineWidthNoWrap(text string) float64 {
	maximum := 0.0
	for _, line := range strings.Split(tr.normalizeTableTextForCurrentFont(text), "\n") {
		maximum = math.Max(maximum, tr.pdf.GetStringWidth(line))
	}
	return maximum
}

func (tr *TableRenderer) calculateRowHeight(row *RowDef, minimum, padding float64, widths []float64) float64 {
	height := minimum
	for _, placement := range tableCellPlacements(row, widths) {
		height = math.Max(height, tr.measureCellHeight(&row.Cells[placement.cellIndex], placement.width, padding))
	}
	return height
}

func (tr *TableRenderer) resolvedCellPadding(cell *CellDef, fallback float64) (top, right, bottom, left float64) {
	top, right, bottom, left = fallback, fallback, fallback, fallback
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
	top, right, bottom, left := tr.resolvedCellPadding(cell, padding)
	contentWidth := math.Max(width-left-right, 1)
	fontSize, lineHeightMultiplier := tr.applyCellStyle(cell)
	text := cell.Text
	if cell.Value != nil {
		text = tr.formatCellValue(cell)
	}
	lines := tr.wrapTextLines(text, contentWidth, cell.NoWrap)
	lineHeight, height := fontSize*lineHeightMultiplier, top+float64(len(lines))*fontSize*lineHeightMultiplier
	if cell.SubText != "" {
		size := cell.SubFontSize
		if size <= 0 {
			size = 7
		}
		tr.pdf.SetFont(firstNonEmpty(cell.SubFontFace, "Helvetica"), "", size)
		subLines := tr.wrapTextLines(cell.SubText, contentWidth, cell.NoWrap)
		if len(subLines) > 0 && len(lines) > 0 {
			height += lineHeight
		}
		height += float64(len(subLines)) * size * 1.2
	}
	return height + bottom
}
