package docflowpdf

import (
	"encoding/json"
	"fmt"
	htmltmpl "html/template"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/otuschhoff/invoice-gen/internal/format"
	"github.com/otuschhoff/invoice-gen/internal/i18n"
	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
	"github.com/otuschhoff/invoice-gen/internal/pdfrender"
	templateload "github.com/otuschhoff/invoice-gen/internal/template"
	"github.com/otuschhoff/invoice-gen/internal/templateflow"
)

const (
	DefaultLocale       = "en"
	DefaultCurrencyCode = "EUR"
)

// FuncMapFactory builds template functions using a default locale and payload locale.
type FuncMapFactory func(defaultLocale, payloadLocale string) htmltmpl.FuncMap

// RenderInput contains all data and options needed to render a flow-driven PDF.
type RenderInput struct {
	OutputPath          string
	Assets              Assets
	AssetInput          *AssetInput
	SourceData          any
	PageWidth           float64
	PageHeight          float64
	PageCount           int
	DefaultLocale       string
	DefaultCurrencyCode string
	DefaultMargins      templateload.PageMargins
	FuncMapFactory      FuncMapFactory
	WarningWriter       io.Writer
}

// Render renders HTML template + CSS + source data + flow JSON to a PDF file.
func Render(input RenderInput) error {
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if input.PageCount < 1 {
		return fmt.Errorf("pageCount must be at least 1")
	}
	if input.PageWidth < 0 {
		return fmt.Errorf("pageWidth must be zero or greater")
	}
	if input.PageHeight < 0 {
		return fmt.Errorf("pageHeight must be zero or greater")
	}

	assets, err := resolveRenderAssets(input)
	if err != nil {
		return err
	}

	effectiveWidth := templateload.A4Width
	if input.PageWidth > 0 {
		effectiveWidth = input.PageWidth
	}
	effectiveHeight := templateload.A4Height
	if input.PageHeight > 0 {
		effectiveHeight = input.PageHeight
	}

	locale := strings.TrimSpace(input.DefaultLocale)
	if locale == "" {
		locale = DefaultLocale
	}
	currencyCode := strings.TrimSpace(input.DefaultCurrencyCode)
	if currencyCode == "" {
		currencyCode = DefaultCurrencyCode
	}

	defaults := templateload.PageSettings{
		Width:   effectiveWidth,
		Height:  effectiveHeight,
		Margins: input.DefaultMargins,
	}
	defaultPage, firstPage, err := templateload.ParseCSSPageSettings(assets.CSS, defaults, templateload.ParseLengthValue)
	if err != nil {
		return fmt.Errorf("failed to parse @page settings from template CSS: %w", err)
	}
	defaultPage.Width = effectiveWidth
	firstPage.Width = effectiveWidth
	defaultPage.Height = effectiveHeight
	firstPage.Height = effectiveHeight

	i18nInst, err := i18n.New(locale)
	if err != nil {
		return fmt.Errorf("failed to initialize i18n: %w", err)
	}
	formatter := format.New(i18nInst, currencyCode)
	l, err := pdfrender.NewLayoutPDF(defaultPage, firstPage, i18nInst, formatter)
	if err != nil {
		return err
	}

	source := input.SourceData
	if source == nil {
		source = assets.SourceData
	}
	if source == nil {
		return fmt.Errorf("source data is required")
	}
	sourceData, err := asJSONObject(source)
	if err != nil {
		return fmt.Errorf("failed to build source JSON payload: %w", err)
	}

	warnWriter := input.WarningWriter
	if warnWriter == nil {
		warnWriter = os.Stderr
	}

	l.StartFlow(input.PageCount)
	l.DeferFlowPageNum = true
	l.PageNumRenderer = func(layout *pdfrender.LayoutPDF, page, pageCount int) {
		if page <= 1 {
			return
		}
		elements, e := pageNumberTemplateFlowElements(layout, assets, page, pageCount, input.FuncMapFactory)
		if e != nil {
			fmt.Fprintf(warnWriter, "Warning: failed to render page-number template: %v\n", e)
			return
		}
		pdfrender.RenderDocTemplateFlow(layout, elements)
	}

	l.BeginPage(1)
	if err := renderMainFlow(l, assets, sourceData, locale, input.FuncMapFactory); err != nil {
		fmt.Fprintf(warnWriter, "Warning: %v\n", err)
	}

	if l.TotalPages < l.PDF.PageNo() {
		l.TotalPages = l.PDF.PageNo()
	}
	l.RenderFinalFlowPageNums()
	if err := l.PDF.OutputFileAndClose(input.OutputPath); err != nil {
		return fmt.Errorf("failed to write PDF to %s: %w", input.OutputPath, err)
	}
	return nil
}

func resolveRenderAssets(input RenderInput) (Assets, error) {
	if input.AssetInput != nil {
		return input.AssetInput.ResolveAssets()
	}
	if err := input.Assets.Validate(); err != nil {
		return Assets{}, err
	}
	return input.Assets, nil
}

