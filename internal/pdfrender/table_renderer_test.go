package pdfrender

import (
	"math"
	"strings"
	"testing"

	"github.com/otuschhoff/gofpdf"
)

func TestTableHexToRGB(t *testing.T) {
	tests := []struct {
		color            string
		red, green, blue int
	}{
		{color: "#123", red: 17, green: 34, blue: 51},
		{color: "#12AbEF", red: 18, green: 171, blue: 239},
		{color: " lightgrey ", red: 211, green: 211, blue: 211},
		{color: "invalid", red: 0, green: 0, blue: 0},
	}
	for _, test := range tests {
		red, green, blue := tableHexToRGB(test.color)
		if red != test.red || green != test.green || blue != test.blue {
			t.Fatalf("tableHexToRGB(%q) = (%d,%d,%d), want (%d,%d,%d)", test.color, red, green, blue, test.red, test.green, test.blue)
		}
	}
}

func TestTableEncodePDFTextLatin1(t *testing.T) {
	if got, want := tableEncodePDFTextLatin1("Grüße € — 漢"), "Gr\xfc\xdfe \x80 \x97 ?"; got != want {
		t.Fatalf("encoded text = %q, want %q", got, want)
	}
}

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

func TestTableCellPlacementsAdvanceAcrossColspan(t *testing.T) {
	row := RowDef{Cells: []CellDef{{Text: "wide", Colspan: 2}, {Text: "last"}}}
	placements := tableCellPlacements(&row, []float64{80, 120, 60})
	if len(placements) != 2 {
		t.Fatalf("placements = %d, want 2", len(placements))
	}
	if placements[0].column != 0 || placements[0].width != 200 || placements[0].offset != 0 {
		t.Fatalf("unexpected spanning placement: %+v", placements[0])
	}
	if placements[1].column != 2 || placements[1].width != 60 || placements[1].offset != 200 {
		t.Fatalf("unexpected following placement: %+v", placements[1])
	}
}

func TestCalculateRowHeightUsesSpannedWidth(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	renderer := NewTableRenderer(pdf, nil)
	row := RowDef{Cells: []CellDef{{Text: "one two three four five six seven eight", Colspan: 2}, {Text: "last"}}}
	widths := []float64{50, 150, 60}
	want := renderer.measureCellHeight(&row.Cells[0], 200, 4)
	if got := renderer.calculateRowHeight(&row, 0, 4, widths); got != want {
		t.Fatalf("row height = %f, want spanning-cell height %f", got, want)
	}
}

func TestResolveTableLayoutAutoKeepsSpanPlacementsConsistent(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	renderer := NewTableRenderer(pdf, nil)
	table := &TableDef{Width: 300, TableLayout: "auto", Columns: []ColumnDef{{}, {}, {}}, Rows: []RowDef{{Cells: []CellDef{{Text: "wide spanning heading", Colspan: 2}, {Text: "last"}}}}}
	layout := renderer.resolveTableLayout(table)
	placements := tableCellPlacements(&table.Rows[0], layout.colWidths)
	if len(placements) != 2 || math.Abs(placements[1].offset-placements[0].width) > 0.001 {
		t.Fatalf("inferred placements are inconsistent: %+v", placements)
	}
}

func TestMeasureTableHeightRejectsColspanBeyondColumns(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	renderer := NewTableRenderer(pdf, nil)
	_, err := renderer.MeasureTableHeight(&TableDef{Columns: []ColumnDef{{}, {}}, Rows: []RowDef{{Cells: []CellDef{{Colspan: 3}}}}})
	if err == nil || !strings.Contains(err.Error(), "colspan 3") {
		t.Fatalf("expected contextual colspan error, got %v", err)
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
