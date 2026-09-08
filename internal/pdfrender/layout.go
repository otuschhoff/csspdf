package pdfrender

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
	"github.com/otuschhoff/gofpdf"
)

const (
	footerBulletSep = " • "
)

// LayoutPDF holds the shared PDF document and pre-built templates used by all
// standalone rendering subcommands. Build one with NewLayoutPDF; use the
// exported methods to compose pages.
type LayoutPDF struct {
	PDF                   *gofpdf.Fpdf
	Formatter             *format.Formatter
	I18n                  *i18n.I18n
	imageSearchDirs       []string
	imageLoader           ImageLoader
	strictRenderErrors    bool
	tableRenderer         *TableRenderer
	warningf              func(format string, args ...any)
	pageNumRenderer       func(l *LayoutPDF, page, pageCount int)
	deferFlowPageNum      bool
	currentPage           int
	totalPages            int
	flowCursorY           float64
	flowBottomMargin      float64
	flowCursorValid       bool
	pageWidth             float64
	pageHeight            float64
	currentMargins        templateload.PageMargins
	defaultPage           templateload.PageSettings
	firstPage             templateload.PageSettings
	defaultAssets         layoutPageAssets
	firstAssets           layoutPageAssets
	runningFooterTemplate gofpdf.Template
	runningFooterSize     gofpdf.SizeType
	userTemplates         map[string]gofpdf.Template
	templateFactories     map[string]TemplateFactory
	namedTemplates        map[string]gofpdf.Template
	ctx                   context.Context
	maxPages              int
}

type layoutPageAssets struct {
	pageWidth  float64
	pageHeight float64
}

// FontRegistration declares one font family/style with ordered candidate file
// paths. The first existing file path will be loaded.
// NewLayoutPDF creates a LayoutPDF from page settings and locale/format helpers.
func NewLayoutPDF(defaultPage, firstPage templateload.PageSettings, i18nInst *i18n.I18n, formatter *format.Formatter) (*LayoutPDF, error) {
	return NewLayoutPDFWithOptions(defaultPage, firstPage, i18nInst, formatter, LayoutOptions{})
}

// NewLayoutPDFWithOptions creates a LayoutPDF and applies optional profile
// configuration such as custom font registrations.
func NewLayoutPDFWithOptions(defaultPage, firstPage templateload.PageSettings, i18nInst *i18n.I18n, formatter *format.Formatter, options LayoutOptions) (*LayoutPDF, error) {
	if err := validatePageSettings("default page", defaultPage); err != nil {
		return nil, err
	}
	if err := validatePageSettings("first page", firstPage); err != nil {
		return nil, err
	}
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.SetCatalogSort(true)
	if !options.MetadataTime.IsZero() {
		pdf.SetCreationDate(options.MetadataTime)
		pdf.SetModificationDate(options.MetadataTime)
	}
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: firstPage.Width, Ht: firstPage.Height})

	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.MaxPages < 0 {
		return nil, fmt.Errorf("maximum pages must not be negative")
	}
	if options.MaxPages > 0 && options.MaxPages < 1 {
		return nil, fmt.Errorf("maximum pages must allow the first page")
	}
	if err := loadDefaultFontSet(pdf, options.FontRegistrations); err != nil {
		return nil, err
	}

	defaultAssets := buildLayoutPageAssets(defaultPage.Width, defaultPage.Height)
	firstAssets := buildLayoutPageAssets(firstPage.Width, firstPage.Height)

	tableRenderer := NewTableRenderer(pdf, formatter)

	return &LayoutPDF{
		PDF:                pdf,
		Formatter:          formatter,
		I18n:               i18nInst,
		imageSearchDirs:    append([]string(nil), options.ImageSearchDirs...),
		imageLoader:        options.ImageLoader,
		strictRenderErrors: options.StrictRenderErrors,
		tableRenderer:      tableRenderer,
		warningf: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
		},
		pageWidth:         firstPage.Width,
		pageHeight:        firstPage.Height,
		currentMargins:    firstPage.Margins,
		defaultPage:       defaultPage,
		firstPage:         firstPage,
		defaultAssets:     defaultAssets,
		firstAssets:       firstAssets,
		templateFactories: normalizedTemplateFactories(options.TemplateFactories),
		ctx:               ctx,
		maxPages:          options.MaxPages,
	}, nil
}

