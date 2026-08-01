package pdfrender

import (
	"strings"
	"testing"

	"github.com/otuschhoff/go-dom2pdf/internal/pdfdom"
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
