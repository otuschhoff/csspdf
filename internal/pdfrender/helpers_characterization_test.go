package pdfrender

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
	"github.com/otuschhoff/gofpdf"
)

func TestTextHelperGeometryAndDefaults(t *testing.T) {
	if got := alignedX(TextAlignLeft, 10, 100, 20); got != 10 {
		t.Fatalf("left aligned x = %v", got)
	}
	if got := alignedX(TextAlignCenter, 10, 100, 20); got != 50 {
		t.Fatalf("center aligned x = %v", got)
	}
	if got := alignedX(TextAlignRight, 10, 100, 20); got != 90 {
		t.Fatalf("right aligned x = %v", got)
	}
	if got := alignedX(TextAlignRight, 10, 0, 20); got != 10 {
		t.Fatalf("unconstrained aligned x = %v", got)
	}
	style := ensureTextStyleDefaults(PDFTextStyle{})
	if style.FontFace != "Helvetica" || style.FontSize != 10 || style.FontColor != "#000" || style.Align != TextAlignLeft || style.LineHeight != 1.2 {
		t.Fatalf("unexpected style defaults: %#v", style)
	}
	box := resolveBox(nil)
	if box.Fit != TextFitWrap {
		t.Fatalf("default fit = %q", box.Fit)
	}
	explicit := resolveBox(&PDFTextBox{X: 3, Fit: pdfdom.TextFitClip})
	if explicit.X != 3 || explicit.Fit != pdfdom.TextFitClip {
		t.Fatalf("explicit box = %#v", explicit)
	}
	if pdfMax(2, 1) != 2 || pdfMax(1, 2) != 2 || pdfMaxInt(2, 1) != 2 || pdfMaxInt(1, 2) != 2 {
		t.Fatal("maximum helpers returned incorrect values")
	}
}