func (l *LayoutPDF) newTextEngine(pdf *gofpdf.Fpdf, i18n Translator) *PDFTextEngine {
	engine := NewPDFTextEngine(pdf, i18n)
	engine.SetValueFormatter(l.Formatter)
	engine.SetImageSearchDirs(l.imageSearchDirs)
	engine.SetImageLoader(l.imageLoader)
	return engine
}

// SetWarningFunc sets the warning sink used by non-fatal renderer warnings.
// Passing nil disables warning output.
func (l *LayoutPDF) SetWarningFunc(fn func(format string, args ...any)) {
	if l == nil {
		return
	}
	l.warningf = fn
}

func (l *LayoutPDF) warnf(format string, args ...any) {
	if l == nil || l.warningf == nil {
		return
	}
	l.warningf(format, args...)
}

// SetPageNumRenderer sets the callback used to render page numbers.
func (l *LayoutPDF) SetPageNumRenderer(fn func(layout *LayoutPDF, page, pageCount int)) {
	if l == nil {
		return
	}
	l.pageNumRenderer = fn
}

// SetDeferFlowPageNum controls whether page numbers are deferred until
// RenderFinalFlowPageNums.
func (l *LayoutPDF) SetDeferFlowPageNum(v bool) {
	if l == nil {
		return
	}
	l.deferFlowPageNum = v
}

// TotalPages returns the current tracked total page count.
func (l *LayoutPDF) TotalPages() int {
	if l == nil {
		return 0
	}
	return l.totalPages
}

// EnsureTotalPagesAtLeast sets the total page count to n if n is larger than
// the current tracked value.
func (l *LayoutPDF) EnsureTotalPagesAtLeast(n int) {
	if l == nil {
		return
	}
	if n > l.totalPages {
		l.totalPages = n
	}
}

func buildLayoutPageAssets(pageWidth, pageHeight float64) layoutPageAssets {
	return layoutPageAssets{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
	}
}

// BeginPage handles pagination (AddPage on page > 1) and stamps the running
// footer template on every page.
func (l *LayoutPDF) BeginPage(page int) error {
	if err := l.ctx.Err(); err != nil {
		return fmt.Errorf("layout canceled before page %d: %w", page, err)
	}
	if l.maxPages > 0 && page > l.maxPages {
		return &PageLimitError{Limit: l.maxPages, Requested: page}
	}
	settings, assets := l.pageConfigFor(page)
	if page > 1 {
		l.PDF.AddPageFormat("P", gofpdf.SizeType{Wd: assets.pageWidth, Ht: assets.pageHeight})
	}
	l.pageWidth = assets.pageWidth
	l.pageHeight = assets.pageHeight
	l.currentMargins = settings.Margins
	l.renderRunningFooterTemplate()
	return nil
}

func (l *LayoutPDF) renderRunningFooterTemplate() {
	if l.runningFooterTemplate == nil || l.runningFooterSize.Wd <= 0 || l.runningFooterSize.Ht <= 0 {
		return
	}
	x := (l.pageWidth - l.runningFooterSize.Wd) / 2
	if x < 0 {
		x = 0
	}
	y := l.pageHeight - l.currentMargins.Bottom - l.runningFooterSize.Ht
	if y < 0 {
		y = l.pageHeight - l.runningFooterSize.Ht
	}
	if y < 0 {
		y = 0
	}
	l.PDF.UseTemplateScaled(l.runningFooterTemplate, gofpdf.PointType{X: x, Y: y}, l.runningFooterSize)
}

func (l *LayoutPDF) pageConfigFor(page int) (templateload.PageSettings, layoutPageAssets) {
	if page == 1 {
		return l.firstPage, l.firstAssets
	}
	return l.defaultPage, l.defaultAssets
}

// RenderFinalFlowPageNums re-renders page numbers on all non-first pages after
// the total page count is known.
func (l *LayoutPDF) RenderFinalFlowPageNums() {
	finalTotal := l.totalPages
	for page := 2; page <= finalTotal; page++ {
		settings, assets := l.pageConfigFor(page)
		l.pageWidth = assets.pageWidth
		l.pageHeight = assets.pageHeight
		l.currentMargins = settings.Margins
		l.PDF.SetPage(page)
		l.renderPageNum(page, finalTotal)
	}
}

