package docflowpdf

import (
	"context"
	"fmt"
	"strings"

	"github.com/otuschhoff/csspdf/internal/flowrender"
	"github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

type renderArtifact struct {
	layout *pdfrender.LayoutPDF
}

type artifactBuilder struct {
	ctx    context.Context
	input  RenderInput
	limits RenderLimits
	warnf  func(string, ...any)
}

func buildArtifactWithLimits(input RenderInput) (*renderArtifact, error) {
	limits, err := normalizeRenderLimits(input.Limits)
	if err != nil {
		return nil, err
	}
	return buildArtifactContext(context.Background(), input, limits)
}

func buildArtifactContext(ctx context.Context, input RenderInput, limits RenderLimits) (*renderArtifact, error) {
	builder := &artifactBuilder{ctx: ctx, input: input, limits: limits, warnf: warningFunc(input)}
	assets, err := builder.prepareAssets()
	if err != nil {
		return nil, err
	}
	fonts, err := resolveLayoutFontRegistrations(ctx, input, limits)
	if err != nil {
		return nil, err
	}
	layout, err := builder.prepareLayout(assets, fonts)
	if err != nil {
		return nil, err
	}
	sourceData, err := builder.prepareSourceData(assets)
	if err != nil {
		return nil, err
	}
	return builder.renderArtifact(layout, assets, sourceData)
}

func (b *artifactBuilder) prepareAssets() (Assets, error) {
	assets, err := resolveRenderAssetsContext(b.ctx, b.input, b.limits)
	if err != nil {
		return Assets{}, err
	}
	if err := validateCombinedSourceBudgets(assets, b.limits.SourceBytes); err != nil {
		return Assets{}, err
	}
	effectiveHTML, err := effectiveTemplateHTML(assets)
	if err != nil {
		return Assets{}, err
	}
	effectiveCSS, err := effectiveTemplateCSS(assets)
	if err != nil {
		return Assets{}, err
	}
	if err := validateSourceSize("template HTML source bytes", effectiveHTML, b.limits.SourceBytes); err != nil {
		return Assets{}, err
	}
	if err := validateSourceSize("template CSS source bytes", effectiveCSS, b.limits.SourceBytes); err != nil {
		return Assets{}, err
	}
	b.warnResolvedCSS(assets)
	assets.CSS = effectiveCSS
	return assets, nil
}

func (b *artifactBuilder) warnResolvedCSS(assets Assets) {
	hasLayers := len(assets.CSSLayers) > 0
	hasLegacy := strings.TrimSpace(assets.CSS) != ""
	if hasLayers {
		b.warnf("resolved CSS layer order (low->high): %s", formatResolvedCSSLayers(assets.CSSLayers, hasLegacy))
	}
	if hasLayers && hasLegacy {
		b.warnf("both CSSLayers and legacy CSS are set; legacy CSS is applied as the final implicit layer")
	}
}

func (b *artifactBuilder) prepareLayout(assets Assets, fonts []pdfrender.FontRegistration) (*pdfrender.LayoutPDF, error) {
	width, height, err := resolvePageDimensions(b.input)
	if err != nil {
		return nil, err
	}
	locale := defaultString(b.input.DefaultLocale, DefaultLocale)
	currencyCode := defaultString(b.input.DefaultCurrencyCode, DefaultCurrencyCode)
	defaults := templateload.PageSettings{Width: width, Height: height, Margins: b.input.DefaultMargins}
	defaultPage, firstPage, err := templateload.ParseCSSPageSettings(assets.CSS, defaults, templateload.ParseLengthValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @page settings from template CSS: %w", err)
	}
	setPageDimensions(&defaultPage, width, height)
	setPageDimensions(&firstPage, width, height)
	i18nInst, err := resolveI18nInputContext(b.ctx, locale, b.input, b.limits)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize i18n: %w", err)
	}
	return pdfrender.NewLayoutPDFWithOptions(defaultPage, firstPage, i18nInst, format.New(i18nInst, currencyCode), pdfrender.LayoutOptions{
		FontRegistrations: fonts, ImageSearchDirs: resolveImageSearchDirs(b.input),
		ImageLoader: confinedImageLoader(b.ctx, b.input, b.limits), StrictRenderErrors: !b.input.AllowPartialRender,
		Context: b.ctx, MaxPages: b.limits.Pages,
	})
}