func TestTextEncodingAndAttributeHelpers(t *testing.T) {
	node := pdfdom.NewElemDiv()
	node.SetAttribute("width", " 12pt ").SetAttribute("fallback", "8px")
	if got := htmlLengthToFloat(node, "missing", "width"); got != 12 {
		t.Fatalf("htmlLengthToFloat = %v", got)
	}
	node.SetAttribute("width", "invalid")
	if got := htmlLengthToFloat(node, "width"); got != 0 {
		t.Fatalf("invalid html length = %v", got)
	}
	if got := encodePDFTextLatin1("ASCII € — 漢\n"); got != "ASCII \x80 \x97 ?\n" {
		t.Fatalf("encoded text = %q", got)
	}
	for input, want := range map[string]TextAlign{"left": TextAlignLeft, " C ": TextAlignCenter, "r": TextAlignRight, "justify": "justify"} {
		if got := htmlNormaliseTextAlign(input); got != want {
			t.Errorf("htmlNormaliseTextAlign(%q) = %q, want %q", input, got, want)
		}
	}
	width, style, color := htmlParseBorderShorthand("2pt DOTTED #ABC")
	if width != 2 || style != "dotted" || color != "#abc" {
		t.Fatalf("border shorthand = %v %q %q", width, style, color)
	}
	for filename, want := range map[string]string{"x.jpeg": "JPG", "x.JPG": "JPG", "x.gif": "GIF", "x.bin": "PNG"} {
		if got := imageTypeFromPath(filename); got != want {
			t.Errorf("imageTypeFromPath(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestResolveImageElementAndSearchPaths(t *testing.T) {
	if _, _, _, _, _, err := resolveImageElement(nil, 10, nil, false); err == nil {
		t.Fatal("nil image node was accepted")
	}
	imageNode := pdfdom.NewElemImg()
	if _, _, _, _, _, err := resolveImageElement(imageNode, 10, nil, false); err == nil {
		t.Fatal("image without source was accepted")
	}
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.png")
	if err := os.WriteFile(filename, []byte("test"), 0o600); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}
	imageNode.SetAttribute("src", "sample.png").SetAttribute("width", "20").SetAttribute("height", "30").SetAttribute("margin-top", "4").SetAttribute("margin-bottom", "5")
	path, width, height, top, bottom, err := resolveImageElement(imageNode, 100, []string{directory}, false)
	if err != nil || path != filename || width != 20 || height != 30 || top != 4 || bottom != 5 {
		t.Fatalf("resolved image = %q %v %v %v %v, err=%v", path, width, height, top, bottom, err)
	}
	deferred, width, height, _, _, err := resolveImageElement(pdfdom.NewElemImg().SetAttribute("src", "remote.png"), 40, nil, true)
	if err != nil || deferred != "remote.png" || width != 40 || height != 40 {
		t.Fatalf("deferred image = %q %v %v, err=%v", deferred, width, height, err)
	}
	if _, _, _, _, _, err := resolveImageElement(pdfdom.NewElemImg().SetAttribute("src", "missing.png"), 0, nil, true); err == nil {
		t.Fatal("unsized image without fallback was accepted")
	}
	if _, ok := resolveImagePath("", nil); ok {
		t.Fatal("empty image path resolved")
	}
	if candidates := imagePathCandidates(filename, []string{"ignored"}); len(candidates) != 1 || candidates[0] != filename {
		t.Fatalf("absolute candidates = %#v", candidates)
	}
}

func TestTextEngineMeasurementAndClipping(t *testing.T) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)
	engine := NewPDFTextEngine(pdf, nil)
	engine.SetDefaultStyle(PDFTextStyle{FontFace: "Helvetica", FontSize: 10})
	if engine.GlyphRegistry() == nil {
		t.Fatal("GlyphRegistry returned nil")
	}
	if got := engine.resolveText(nil); got != "" {
		t.Fatalf("resolveText(nil) = %q", got)
	}
	if got := engine.resolveText(&PDFTextNode{Text: "literal"}); got != "literal" {
		t.Fatalf("literal text = %q", got)
	}
	if got := engine.resolveText(&PDFTextNode{I18nKey: "missing", Text: "fallback"}); got != "fallback" {
		t.Fatalf("fallback text = %q", got)
	}
	if got := engine.resolveText(&PDFTextNode{I18nKey: "missing"}); got != "missing" {
		t.Fatalf("missing i18n text = %q", got)
	}
	if got := engine.maximumTextWidth([]string{"a", "longer"}); got <= 0 {
		t.Fatalf("maximum width = %v", got)
	}
	if got, clipped := engine.clipToWidth("hello", 1000); got != "hello" || clipped {
		t.Fatalf("unexpected wide clip = %q, %v", got, clipped)
	}
	if got, clipped := engine.clipToWidth("hello", 1); got != "" || !clipped {
		t.Fatalf("unexpected narrow clip = %q, %v", got, clipped)
	}
}

func TestTableCellStyleAndParsingHelpers(t *testing.T) {
	cell := &CellDef{}
	style := &pdfdom.PDFTextStyle{FontFace: "Courier", FontStyle: "BI", FontColor: "#123", FontSize: 7, LineHeight: 1.4, Align: pdfdom.TextAlignRight}
	applyMainCellTextStyle(cell, style, false)
	if cell.FontFace != "Courier" || cell.FontStyle != "BI" || cell.FontColor != "#123" || cell.FontSize != 7 || cell.LineHeight != 1.4 || cell.Align != "R" || !cell.Bold || !cell.Small {
		t.Fatalf("main cell style = %#v", cell)
	}
	applySubCellTextStyle(cell, &pdfdom.PDFTextStyle{FontFace: "Times", FontColor: "#456", FontSize: 6})
	if cell.SubFontFace != "Times" || cell.SubFontColor != "#456" || cell.SubFontSize != 6 {
		t.Fatalf("sub cell style = %#v", cell)
	}
	applySubCellTextStyle(cell, nil)
	for input, want := range map[pdfdom.TextAlign]string{pdfdom.TextAlignLeft: "L", pdfdom.TextAlignCenter: "C", pdfdom.TextAlignRight: "R", "other": "L"} {
		if got := textAlignToTableAlign(input); got != want {
			t.Errorf("textAlignToTableAlign(%q) = %q, want %q", input, got, want)
		}
	}
	for input, want := range map[string]string{"center": "C", " R ": "R", "left": "L", "other": "L"} {
		if got := tableAlignFromAttr(input); got != want {
			t.Errorf("tableAlignFromAttr(%q) = %q, want %q", input, got, want)
		}
	}
	value := 9.0
	scanFloatAttribute("12.5", &value)
	if value != 12.5 {
		t.Fatalf("scanned float = %v", value)
	}
	scanFloatAttribute("invalid", &value)
	if value != 12.5 {
		t.Fatalf("invalid float changed target to %v", value)
	}
	integer := 9
	scanIntAttribute("3", &integer)
	if integer != 3 {
		t.Fatalf("scanned integer = %v", integer)
	}
}