func (l *LayoutPDF) renderPageNum(page, pageCount int) {
	if l.pageNumRenderer != nil {
		l.pageNumRenderer(l, page, pageCount)
		return
	}
	l.RenderPageNum(page, pageCount)
}

// PageTemplateData returns page/runtime metadata intended for template payloads.
// Values are resolved from the current layout state and optional explicit
// pageNumber/pageTotal overrides.
func (l *LayoutPDF) PageTemplateData(pageNumber, pageTotal int) map[string]any {
	data := map[string]any{
		"pageNumber":      pageNumber,
		"pageNumberTotal": pageTotal,
		"width":           0.0,
		"height":          0.0,
		"orientation":     "portrait",
		"marginLeft":      0.0,
		"marginRight":     0.0,
		"marginTop":       0.0,
		"marginBottom":    0.0,
		"contentX":        0.0,
		"contentY":        0.0,
		"contentWidth":    0.0,
		"contentHeight":   0.0,
		"contentBottom":   0.0,
	}
	if l == nil {
		return data
	}

	if pageNumber <= 0 {
		pageNumber = l.PDF.PageNo()
	}
	if pageNumber <= 0 {
		pageNumber = l.currentPage
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}

	if pageTotal <= 0 {
		pageTotal = l.totalPages
	}
	if pageTotal < pageNumber {
		pageTotal = pageNumber
	}

	orientation := "portrait"
	if l.pageWidth > l.pageHeight {
		orientation = "landscape"
	}

	contentX, contentY, contentW := l.CurrentFlowBox()
	contentBottom := l.CurrentFlowBottom()
	contentH := contentBottom - contentY
	if contentH < 0 {
		contentH = 0
	}

	data["pageNumber"] = pageNumber
	data["pageNumberTotal"] = pageTotal
	data["width"] = l.pageWidth
	data["height"] = l.pageHeight
	data["orientation"] = orientation
	data["marginLeft"] = l.currentMargins.Left
	data["marginRight"] = l.currentMargins.Right
	data["marginTop"] = l.currentMargins.Top
	data["marginBottom"] = l.currentMargins.Bottom
	data["contentX"] = contentX
	data["contentY"] = contentY
	data["contentWidth"] = contentW
	data["contentHeight"] = contentH
	data["contentBottom"] = contentBottom

	return data
}

// ShouldBreakPageBefore reports whether node carries a break-before:page attribute.
func ShouldBreakPageBefore(node pdfdom.PDFElementNode) bool {
	value := flowBreakValue(node, "breakBefore")
	if value == "" {
		value = flowBreakValue(node, "break-before")
	}
	return value == "page"
}

// ShouldBreakPageAfter reports whether node carries a break-after:page attribute.
func ShouldBreakPageAfter(node pdfdom.PDFElementNode) bool {
	value := flowBreakValue(node, "breakAfter")
	if value == "" {
		value = flowBreakValue(node, "break-after")
	}
	return value == "page"
}

