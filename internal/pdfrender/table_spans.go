package pdfrender

import "fmt"

type tableCellPlacement struct {
	cellIndex int
	column    int
	span      int
	offset    float64
	width     float64
}

func tableCellPlacements(row *RowDef, colWidths []float64) []tableCellPlacement {
	placements := make([]tableCellPlacement, 0, len(row.Cells))
	column := 0
	offset := 0.0
	for cellIndex := range row.Cells {
		span := normalizedColspan(row.Cells[cellIndex].Colspan)
		width := 0.0
		for idx := column; idx < column+span && idx < len(colWidths); idx++ {
			width += colWidths[idx]
		}
		placements = append(placements, tableCellPlacement{cellIndex: cellIndex, column: column, span: span, offset: offset, width: width})
		column += span
		offset += width
	}
	return placements
}

func normalizedColspan(span int) int {
	if span < 1 {
		return 1
	}
	return span
}

func validateTableSpans(table *TableDef, columnCount int) error {
	for rowIndex, row := range table.Rows {
		occupied := 0
		for cellIndex, cell := range row.Cells {
			span := normalizedColspan(cell.Colspan)
			if occupied+span > columnCount {
				return fmt.Errorf("table row %d cell %d colspan %d exceeds %d remaining columns", rowIndex, cellIndex, span, columnCount-occupied)
			}
			occupied += span
		}
	}
	return nil
}

func (tr *TableRenderer) resolvedRowHeights(table *TableDef) ([]float64, error) {
	if table == nil {
		return nil, fmt.Errorf("table cannot be nil")
	}
	if len(table.Columns) == 0 {
		return nil, fmt.Errorf("table must define at least one column")
	}
	layout := tr.resolveTableLayout(table)
	if err := validateTableSpans(table, len(layout.colWidths)); err != nil {
		return nil, err
	}
	heights := make([]float64, len(table.Rows))
	for rowIndex, row := range table.Rows {
		height := row.Height
		if height == 0 {
			height = layout.rowHeightMin
		}
		if calculated := tr.calculateRowHeight(&row, height, layout.padding, layout.colWidths); calculated > height {
			height = calculated
		}
		heights[rowIndex] = height
	}
	return heights, nil
}