func TestLayoutFlowStateAndPageMetadata(t *testing.T) {
	settings := templateload.PageSettings{Width: 300, Height: 200, Margins: templateload.PageMargins{Top: 10, Right: 20, Bottom: 30, Left: 40}}
	layout, err := NewLayoutPDF(settings, settings, nil, nil)
	if err != nil {
		t.Fatalf("NewLayoutPDF returned error: %v", err)
	}
	layout.StartFlow()
	if layout.TotalPages() != 1 {
		t.Fatalf("total pages = %d", layout.TotalPages())
	}
	layout.EnsureTotalPagesAtLeast(3)
	layout.EnsureTotalPagesAtLeast(2)
	if layout.TotalPages() != 3 {
		t.Fatalf("total pages after growth = %d", layout.TotalPages())
	}
	x, y, width := layout.CurrentFlowBox()
	if x != 40 || y != 10 || width != 240 || layout.CurrentFlowBottom() != 170 {
		t.Fatalf("flow box = %v %v %v bottom=%v", x, y, width, layout.CurrentFlowBottom())
	}
	metadata := layout.PageTemplateData(2, 3)
	if metadata["pageNumber"] != 2 || metadata["pageNumberTotal"] != 3 || metadata["orientation"] != "landscape" || metadata["contentHeight"] != 160.0 {
		t.Fatalf("page metadata = %#v", metadata)
	}
	var warning string
	layout.SetWarningFunc(func(format string, args ...any) { warning = format })
	layout.warnf("warning")
	if warning != "warning" {
		t.Fatalf("warning callback received %q", warning)
	}
	layout.SetWarningFunc(nil)
	layout.warnf("ignored")
	layout.SetDeferFlowPageNum(true)
	called := 0
	layout.SetPageNumRenderer(func(*LayoutPDF, int, int) { called++ })
	layout.renderPageNum(1, 3)
	if called != 1 {
		t.Fatalf("page renderer calls = %d", called)
	}

	var nilLayout *LayoutPDF
	nilLayout.SetWarningFunc(nil)
	nilLayout.SetPageNumRenderer(nil)
	nilLayout.SetDeferFlowPageNum(true)
	nilLayout.EnsureTotalPagesAtLeast(2)
	if nilLayout.TotalPages() != 0 || nilLayout.PageTemplateData(0, 0)["pageNumber"] != 0 {
		t.Fatal("nil layout accessors returned unexpected state")
	}
}

func TestPageLimitErrorAndCanceledLayout(t *testing.T) {
	err := &PageLimitError{Limit: 2, Requested: 3}
	if !strings.Contains(err.Error(), "limit=2") || !errors.Is(err, err) {
		t.Fatalf("unexpected page limit error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	settings := templateload.PageSettings{Width: 200, Height: 300, Margins: templateload.PageMargins{Top: 10, Right: 10, Bottom: 10, Left: 10}}
	if _, createErr := NewLayoutPDFWithOptions(settings, settings, nil, nil, LayoutOptions{Context: ctx}); !errors.Is(createErr, context.Canceled) {
		t.Fatalf("canceled layout error = %v", createErr)
	}
}