func renderMainFlow(layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, defaultLocale string, funcFactory FuncMapFactory) error {
	ctx := transformContext{
		Layout: layout,
		Source: source,
	}
	for _, section := range assets.Flow.MainFlow {
		payload, err := transformSectionPayload(section, ctx)
		if err != nil {
			return fmt.Errorf("failed to transform section %q via %q: %w", section.Template, section.Transformer, err)
		}
		jsonData, err := asJSONObject(payload)
		if err != nil {
			return fmt.Errorf("invalid JSON section payload for template %q: %w", section.Template, err)
		}

		var funcs htmltmpl.FuncMap
		if funcFactory != nil {
			funcs = funcFactory(defaultLocale, requiredString(jsonData, "locale"))
		}

		elements, err := templateflow.BuildNamedElementsWithFuncs(assets.HTML, section.Template, assets.CSS, jsonData, funcs)
		if err != nil {
			return err
		}
		pdfrender.RenderDocTemplateFlow(layout, elements)
	}
	return nil
}

func pageNumberTemplateFlowElements(layout *pdfrender.LayoutPDF, assets Assets, page, total int, funcFactory FuncMapFactory) ([]pdfdom.PDFElementNode, error) {
	payload, err := transformSectionPayload(assets.Flow.PageNumber, transformContext{Layout: layout, Page: page, Total: total})
	if err != nil {
		return nil, err
	}
	data, err := asJSONObject(payload)
	if err != nil {
		return nil, err
	}

	var funcs htmltmpl.FuncMap
	if funcFactory != nil {
		funcs = funcFactory(layout.I18n.Locale(), requiredString(data, "locale"))
	}

	return templateflow.BuildNamedElementsWithFuncs(assets.HTML, assets.Flow.PageNumber.Template, assets.CSS, data, funcs)
}

type transformContext struct {
	Layout *pdfrender.LayoutPDF
	Source map[string]any
	Page   int
	Total  int
}

func transformSectionPayload(section Section, ctx transformContext) (map[string]any, error) {
	switch section.Transformer {
	case "generic":
		return transformGenericSection(section, ctx)
	default:
		return nil, fmt.Errorf("unknown section transformer %q", section.Transformer)
	}
}

func transformGenericSection(section Section, ctx transformContext) (map[string]any, error) {
	payload := make(map[string]any)
	if section.Payload.IncludeSource {
		payload["Source"] = ctx.Source
	}
	for path, value := range section.Payload.Static {
		setNestedValue(payload, path, value)
	}
	for path, runtimeExpr := range section.Payload.Runtime {
		value, err := resolveRuntimeValue(runtimeExpr, ctx)
		if err != nil {
			return nil, err
		}
		setNestedValue(payload, path, value)
	}

	locale := resolvePayloadLocale(section.Payload.LocalePath, ctx)
	payload["locale"] = locale

	if ctx.Layout != nil && ctx.Layout.I18n != nil {
		vars := extractStringVarsFromSourcePaths(ctx.Source, section.Payload.I18nVars)
		payload["i18n"] = ctx.Layout.I18n.TemplateData(vars)
	}

	return payload, nil
}

func resolvePayloadLocale(localePath string, ctx transformContext) string {
	if ctx.Source != nil && strings.TrimSpace(localePath) != "" {
		if locale, ok := getStringAtPath(ctx.Source, localePath); ok {
			return locale
		}
	}
	if ctx.Layout != nil && ctx.Layout.I18n != nil {
		if locale := strings.TrimSpace(ctx.Layout.I18n.Locale()); locale != "" {
			return locale
		}
	}
	if ctx.Source != nil {
		if locale, ok := getStringAtPath(ctx.Source, "locale"); ok {
			return locale
		}
	}
	return DefaultLocale
}

func resolveRuntimeValue(expr string, ctx transformContext) (any, error) {
	switch {
	case expr == "flow.tableWidth":
		if ctx.Layout == nil {
			return nil, fmt.Errorf("runtime value %q requires layout", expr)
		}
		_, _, width := ctx.Layout.CurrentFlowBox()
		return width, nil
	case strings.HasPrefix(expr, "flow.remainingWidth:"):
		if ctx.Layout == nil {
			return nil, fmt.Errorf("runtime value %q requires layout", expr)
		}
		_, _, width := ctx.Layout.CurrentFlowBox()
		spec := strings.TrimPrefix(expr, "flow.remainingWidth:")
		for _, part := range strings.Split(spec, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid remaining width part %q in %q: %w", value, expr, err)
			}
			width -= n
		}
		return width, nil
	case expr == "page.number":
		return ctx.Page, nil
	case expr == "page.total":
		return ctx.Total, nil
	default:
		return nil, fmt.Errorf("unknown runtime value expression %q", expr)
	}
}

func setNestedValue(target map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		target[parts[0]] = value
		return
	}

	current := target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func getValueAtPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func getStringAtPath(root map[string]any, path string) (string, bool) {
	v, ok := getValueAtPath(root, path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func extractStringVarsFromSourcePaths(source map[string]any, paths map[string]string) map[string]string {
	if len(paths) == 0 || source == nil {
		return nil
	}
	vars := make(map[string]string, len(paths))
	for name, path := range paths {
		if value, ok := getStringAtPath(source, path); ok {
			vars[name] = value
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return vars
}

func asJSONObject(data any) (map[string]any, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data to JSON: %w", err)
	}
	out := make(map[string]any)
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data from JSON: %w", err)
	}
	return out, nil
}

func requiredString(root map[string]any, key string) string {
	if s, ok := root[key].(string); ok {
		return s
	}
	return ""
}
