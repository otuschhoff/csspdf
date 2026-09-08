package pdfrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
	"github.com/otuschhoff/gofpdf"
)

func TestRenderDocTemplateFlowCancelsDuringLayout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	settings := templateload.PageSettings{Width: 300, Height: 400, Margins: templateload.PageMargins{Top: 20, Right: 20, Bottom: 20, Left: 20}}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	layout, err := NewLayoutPDFWithOptions(settings, settings, nil, nil, LayoutOptions{
		Context: ctx,
		ImageLoader: func(string) (ImageResource, error) {
			cancel()
			return ImageResource{Name: "cancel.png", Type: "PNG", Data: imageData.Bytes()}, nil
		},
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	layout.StartFlow()
	imageNode := pdfdom.NewElemImg()
	imageNode.SetAttribute("src", "cancel.png")
	imageNode.SetAttribute("width", "10")
	imageNode.SetAttribute("height", "10")
	err = RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{imageNode, textDiv("must not render")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected layout cancellation, got %v", err)
	}
}

func TestLayoutLazilyInitializesConfiguredTemplate(t *testing.T) {
	settings := templateload.PageSettings{Width: 300, Height: 400, Margins: templateload.PageMargins{Top: 20, Right: 20, Bottom: 20, Left: 20}}
	created := false
	layout, err := NewLayoutPDFWithOptions(settings, settings, nil, nil, LayoutOptions{TemplateFactories: map[string]TemplateFactory{
		"RingLogo": func(pdf *gofpdf.Fpdf) gofpdf.Template {
			created = true
			return CreateRingLogoTemplate(pdf, LogoBaseRadius, LogoTplCenter, LogoTplCenter)
		},
	}})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	if created {
		t.Fatal("configured template initialized eagerly")
	}
	template, err := layout.TemplateByName("RingLogo")
	if err != nil || template == nil {
		t.Fatalf("resolve ring logo lazily: template=%v err=%v", template, err)
	}
	if !created {
		t.Fatal("configured template factory was not called")
	}
	created = false
	if _, err := layout.TemplateByName("ringlogo"); err != nil || created {
		t.Fatalf("expected normalized cached template, created=%v err=%v", created, err)
	}
}

func TestGenericLayoutHasNoProfileTemplates(t *testing.T) {
	if _, err := newFlowTestLayout(t).TemplateByName("RingLogo"); err == nil {
		t.Fatal("generic layout unexpectedly provided an invoice profile template")
	}
}

func TestRenderDocTemplateFlowPersistsCursorAcrossCalls(t *testing.T) {
	layout := newFlowTestLayout(t)
	first := textDiv("FIRST")
	second := textDiv("SECOND")

	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{first}); err != nil {
		t.Fatalf("render first section: %v", err)
	}
	firstEnd := layout.flowCursorY
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{second}); err != nil {
		t.Fatalf("render second section: %v", err)
	}
	if layout.flowCursorY <= firstEnd {
		t.Fatalf("second section did not advance flow cursor: first=%f final=%f", firstEnd, layout.flowCursorY)
	}
}

func TestRenderDocTemplateFlowEmptySectionDoesNotAdvanceCursor(t *testing.T) {
	layout := newFlowTestLayout(t)
	if err := RenderDocTemplateFlow(layout, nil); err != nil {
		t.Fatalf("render empty section: %v", err)
	}
	if layout.flowCursorY != 20 || layout.flowBottomMargin != 0 {
		t.Fatalf("empty section changed flow state: cursor=%f margin=%f", layout.flowCursorY, layout.flowBottomMargin)
	}
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv("AFTER-EMPTY")}); err != nil {
		t.Fatalf("render section after empty section: %v", err)
	}
	if layout.flowCursorY <= 20 {
		t.Fatalf("content after empty section did not advance cursor: %f", layout.flowCursorY)
	}
}

func TestRenderDocTemplateOverlayDoesNotAdvancePersistentCursor(t *testing.T) {
	layout := newFlowTestLayout(t)
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv("FLOW")}); err != nil {
		t.Fatalf("render flow: %v", err)
	}
	wantCursor := layout.flowCursorY
	wantMargin := layout.flowBottomMargin
	overlay := textDiv("OVERLAY")
	overlay.SetAttribute("position", "absolute")
	overlay.SetAttribute("top", "200")
	if err := RenderDocTemplateOverlay(layout, []pdfdom.PDFElementNode{overlay}); err != nil {
		t.Fatalf("render overlay: %v", err)
	}
	if layout.flowCursorY != wantCursor || layout.flowBottomMargin != wantMargin {
		t.Fatalf("overlay changed persistent flow state: cursor=%f margin=%f", layout.flowCursorY, layout.flowBottomMargin)
	}
}