func defaultString(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func setPageDimensions(page *templateload.PageSettings, width, height float64) {
	page.Width = width
	page.Height = height
}

func (b *artifactBuilder) prepareSourceData(assets Assets) (map[string]any, error) {
	source := b.input.SourceData
	if source == nil {
		source = assets.SourceData
	}
	if source == nil {
		return nil, fmt.Errorf("source data is required")
	}
	data, err := asJSONObjectWithLimit(source, b.limits.SourceBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build source JSON payload: %w", err)
	}
	return data, nil
}

func (b *artifactBuilder) renderArtifact(layout *pdfrender.LayoutPDF, assets Assets, sourceData map[string]any) (*renderArtifact, error) {
	layout.SetWarningFunc(b.warnf)
	layout.StartFlow()
	layout.SetDeferFlowPageNum(true)
	complexity := &flowrender.ComplexityBudget{}
	pageNumberRenderError := configurePageNumberRenderer(b.ctx, b.limits, complexity, layout, assets, sourceData, b.input, b.warnf)
	if err := layout.BeginPage(1); err != nil {
		return nil, err
	}
	if err := renderMainFlow(b.ctx, b.limits, complexity, layout, assets, sourceData, b.input); err != nil {
		if policyErr := handleMainFlowError(err, b.input, b.warnf); policyErr != nil {
			return nil, policyErr
		}
	}
	if layout.TotalPages() < layout.PDF.PageNo() {
		layout.EnsureTotalPagesAtLeast(layout.PDF.PageNo())
	}
	layout.RenderFinalFlowPageNums()
	if err := pageNumberRenderError(); err != nil {
		return nil, err
	}
	return &renderArtifact{layout: layout}, nil
}

func validateCombinedSourceBudgets(assets Assets, limit int64) error {
	if actual, exceeded := combinedHTMLSourceBytes(assets, limit); exceeded {
		return &BudgetError{Stage: "template HTML source bytes", Limit: limit, Actual: actual}
	}
	if actual, exceeded := combinedCSSSourceBytes(assets, limit); exceeded {
		return &BudgetError{Stage: "template CSS source bytes", Limit: limit, Actual: actual}
	}
	return nil
}

func validateSourceSize(stage, source string, limit int64) error {
	if int64(len(source)) > limit {
		return &BudgetError{Stage: stage, Limit: limit, Actual: int64(len(source))}
	}
	return nil
}

func combinedHTMLSourceBytes(assets Assets, limit int64) (int64, bool) {
	parts := make([]string, 0, len(assets.HTMLLayers)+1)
	for _, layer := range assets.HTMLLayers {
		parts = append(parts, layer.HTML)
	}
	return combinedSourceBytes(append(parts, assets.HTML), limit)
}

func combinedCSSSourceBytes(assets Assets, limit int64) (int64, bool) {
	parts := make([]string, 0, len(assets.CSSLayers)+1)
	for _, layer := range assets.CSSLayers {
		parts = append(parts, layer.CSS)
	}
	return combinedSourceBytes(append(parts, assets.CSS), limit)
}

func combinedSourceBytes(parts []string, limit int64) (int64, bool) {
	var total int64
	for _, part := range parts {
		size := int64(len(part))
		if size > limit-total {
			return limit + 1, true
		}
		total += size
	}
	return total, false
}

func formatResolvedCSSLayers(layers []CSSLayer, hasLegacyCSS bool) string {
	if len(layers) == 0 {
		if hasLegacyCSS {
			return "legacy-css"
		}
		return "none"
	}
	parts := make([]string, 0, len(layers)+1)
	for idx, layer := range layers {
		name := strings.TrimSpace(layer.Name)
		if name == "" {
			name = fmt.Sprintf("layer-%d", idx+1)
		}
		parts = append(parts, name)
	}
	if hasLegacyCSS {
		parts = append(parts, "legacy-css")
	}
	return strings.Join(parts, " -> ")
}