func flowBreakValue(node pdfdom.PDFElementNode, attr string) string {
	if node == nil {
		return ""
	}
	if value, ok := node.Attribute(attr); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

// RenderPageNum draws the current/total page number in the top-right corner.
// It is a no-op on page 1.
func (l *LayoutPDF) RenderPageNum(page, pageCount int) {
	if page <= 1 {
		return
	}
	l.PDF.SetTextColor(0x22, 0x22, 0x22)
	l.PDF.SetFont("Helvetica", "", 10)
	l.PDF.Text(l.pageWidth-98, 43, fmt.Sprintf("%d ", page))
	l.PDF.SetTextColor(0x66, 0x66, 0x66)
	l.PDF.SetFont("Helvetica", "", 7)
	l.PDF.Text(l.pageWidth-89, 43, fmt.Sprintf("/ %d", pageCount))
}

// resolveFlowPlacementWith is the pure-function core of placement resolution.
// pageW is the reference page/container width used for right-edge calculations.
func resolveFlowPlacementWith(node pdfdom.PDFElementNode, flowX, flowY, flowW, pageW float64) (x, y, width float64, absolute bool) {
	x = flowX
	y = flowY
	width = flowW

	positionRaw, hasPosition := node.Attribute("position")
	if !hasPosition || !strings.EqualFold(strings.TrimSpace(positionRaw), "absolute") {
		return x, y, width, false
	}

	absolute = true

	left := htmlLengthToFloat(node, "left")
	right := htmlLengthToFloat(node, "right")
	top := htmlLengthToFloat(node, "top")
	declaredWidth := htmlLengthToFloat(node, "width")

	if top > 0 {
		y = top
	}

	if declaredWidth > 0 {
		width = declaredWidth
	} else if left > 0 && right > 0 {
		width = max(1, pageW-left-right)
	}

	if left > 0 {
		x = left
	} else if right > 0 {
		x = pageW - right - width
	}

	return x, y, width, absolute
}

// ResolveFlowPlacement determines the final position and width of a flow element,
// applying absolute positioning when declared.
func (l *LayoutPDF) ResolveFlowPlacement(node pdfdom.PDFElementNode, flowX, flowY, flowW float64) (x, y, width float64, absolute bool) {
	return resolveFlowPlacementWith(node, flowX, flowY, flowW, l.pageWidth)
}

// TemplateByName returns the pre-built gofpdf.Template for a named slot.
func (l *LayoutPDF) TemplateByName(name string) (gofpdf.Template, error) {
	// User-defined templates (created by <create-template> elements) take
	// precedence over the built-in named slots.
	if l.userTemplates != nil {
		if tpl, ok := l.userTemplates[name]; ok {
			return tpl, nil
		}
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if template, ok := l.namedTemplates[key]; ok {
		return template, nil
	}
	if factory, ok := l.templateFactories[key]; ok {
		template := factory(l.PDF)
		if l.namedTemplates == nil {
			l.namedTemplates = make(map[string]gofpdf.Template)
		}
		l.namedTemplates[key] = template
		return template, nil
	}
	return nil, fmt.Errorf("unknown template name %q", name)
}

func normalizedTemplateFactories(factories map[string]TemplateFactory) map[string]TemplateFactory {
	result := make(map[string]TemplateFactory, len(factories))
	for name, factory := range factories {
		if key := strings.ToLower(strings.TrimSpace(name)); key != "" && factory != nil {
			result[key] = factory
		}
	}
	return result
}

// RenderUseTemplateElement renders a <use-template> element using the named template.
// fallbackX and fallbackY are the current document-flow coordinates; they are
// used when the element does not carry explicit x or y attributes.
// Returns the rendered height, whether an explicit position was given (absolute=true
// means the caller should NOT advance the flow cursor), and any error.
func (l *LayoutPDF) RenderUseTemplateElement(elem *pdfdom.ElemUseTemplate, fallbackX, fallbackY float64) (renderedHeight float64, absolute bool, err error) {
	if elem == nil {
		return 0, false, fmt.Errorf("use-template element is nil")
	}

	name, ok := elem.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return 0, false, fmt.Errorf("use-template missing name attribute")
	}

	x, y := fallbackX, fallbackY
	hasExplicitPos := false
	if raw, hasX := elem.Attribute("x"); hasX {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			x = v
			absolute = true
			hasExplicitPos = true
		}
	}
	if raw, hasY := elem.Attribute("y"); hasY {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			y = v
			absolute = true
			hasExplicitPos = true
		}
	}

	tpl, err := l.TemplateByName(name)
	if err != nil {
		return 0, false, err
	}

	// width and height may come from HTML attributes or from CSS (which
	// CSSDeclarationToAttr has already baked into the element's attributes).
	_, nativeSize := tpl.Size()
	width, height := nativeSize.Wd, nativeSize.Ht
	hasExplicitSize := false
	if raw, ok := elem.Attribute("width"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			width = v
			hasExplicitSize = true
		}
	}
	if raw, ok := elem.Attribute("height"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			height = v
			hasExplicitSize = true
		}
	}

	// When no explicit position or size was given, let gofpdf place the
	// template at its own registered corner/size (UseTemplate).  Otherwise
	// use UseTemplateScaled so the caller's coordinates are respected.
	if !hasExplicitPos && !hasExplicitSize {
		l.PDF.UseTemplate(tpl)
	} else {
		l.PDF.UseTemplateScaled(
			tpl,
			gofpdf.PointType{X: x, Y: y},
			gofpdf.SizeType{Wd: width, Ht: height},
		)
	}

	return height, absolute, nil
}
