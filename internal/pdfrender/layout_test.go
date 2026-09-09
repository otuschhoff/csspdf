package pdfrender

import (
	"math"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
	"github.com/otuschhoff/gofpdf"
)

func mustAddNode(t testing.TB, parent pdfdom.PDFElementNode, child pdfdom.PDFNode) pdfdom.PDFElementNode {
	t.Helper()
	if err := parent.Add(child); err != nil {
		t.Fatalf("add %T to %T: %v", child, parent, err)
	}
	return parent
}

func TestNewLayoutPDFRejectsInvalidContentGeometry(t *testing.T) {
	valid := templateload.PageSettings{Width: 200, Height: 300, Margins: templateload.PageMargins{Top: 10, Right: 10, Bottom: 10, Left: 10}}
	testCases := []struct {
		name     string
		settings templateload.PageSettings
		contains string
	}{
		{name: "non-finite width", settings: templateload.PageSettings{Width: math.Inf(1), Height: 300}, contains: "finite"},
		{name: "zero height", settings: templateload.PageSettings{Width: 200}, contains: "greater than zero"},
		{name: "negative margin", settings: templateload.PageSettings{Width: 200, Height: 300, Margins: templateload.PageMargins{Left: -1}}, contains: "zero or greater"},
		{name: "impossible horizontal margins", settings: templateload.PageSettings{Width: 200, Height: 300, Margins: templateload.PageMargins{Left: 100, Right: 100}}, contains: "non-positive content box"},
		{name: "impossible vertical margins", settings: templateload.PageSettings{Width: 200, Height: 300, Margins: templateload.PageMargins{Top: 200, Bottom: 100}}, contains: "non-positive content box"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewLayoutPDF(testCase.settings, valid, nil, nil)
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("expected error containing %q, got %v", testCase.contains, err)
			}
		})
	}
}

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

	mustAddNode(t, colgroup, col1)
	mustAddNode(t, colgroup, col2)
	mustAddNode(t, colgroup, col3)
	mustAddNode(t, table, colgroup)

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

func TestTableDefFromElementInfersOccupiedColumnsFromColspan(t *testing.T) {
	table := pdfdom.NewElemTable()
	table.SetAttribute("width", "300")
	table.SetAttribute("padding", "4")
	table.SetAttribute("rowHeightMin", "20")
	row := pdfdom.NewElemTr()
	mustAddNode(t, row, pdfdom.NewElemTd().SetAttribute("colspan", "2").SetAttribute("width", "200"))
	mustAddNode(t, row, pdfdom.NewElemTd().SetAttribute("width", "100"))
	mustAddNode(t, table, row)

	layout := &LayoutPDF{}
	definition, err := layout.TableDefFromElement(table, 300)
	if err != nil {
		t.Fatalf("TableDefFromElement returned error: %v", err)
	}
	if len(definition.Columns) != 3 {
		t.Fatalf("inferred columns = %d, want 3 occupied columns", len(definition.Columns))
	}
	if definition.Columns[0].Width != 100 || definition.Columns[1].Width != 100 || definition.Columns[2].Width != 100 {
		t.Fatalf("unexpected inferred widths: %+v", definition.Columns)
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
	mustAddNode(t, colgroup, col)
	mustAddNode(t, table, colgroup)

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

func TestNormalizeTableFontStyle(t *testing.T) {
	tests := map[string]string{
		"":                         "",
		" NORMAL ":                 "",
		"ui":                       "IU",
		"bold, italic|underline":   "BIU",
		"900;oblique":              "BI",
		"underline bold underline": "BU",
	}
	for input, expected := range tests {
		if actual := normalizeTableFontStyle(input); actual != expected {
			t.Errorf("normalizeTableFontStyle(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestTemplateElementLength(t *testing.T) {
	element := pdfdom.NewElemUseTemplate()
	if value, explicit := templateElementLength(element, "x", 42); value != 42 || explicit {
		t.Fatalf("missing attribute = %v, %v; want fallback and implicit", value, explicit)
	}

	element.SetAttribute("x", "invalid")
	if value, explicit := templateElementLength(element, "x", 42); value != 42 || explicit {
		t.Fatalf("invalid attribute = %v, %v; want fallback and implicit", value, explicit)
	}

	element.SetAttribute("x", " 17.5 ")
	if value, explicit := templateElementLength(element, "x", 42); value != 17.5 || !explicit {
		t.Fatalf("explicit attribute = %v, %v; want 17.5 and explicit", value, explicit)
	}
}

func TestH1MeasureInBox_IncludesDefaultBlockMargins(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	engine := NewPDFTextEngine(pdf, nil)

	heading := pdfdom.NewElemH1()
	mustAddNode(t, heading, &pdfdom.PDFTextNode{Text: "Leistungsnachweis"})

	metrics, err := engine.MeasureInBox(heading, &pdfdom.PDFTextBox{X: 0, Y: 0, Width: 500, Fit: pdfdom.TextFitWrap})
	if err != nil {
		t.Fatalf("MeasureInBox returned error: %v", err)
	}
	if metrics.Height < 40 {
		t.Fatalf("expected h1 height to include default block margins, got %f", metrics.Height)
	}
}

func TestDivMeasureInBox_IncludesInlineCurrencyValue(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	i18nInst, err := i18n.New("de")
	if err != nil {
		t.Fatalf("failed to create i18n instance: %v", err)
	}
	engine := NewPDFTextEngine(pdf, i18nInst)
	engine.SetValueFormatter(format.New(i18nInst, "EUR"))

	div := pdfdom.NewElemDiv()
	mustAddNode(t, div, &pdfdom.PDFTextNode{Text: "Total: "})
	mustAddNode(t, div, pdfdom.NewElemCurrencyValue(22500))

	metrics, err := engine.MeasureInBox(div, &pdfdom.PDFTextBox{X: 0, Y: 0, Width: 500, Fit: pdfdom.TextFitWrap})
	if err != nil {
		t.Fatalf("MeasureInBox returned error: %v", err)
	}
	if metrics.Width <= 0 {
		t.Fatalf("expected inline currency value to contribute width, got %f", metrics.Width)
	}
	if metrics.LineCount == 0 {
		t.Fatalf("expected inline currency value to contribute line content")
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
