package pdfrender

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestSemanticPDFPreservesSectionPlacement(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv(t, "FIRST")}); err != nil {
		t.Fatalf("render first section: %v", err)
	}
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv(t, "SECOND")}); err != nil {
		t.Fatalf("render second section: %v", err)
	}
	pdfBytes := outputLayoutPDF(t, layout)
	pages := validateAndExtractPDF(t, pdfBytes, 1)
	firstY := textY(t, pages[0], "FIRST")
	secondY := textY(t, pages[0], "SECOND")
	if firstY == secondY {
		t.Fatalf("sections overlap at PDF y coordinate %.2f", firstY)
	}
}

func TestSemanticPDFHonorsExplicitBreakAndSubsequentPageMargin(t *testing.T) {
	defaultPage := templateload.PageSettings{Width: 300, Height: 400, Margins: templateload.PageMargins{Top: 60, Right: 20, Bottom: 20, Left: 20}}
	firstPage := defaultPage
	firstPage.Margins.Top = 20
	layout, err := NewLayoutPDF(defaultPage, firstPage, nil, nil)
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	layout.StartFlow()
	layout.PDF.SetCompression(false)
	first := textDiv(t, "FIRST-PAGE")
	first.SetAttribute("breakAfter", "page")
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{first, textDiv(t, "SECOND-PAGE")}); err != nil {
		t.Fatalf("render explicit break: %v", err)
	}
	pages := validateAndExtractPDF(t, outputLayoutPDF(t, layout), 2)
	firstY := textY(t, pages[0], "FIRST-PAGE")
	secondY := textY(t, pages[1], "SECOND-PAGE")
	if math.Abs((firstY-secondY)-40) > 0.01 {
		t.Fatalf("page text y difference = %.2f, want 40pt margin difference", firstY-secondY)
	}
}

func TestSemanticPDFRunningFooterPersistsAcrossPages(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	footer := textDiv(t, "RUNNING-FOOTER")
	footer.SetAttribute("position", "running(site-footer)")
	footer.SetAttribute("height", "20")
	var text strings.Builder
	for line := 0; line < 80; line++ {
		fmt.Fprintf(&text, "FOOTER-LINE-%03d\n", line)
	}
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{footer, textDiv(t, text.String())}); err != nil {
		t.Fatalf("render running footer document: %v", err)
	}
	if layout.runningFooterTemplate == nil || layout.totalPages < 2 {
		t.Fatalf("running footer was not retained across pagination: pages=%d", layout.totalPages)
	}
	pages := validateAndExtractPDF(t, outputLayoutPDF(t, layout), layout.totalPages)
	for page, content := range pages {
		if !strings.Contains(content, " Do") {
			t.Fatalf("page %d does not stamp the running-footer template", page+1)
		}
	}
}

func TestSemanticPDFValidatesPaginatedTableAndText(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	table := paginationTestTable(t, make([]float64, 100))
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{table}); err != nil {
		t.Fatalf("render table: %v", err)
	}
	pdfBytes := outputLayoutPDF(t, layout)
	pages := validateAndExtractPDF(t, pdfBytes, layout.totalPages)
	allContent := strings.Join(pages, "\n")
	for row := 0; row < 100; row++ {
		marker := fmt.Sprintf("ROW-%03d", row)
		if count := strings.Count(allContent, marker); count != 1 {
			t.Fatalf("marker %q occurs %d times in parsed content, want once", marker, count)
		}
	}
	for page, content := range pages {
		if !strings.Contains(content, "HEADER") {
			t.Fatalf("page %d does not contain repeated table header", page+1)
		}
	}
}

func TestSemanticPDFValidatesLongTextContinuation(t *testing.T) {
	layout := newFlowTestLayout(t)
	layout.PDF.SetCompression(false)
	var text strings.Builder
	for line := 0; line < 80; line++ {
		if line > 0 {
			text.WriteByte('\n')
		}
		fmt.Fprintf(&text, "SEMANTIC-%03d", line)
	}
	if err := RenderDocTemplateFlow(layout, []pdfdom.PDFElementNode{textDiv(t, text.String())}); err != nil {
		t.Fatalf("render long text: %v", err)
	}
	pdfBytes := outputLayoutPDF(t, layout)
	pages := validateAndExtractPDF(t, pdfBytes, layout.totalPages)
	allContent := strings.Join(pages, "\n")
	for line := 0; line < 80; line++ {
		marker := fmt.Sprintf("SEMANTIC-%03d", line)
		if count := strings.Count(allContent, marker); count != 1 {
			t.Fatalf("marker %q occurs %d times in parsed content, want once", marker, count)
		}
	}
}

func outputLayoutPDF(t *testing.T, layout *LayoutPDF) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := layout.PDF.Output(&output); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	return output.Bytes()
}

func validateAndExtractPDF(t *testing.T, pdfBytes []byte, wantPages int) []string {
	t.Helper()
	configuration := model.NewDefaultConfiguration()
	if err := api.Validate(bytes.NewReader(pdfBytes), configuration); err != nil {
		t.Fatalf("pdfcpu validation failed: %v", err)
	}
	pageCount, err := api.PageCount(bytes.NewReader(pdfBytes), configuration)
	if err != nil {
		t.Fatalf("pdfcpu page count failed: %v", err)
	}
	if pageCount != wantPages {
		t.Fatalf("parsed page count = %d, want %d", pageCount, wantPages)
	}
	pages := make([]string, pageCount)
	err = api.ExtractContent(bytes.NewReader(pdfBytes), nil, func(reader io.Reader, page int) error {
		content, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		if page < 1 || page > len(pages) {
			return fmt.Errorf("unexpected page number %d", page)
		}
		pages[page-1] = string(content)
		return nil
	}, configuration)
	if err != nil {
		t.Fatalf("pdfcpu content extraction failed: %v", err)
	}
	return pages
}

func textY(t *testing.T, content, marker string) float64 {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)[-0-9.]+\s+([-0-9.]+)\s+Td\s+\(` + regexp.QuoteMeta(marker) + `\)\s+Tj`)
	match := pattern.FindStringSubmatch(content)
	if len(match) != 2 {
		t.Fatalf("could not find text position for %q in content stream", marker)
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatalf("parse y coordinate for %q: %v", marker, err)
	}
	return value
}