func TestRenderDocTemplateOverlayRejectsPageBreaksWithoutChangingPageState(t *testing.T) {
	layout := newFlowTestLayout(t)
	block := textDiv("OVERLAY")
	block.SetAttribute("breakBefore", "page")
	err := RenderDocTemplateOverlay(layout, []pdfdom.PDFElementNode{block})
	if err == nil || !strings.Contains(err.Error(), "overlay cannot contain break-before") {
		t.Fatalf("expected page-local break error, got %v", err)
	}
	if layout.currentPage != 1 || layout.totalPages != 1 {
		t.Fatalf("overlay changed page state: current=%d total=%d", layout.currentPage, layout.totalPages)
	}
}

func TestNextPlanWindowMovesFinalLineWhenBottomSpacingDoesNotFit(t *testing.T) {
	bands := []verticalBand{{top: 0, bottom: 10}, {top: 10, bottom: 20}}
	end, next, done, ok := nextPlanWindow(bands, 0, 22, 25)
	if !ok || done || end != 10 || next != 10 {
		t.Fatalf("first window = end %f next %f done %t ok %t", end, next, done, ok)
	}
	end, _, done, ok = nextPlanWindow(bands, next, 22, 25)
	if !ok || !done || end != 25 {
		t.Fatalf("final window = end %f done %t ok %t", end, done, ok)
	}
}

func TestRenderDocTemplateFlowRejectsOutOfBoundsAbsoluteBlock(t *testing.T) {
	layout := newFlowTestLayout(t)
	block := textDiv("OUTSIDE")
	block.SetAttribute("position", "absolute")
	block.SetAttribute("top", "395")
	err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{block})
	if err == nil || !strings.Contains(err.Error(), "absolute div bounds") {
		t.Fatalf("expected absolute bounds error, got %v", err)
	}
}

func TestRenderDocTemplateFlowRejectsOutOfBoundsAbsoluteTable(t *testing.T) {
	layout := newFlowTestLayout(t)
	table := paginationTestTable([]float64{20})
	table.SetAttribute("position", "absolute")
	table.SetAttribute("left", "50")
	table.SetAttribute("width", "260")
	err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table})
	if err == nil || !strings.Contains(err.Error(), "absolute table bounds") || !strings.Contains(err.Error(), "exceed page 300.00x400.00") {
		t.Fatalf("expected absolute table width bounds error, got %v", err)
	}
}

func TestValidateWholeBlockHeightRejectsOversizedAndNonFiniteValues(t *testing.T) {
	for _, height := range []float64{161, math.NaN(), math.Inf(1)} {
		if err := validateWholeBlockHeight("image", height, 160); err == nil {
			t.Fatalf("expected height %v to fail", height)
		}
	}
}

func newFlowTestLayout(t *testing.T) *LayoutPDF {
	t.Helper()
	settings := templateload.PageSettings{Width: 300, Height: 400, Margins: templateload.PageMargins{Top: 20, Right: 20, Bottom: 20, Left: 20}}
	layout, err := NewLayoutPDF(settings, settings, nil, nil)
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	layout.StartFlow()
	return layout
}

func textDiv(text string) *pdfdom.ElemDiv {
	div := pdfdom.NewElemDiv()
	div.Add(&pdfdom.PDFTextNode{Text: text})
	return div
}

func TestRenderDocTemplateFlowPaginatesHundredTableRows(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	table := paginationTestTable(make([]float64, 100))
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table}); err != nil {
		t.Fatalf("render table: %v", err)
	}
	if layout.totalPages < 5 {
		t.Fatalf("page count = %d, want at least 5", layout.totalPages)
	}
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	for row := 0; row < 100; row++ {
		marker := fmt.Appendf(nil, "ROW-%03d", row)
		if count := bytes.Count(output.Bytes(), marker); count != 1 {
			t.Fatalf("marker %q occurs %d times, want once", marker, count)
		}
	}
	if count := bytes.Count(output.Bytes(), []byte("HEADER")); count != layout.totalPages {
		t.Fatalf("header occurs %d times, want once on each of %d pages", count, layout.totalPages)
	}
}

