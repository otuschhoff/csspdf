package pdfrender

import (
	"testing"

	"github.com/otuschhoff/gofpdf"
)

func TestResolveTableLayout_AutoPrefersWiderDescriptionColumn(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	r := NewTableRenderer(pdf, nil)

	table := &TableDef{
		Width:       500,
		TableLayout: "auto",
		Padding:     4,
		Columns:     []ColumnDef{{}, {}, {}, {}, {}},
		Rows: []RowDef{
			{
				Cells: []CellDef{
					{Text: "Date", NoWrap: true},
					{Text: "WW", NoWrap: true},
					{Text: "Hours", NoWrap: true},
					{Text: "MD", NoWrap: true},
					{Text: "Description of Services"},
				},
			},
			{
				Cells: []CellDef{
					{Text: "2026-06-30", NoWrap: true},
					{Text: "27.2", NoWrap: true},
					{Text: "12:30", NoWrap: true},
					{Text: "1.56", NoWrap: true},
					{Text: "Long consulting entry that should receive most of the residual width to reduce line wraps."},
				},
			},
		},
	}

	layout := r.resolveTableLayout(table)
	if len(layout.colWidths) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(layout.colWidths))
	}
	if layout.colWidths[4] <= layout.colWidths[0] {
		t.Fatalf("expected description column wider than date column: desc=%f date=%f", layout.colWidths[4], layout.colWidths[0])
	}
	if layout.colWidths[4] <= layout.colWidths[3] {
		t.Fatalf("expected description column wider than narrow metric columns: desc=%f days=%f", layout.colWidths[4], layout.colWidths[3])
	}
}

func TestResolveTableLayout_AutoHonorsNoWrapMinimums(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	r := NewTableRenderer(pdf, nil)

	table := &TableDef{
		Width:       220,
		TableLayout: "auto",
		Padding:     4,
		Columns:     []ColumnDef{{}, {}, {}},
		Rows: []RowDef{
			{
				Cells: []CellDef{
					{Text: "2026-06-30", NoWrap: true},
					{Text: "12:30", NoWrap: true},
					{Text: "desc"},
				},
			},
		},
	}

	requiredDate := r.measureNoWrapCellRequiredWidth(&table.Rows[0].Cells[0], table.Padding)
	requiredHours := r.measureNoWrapCellRequiredWidth(&table.Rows[0].Cells[1], table.Padding)

	layout := r.resolveTableLayout(table)
	if layout.colWidths[0] < requiredDate {
		t.Fatalf("date nowrap width not honored: got=%f required=%f", layout.colWidths[0], requiredDate)
	}
	if layout.colWidths[1] < requiredHours {
		t.Fatalf("hours nowrap width not honored: got=%f required=%f", layout.colWidths[1], requiredHours)
	}
}

func TestResolveTableLayout_DoesNotExceedExplicitWidth(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	r := NewTableRenderer(pdf, nil)

	table := &TableDef{
		Width:       220,
		TableLayout: "auto",
		Padding:     4,
		Columns:     []ColumnDef{{}, {}, {}},
		Rows: []RowDef{
			{
				Cells: []CellDef{
					{Text: "2026-06-30", NoWrap: true},
					{Text: "12:30", NoWrap: true},
					{Text: "Long description that should wrap and not force table overflow into right margin."},
				},
			},
		},
	}

	layout := r.resolveTableLayout(table)
	total := 0.0
	for _, w := range layout.colWidths {
		total += w
	}

	if total > table.Width+0.001 {
		t.Fatalf("resolved columns exceed explicit width: total=%f width=%f", total, table.Width)
	}
	if layout.tableWidth > table.Width+0.001 {
		t.Fatalf("resolved table width exceeds explicit width: got=%f width=%f", layout.tableWidth, table.Width)
	}
}
