package pdfrender

import (
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/gofpdf"
)

func TestTableDefFromElement_AllowsFlexibleColWithoutWidth(t *testing.T) {
	table := pdfdom.NewElemTable()
	table.SetAttribute("width", "100%")
	table.SetAttribute("padding", "4")
	table.SetAttribute("rowHeightMin", "10")

	colgroup := pdfdom.NewElemColgroup()
	col1 := pdfdom.NewElemCol()
	col1.SetAttribute("width", "26")
	col2 := pdfdom.NewElemCol()
	col3 := pdfdom.NewElemCol()
	col3.SetAttribute("width", "83")

	colgroup.Add(col1)
	colgroup.Add(col2)
	colgroup.Add(col3)
	table.Add(colgroup)

	layout := &LayoutPDF{}
	def, err := layout.TableDefFromElement(table, 500)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(def.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(def.Columns))
	}
	if def.Columns[0].Width != 26 {
		t.Fatalf("expected first column width 26, got %f", def.Columns[0].Width)
	}
	if def.Columns[1].Width != 0 {
		t.Fatalf("expected second column flexible width (0), got %f", def.Columns[1].Width)
	}
	if def.Columns[2].Width != 83 {
		t.Fatalf("expected third column width 83, got %f", def.Columns[2].Width)
	}
}

func TestTableDefFromElement_InvalidColWidthHasHighlightedDiagnostic(t *testing.T) {
	table := pdfdom.NewElemTable()
	table.SetAttribute("width", "100%")
	table.SetAttribute("padding", "4")
	table.SetAttribute("rowHeightMin", "10")

	colgroup := pdfdom.NewElemColgroup()
	col := pdfdom.NewElemCol()
	col.SetAttribute("width", "abc")
	colgroup.Add(col)
	table.Add(colgroup)

	layout := &LayoutPDF{}
	_, err := layout.TableDefFromElement(table, 500)
	if err == nil {
		t.Fatal("expected an error for invalid col width")
	}
	msg := err.Error()
	if !strings.Contains(msg, "invalid <col> width") {
		t.Fatalf("expected invalid col width message, got %q", msg)
	}
	if !strings.Contains(msg, "<col width=\"") {
		t.Fatalf("expected offending col syntax in message, got %q", msg)
	}
	if !strings.Contains(msg, "^") {
		t.Fatalf("expected caret marker in diagnostic, got %q", msg)
	}
}

func TestCellDefFromTableCell_UsesFillAsBackground(t *testing.T) {
	cellElem := pdfdom.NewElemTd()
	cellElem.SetAttribute("fill", "#EEE")

	layout := &LayoutPDF{}
	cell := layout.CellDefFromTableCell(cellElem, false)

	if cell.Background != "#EEE" {
		t.Fatalf("expected cell background to come from fill, got %q", cell.Background)
	}
}

func TestH1MeasureInBox_IncludesDefaultBlockMargins(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	engine := NewPDFTextEngine(pdf, nil)

	heading := pdfdom.NewElemH1()
	heading.Add(&pdfdom.PDFTextNode{Text: "Leistungsnachweis"})

	metrics, err := engine.MeasureInBox(heading, &pdfdom.PDFTextBox{X: 0, Y: 0, Width: 500, Fit: pdfdom.TextFitWrap})
	if err != nil {
		t.Fatalf("MeasureInBox returned error: %v", err)
	}
	if metrics.Height < 40 {
		t.Fatalf("expected h1 height to include default block margins, got %f", metrics.Height)
	}
}

func TestTableBlockMargins_DefaultAndExplicit(t *testing.T) {
	table := &TableDef{}
	top, bottom := tableBlockMargins(table)
	if top != defaultTableFontSize || bottom != defaultTableFontSize {
		t.Fatalf("expected default table margins to match default table font size, got top=%f bottom=%f", top, bottom)
	}

	tableElem := pdfdom.NewElemTable()
	tableElem.SetAttribute("width", "100%")
	tableElem.SetAttribute("padding", "4")
	tableElem.SetAttribute("rowHeightMin", "10")
	tableElem.SetAttribute("marginTop", "12")
	tableElem.SetAttribute("marginBottom", "14")

	layout := &LayoutPDF{}
	def, err := layout.TableDefFromElement(tableElem, 500)
	if err != nil {
		t.Fatalf("TableDefFromElement returned error: %v", err)
	}
	top, bottom = tableBlockMargins(def)
	if top != 12 || bottom != 14 {
		t.Fatalf("expected explicit table margins to be honored, got top=%f bottom=%f", top, bottom)
	}
}