func TestRenderDocTemplateFlowPaginatesMixedHeightRows(t *testing.T) {
	layout := newFlowTestLayout(t)
	heights := make([]float64, 12)
	for row := 0; row < 12; row++ {
		heights[row] = 18
		if row%3 == 0 {
			heights[row] = 55
		}
	}
	table := paginationTestTable(heights)
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table}); err != nil {
		t.Fatalf("render mixed rows: %v", err)
	}
	if layout.totalPages < 2 {
		t.Fatalf("mixed-height table did not paginate: pages=%d", layout.totalPages)
	}
}

func TestRenderDocTemplateFlowRejectsOversizedTableRow(t *testing.T) {
	layout := newFlowTestLayout(t)
	table := paginationTestTable([]float64{500})
	err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table})
	if err == nil || !strings.Contains(err.Error(), "table row 1") || !strings.Contains(err.Error(), "exceeds available content height") {
		t.Fatalf("expected contextual oversized-row error, got %v", err)
	}
}

func TestRenderDocTemplateFlowRendersHeaderOnlyTableOnce(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	table := paginationTestTable(nil)
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table}); err != nil {
		t.Fatalf("render header-only table: %v", err)
	}
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	if count := bytes.Count(output.Bytes(), []byte("HEADER")); count != 1 {
		t.Fatalf("header occurs %d times, want once", count)
	}
}

func TestRenderDocTemplateFlowPaginatesTableWithoutHeader(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	table := paginationTestTableWithHeader(make([]float64, 40), false)
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table}); err != nil {
		t.Fatalf("render table without header: %v", err)
	}
	if layout.totalPages < 2 {
		t.Fatalf("body-only table page count = %d, want at least 2", layout.totalPages)
	}
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte("HEADER")) {
		t.Fatal("body-only table unexpectedly rendered a header")
	}
	for row := 0; row < 40; row++ {
		marker := fmt.Appendf(nil, "ROW-%03d", row)
		if count := bytes.Count(output.Bytes(), marker); count != 1 {
			t.Fatalf("marker %q occurs %d times, want once", marker, count)
		}
	}
}

func TestRenderDocTemplateFlowContinuesLongTextWithoutLoss(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	var text strings.Builder
	for line := 0; line < 80; line++ {
		if line > 0 {
			text.WriteByte('\n')
		}
		fmt.Fprintf(&text, "LINE-%03d", line)
	}
	block := textDiv(text.String())
	block.SetAttribute("marginTop", "12")
	block.SetAttribute("marginBottom", "14")
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{block}); err != nil {
		t.Fatalf("render long text: %v", err)
	}
	if layout.totalPages < 3 {
		t.Fatalf("long text page count = %d, want at least 3", layout.totalPages)
	}
	if layout.flowCursorY > layout.CurrentFlowBottom() {
		t.Fatalf("final flow cursor %.2f exceeds content bottom %.2f", layout.flowCursorY, layout.CurrentFlowBottom())
	}
	if layout.flowBottomMargin != 14 {
		t.Fatalf("final bottom margin = %.2f, want 14", layout.flowBottomMargin)
	}
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	for line := 0; line < 80; line++ {
		marker := fmt.Appendf(nil, "LINE-%03d", line)
		if count := bytes.Count(output.Bytes(), marker); count != 1 {
			t.Fatalf("marker %q occurs %d times, want once", marker, count)
		}
	}
}

func TestRenderDocTemplateFlowPaintsBorderOnTextContinuationFragments(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	block := textDiv(strings.Repeat("bordered continuation content\n", 80))
	block.SetAttribute("border", "1 solid #123456")
	block.SetAttribute("width", "260")
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{block}); err != nil {
		t.Fatalf("render bordered continuation: %v", err)
	}
	if layout.totalPages < 2 {
		t.Fatalf("bordered text page count = %d, want at least 2", layout.totalPages)
	}
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	if count := bytes.Count(output.Bytes(), []byte(" re S")); count < layout.totalPages {
		t.Fatalf("border drawing commands = %d, want at least one per page across %d pages", count, layout.totalPages)
	}
}

