package pdfrender

import (
	"fmt"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

const tableCursorGap = 5.0

func (state *docFlowState) renderPaginatedTable(node *pdfdom.ElemTable, initial *TableDef, initialX, initialY float64) error {
	headerCount := leadingHeaderRowCount(initial.Rows)
	dataIndex := headerCount
	if dataIndex == len(initial.Rows) {
		dataIndex = 0
		headerCount = 0
	}
	firstFragment := true
	tableDef := initial
	xPos, yPos := initialX, initialY

	for dataIndex < len(tableDef.Rows) {
		rowHeights, err := state.layout.tableRenderer.resolvedRowHeights(tableDef)
		if err != nil {
			return fmt.Errorf("failed to measure table rows: %w", err)
		}
		available := state.layout.CurrentFlowBottom() - yPos - tableDef.MarginBottom - tableCursorGap
		fragmentRows, end, baseHeight := selectTableFragment(tableDef, rowHeights, headerCount, dataIndex, available, firstFragment)
		if end == dataIndex {
			if firstFragment && state.currentY > state.y {
				var err error
				tableDef, xPos, yPos, err = state.nextTablePage(node)
				if err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("table row %d height %.2f exceeds available content height %.2f with repeated headers", dataIndex, rowHeights[dataIndex], available-baseHeight)
		}

		if err := state.renderTableFragment(tableDef, fragmentRows, xPos, yPos, firstFragment); err != nil {
			return fmt.Errorf("failed to render table fragment beginning at row %d: %w", dataIndex, err)
		}
		dataIndex = end
		firstFragment = false
		if dataIndex < len(tableDef.Rows) {
			var err error
			tableDef, xPos, yPos, err = state.nextTablePage(node)
			if err != nil {
				return err
			}
		}
	}

	state.currentY = state.layout.PDF.GetY()
	state.pendingBottomMargin = tableDef.MarginBottom
	return nil
}

func (state *docFlowState) renderTableFragment(table *TableDef, rows []RowDef, x, y float64, first bool) error {
	fragment := *table
	fragment.Rows = rows
	if !first {
		fragment.Title = ""
	}
	state.layout.PDF.SetXY(x, y)
	return state.layout.tableRenderer.RenderTable(&fragment)
}

func (state *docFlowState) nextTablePage(node *pdfdom.ElemTable) (*TableDef, float64, float64, error) {
	if err := state.nextPage(); err != nil {
		return nil, 0, 0, err
	}
	return state.rebuildTableForCurrentPage(node)
}

func selectTableFragment(table *TableDef, rowHeights []float64, headerCount, dataIndex int, available float64, firstFragment bool) ([]RowDef, int, float64) {
	baseHeight := 0.0
	if firstFragment && table.Title != "" {
		baseHeight = defaultTableFontSize + 10
	}
	rows := append([]RowDef(nil), table.Rows[:headerCount]...)
	for index := 0; index < headerCount; index++ {
		baseHeight += rowHeights[index]
	}
	end := dataIndex
	used := baseHeight
	for end < len(table.Rows) && used+rowHeights[end] <= available {
		rows = append(rows, table.Rows[end])
		used += rowHeights[end]
		end++
	}
	return rows, end, baseHeight
}

func (state *docFlowState) rebuildTableForCurrentPage(node *pdfdom.ElemTable) (*TableDef, float64, float64, error) {
	xPos, _, width, _ := state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	tableDef, err := state.layout.TableDefFromElement(node, width)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to rebuild table definition after page break: %w", err)
	}
	topMargin, _ := tableBlockMargins(tableDef)
	yPos := state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, isVerticalMarginCollapsible(node))
	return tableDef, xPos, yPos, nil
}

func leadingHeaderRowCount(rows []RowDef) int {
	count := 0
	for count < len(rows) && rows[count].Header {
		count++
	}
	return count
}