func TestRenderDocTemplateFlowContinuationPersistsExactFinalCursor(t *testing.T) {
	layout := newFlowTestLayout(t)
	lines := make([]string, 31)
	for index := range lines {
		lines[index] = fmt.Sprintf("CURSOR-%02d", index)
	}
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv(strings.Join(lines, "\n"))}); err != nil {
		t.Fatalf("render cursor continuation: %v", err)
	}
	if layout.totalPages != 2 {
		t.Fatalf("page count = %d, want 2", layout.totalPages)
	}
	const wantCursor = 32.0 // 20pt page top plus one 12pt line on the final page.
	if math.Abs(layout.flowCursorY-wantCursor) > 0.001 {
		t.Fatalf("final flow cursor = %.3f, want %.3f", layout.flowCursorY, wantCursor)
	}
}

func paginationTestTable(rowHeights []float64) *pdfdom.ElemTable {
	return paginationTestTableWithHeader(rowHeights, true)
}

func paginationTestTableWithHeader(rowHeights []float64, includeHeader bool) *pdfdom.ElemTable {
	table := pdfdom.NewElemTable()
	table.SetAttribute("width", "260")
	table.SetAttribute("padding", "2")
	table.SetAttribute("rowHeightMin", "14")
	columns := pdfdom.NewElemColgroup()
	columns.Add(pdfdom.NewElemCol().SetAttribute("width", "80"))
	columns.Add(pdfdom.NewElemCol().SetAttribute("width", "180"))
	table.Add(columns)

	if includeHeader {
		header := pdfdom.NewElemThead()
		headerRow := pdfdom.NewElemTr()
		headerRow.Add(pdfdom.NewElemTh().Add(&pdfdom.PDFTextNode{Text: "HEADER"}))
		headerRow.Add(pdfdom.NewElemTh().Add(&pdfdom.PDFTextNode{Text: "VALUE"}))
		header.Add(headerRow)
		table.Add(header)
	}

	body := pdfdom.NewElemTbody()
	for row, height := range rowHeights {
		rowElem := pdfdom.NewElemTr()
		if height > 0 {
			rowElem.SetAttribute("height", fmt.Sprintf("%.2f", height))
		}
		rowElem.Add(pdfdom.NewElemTd().Add(&pdfdom.PDFTextNode{Text: fmt.Sprintf("ROW-%03d", row)}))
		rowElem.Add(pdfdom.NewElemTd().Add(&pdfdom.PDFTextNode{Text: "value"}))
		body.Add(rowElem)
	}
	table.Add(body)
	return table
}

func TestInterElementSpacing_CollapseUsesMaximum(t *testing.T) {
	got := interElementSpacing(13.4, 10.2, true)
	if got != 13.4 {
		t.Fatalf("expected collapsed spacing to use max margin, got %f", got)
	}
}

func TestInterElementSpacing_AdditiveUsesSum(t *testing.T) {
	got := interElementSpacing(13.4, 10.2, false)
	if got != 23.6 {
		t.Fatalf("expected additive spacing to sum margins, got %f", got)
	}
}

func TestIsVerticalMarginCollapsible(t *testing.T) {
	if !isVerticalMarginCollapsible(pdfdom.NewElemDiv()) {
		t.Fatal("expected div margins to be collapsible")
	}
	if !isVerticalMarginCollapsible(pdfdom.NewElemH1()) {
		t.Fatal("expected h1 margins to be collapsible")
	}
	if !isVerticalMarginCollapsible(pdfdom.NewElemTable()) {
		t.Fatal("expected table margins to be collapsible")
	}
	if isVerticalMarginCollapsible(pdfdom.NewElemImg()) {
		t.Fatal("expected img margins to be non-collapsible")
	}
}

func TestCollapsedSpacingForInternalMarginsUsesContentTop(t *testing.T) {
	currentY := 100.0
	previousBottom := 13.4
	currentTop := 10.0

	contentTop := currentY + interElementSpacing(previousBottom, currentTop, true)
	boxY := contentTop - currentTop

	if contentTop != 113.4 {
		t.Fatalf("unexpected content top: %f", contentTop)
	}
	if boxY != 103.4 {
		t.Fatalf("unexpected box Y for internally-margined element: %f", boxY)
	}
}
