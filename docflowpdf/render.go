package docflowpdf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmltmpl "html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/otuschhoff/csspdf/internal/flowrender"
	"github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

const (
	DefaultLocale       = "en"
	DefaultCurrencyCode = "EUR"
	DefaultPageFormat   = "A4"

	PageOrientationPortrait  = "portrait"
	PageOrientationLandscape = "landscape"
)

var errI18nMacroExpansion = errors.New("i18n macro expansion failed")

type i18nTemplateValueError struct {
	Path  string
	Value string
	Err   error
}

func (e *i18nTemplateValueError) Error() string {
	return fmt.Sprintf("path %s: %v", e.Path, e.Err)
}

func (e *i18nTemplateValueError) Unwrap() error {
	return e.Err
}

var undefinedTemplateFunctionPattern = regexp.MustCompile(`function "([^"]+)" not defined`)
var templateExecutionTokenPattern = regexp.MustCompile(`at <([^>]+)>`)

type fatalFlowRenderError struct {
	err error
}

func (e *fatalFlowRenderError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *fatalFlowRenderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// RenderInput contains all data and options needed to render a flow-driven PDF.
type RenderInput struct {
	OutputPath               string
	AssetBaseDir             string
	EnableI18nTemplateMacros bool
	Assets                   Assets
	AssetInput               *AssetInput
	SourceData               any
	I18nSource               JSONSource
	FontRegistrations        []FontRegistration
	PageWidth                float64
	PageHeight               float64
	PageFormat               string
	PageOrientation          string
	DefaultLocale            string
	DefaultCurrencyCode      string
	DefaultMargins           templateload.PageMargins
	Now                      func() time.Time
	FuncMapFactoryEx         FuncMapFactoryWithContext
	FuncMapFactory           FuncMapFactory
	Logger                   Logger
	// AllowPartialRender preserves the legacy behavior of logging recoverable
	// template and element errors while emitting a potentially incomplete PDF.
	AllowPartialRender bool
	// Deprecated: use Logger.
	WarningWriter io.Writer
}

// FontRegistration declares one font family/style with ordered candidate file
// paths. The first existing path will be registered.
type FontRegistration struct {
	Family  string
	Style   string
	Sources []string
}

// RenderWithInput renders using a fully specified RenderInput.
func RenderWithInput(input RenderInput) error {
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	return RenderToFile(input, input.OutputPath)
}

// RenderToWriter renders and writes a PDF to an io.Writer.
func RenderToWriter(input RenderInput, out io.Writer) error {
	if out == nil {
		return fmt.Errorf("output writer is required")
	}
	artifact, err := buildArtifact(input)
	if err != nil {
		return err
	}
	if err := artifact.layout.PDF.Output(out); err != nil {
		return fmt.Errorf("failed to output PDF: %w", err)
	}
	return nil
}

// RenderToBytes renders and returns a complete PDF byte slice.
func RenderToBytes(input RenderInput) ([]byte, error) {
	var buf bytes.Buffer
	if err := RenderToWriter(input, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildArtifact(input RenderInput) (*renderArtifact, error) {
	warnf := warningFunc(input)

	if input.PageWidth < 0 {
		return nil, fmt.Errorf("pageWidth must be zero or greater")
	}
	if input.PageHeight < 0 {
		return nil, fmt.Errorf("pageHeight must be zero or greater")
	}

	assets, err := resolveRenderAssets(input)
	if err != nil {
		return nil, err
	}
	effectiveHTML, err := effectiveTemplateHTML(assets)
	if err != nil {
		return nil, err
	}
	effectiveCSS, err := effectiveTemplateCSS(assets)
	if err != nil {
		return nil, err
	}
	_ = effectiveHTML
	if len(assets.CSSLayers) > 0 {
		warnf("resolved CSS layer order (low->high): %s", formatResolvedCSSLayers(assets.CSSLayers, strings.TrimSpace(assets.CSS) != ""))
	}
	if len(assets.CSSLayers) > 0 && strings.TrimSpace(assets.CSS) != "" {
		warnf("both CSSLayers and legacy CSS are set; legacy CSS is applied as the final implicit layer")
	}
	assets.CSS = effectiveCSS
	resolvedFontRegistrations, err := resolveFontRegistrations(input)
	if err != nil {
		return nil, err
	}

	effectiveWidth, effectiveHeight, err := resolvePageDimensions(input)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to parse @page settings from template CSS: %w", err)
	}
	defaultPage.Width = effectiveWidth
	firstPage.Width = effectiveWidth
	defaultPage.Height = effectiveHeight
	firstPage.Height = effectiveHeight

	i18nInst, err := resolveI18nInput(locale, input)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize i18n: %w", err)
	}
	formatter := format.New(i18nInst, currencyCode)
	l, err := pdfrender.NewLayoutPDFWithOptions(defaultPage, firstPage, i18nInst, formatter, pdfrender.LayoutOptions{
		FontRegistrations:  toLayoutFontRegistrations(resolvedFontRegistrations),
		ImageSearchDirs:    resolveImageSearchDirs(input),
		StrictRenderErrors: !input.AllowPartialRender,
	})
	if err != nil {
		return nil, err
	}

	source := input.SourceData
	if source == nil {
		source = assets.SourceData
	}
	if source == nil {
		return nil, fmt.Errorf("source data is required")
	}
	sourceData, err := asJSONObject(source)
	if err != nil {
		return nil, fmt.Errorf("failed to build source JSON payload: %w", err)
	}

	l.SetWarningFunc(warnf)

	l.StartFlow()
	l.SetDeferFlowPageNum(true)
	pageNumberRenderError := configurePageNumberRenderer(l, assets, sourceData, input, warnf)

	l.BeginPage(1)
	if err := renderMainFlow(l, assets, sourceData, input); err != nil {
		if policyErr := handleMainFlowError(err, input, warnf); policyErr != nil {
			return nil, policyErr
		}
	}

	if l.TotalPages() < l.PDF.PageNo() {
		l.EnsureTotalPagesAtLeast(l.PDF.PageNo())
	}
	l.RenderFinalFlowPageNums()
	if err := pageNumberRenderError(); err != nil {
		return nil, err
	}
	return &renderArtifact{layout: l}, nil
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

func resolvePageDimensions(input RenderInput) (float64, float64, error) {
	formatName := strings.TrimSpace(input.PageFormat)
	if formatName == "" {
		formatName = DefaultPageFormat
	}

	orientation := strings.ToLower(strings.TrimSpace(input.PageOrientation))
	if orientation == "" {
		orientation = PageOrientationPortrait
	}
	if orientation != PageOrientationPortrait && orientation != PageOrientationLandscape {
		return 0, 0, fmt.Errorf("unsupported page orientation %q (expected %q or %q)", input.PageOrientation, PageOrientationPortrait, PageOrientationLandscape)
	}

	width, height, ok := templateload.ResolveNamedPageSize(formatName)
	if !ok {
		return 0, 0, fmt.Errorf("unsupported page format %q", input.PageFormat)
	}

	if orientation == PageOrientationLandscape {
		if height > width {
			width, height = height, width
		}
	} else if width > height {
		width, height = height, width
	}

	if input.PageWidth > 0 {
		width = input.PageWidth
	}
	if input.PageHeight > 0 {
		height = input.PageHeight
	}

	return width, height, nil
}

type renderArtifact struct {
	layout *pdfrender.LayoutPDF
}

func warningFunc(input RenderInput) func(string, ...any) {
	if input.Logger != nil {
		return input.Logger.Warnf
	}
	if input.WarningWriter != nil {
		return func(format string, args ...any) {
			fmt.Fprintf(input.WarningWriter, "Warning: "+format+"\n", args...)
		}
	}
	return func(string, ...any) {}
}

func resolveRenderAssets(input RenderInput) (Assets, error) {
	if baseDir := strings.TrimSpace(input.AssetBaseDir); baseDir != "" {
		overrides := AssetInput{}
		if input.AssetInput != nil {
			overrides = *input.AssetInput
		}
		return overrides.ResolveWithBaseDir(baseDir)
	}

	if input.AssetInput != nil {
		return input.AssetInput.ResolveAssets()
	}
	assets := input.Assets
	assets.Flow = cloneFlow(input.Assets.Flow)
	if _, err := effectiveTemplateHTML(assets); err != nil {
		return Assets{}, err
	}
	if err := applyFlowDefaults(&assets.Flow, templateSourcesInRenderOrder(assets)); err != nil {
		return Assets{}, err
	}
	if err := assets.Validate(); err != nil {
		return Assets{}, err
	}
	return assets, nil
}

func resolveI18nInput(locale string, input RenderInput) (*i18n.I18n, error) {
	i18nSource := input.I18nSource
	if !i18nSource.IsSet() {
		baseDir := strings.TrimSpace(input.AssetBaseDir)
		if baseDir != "" {
			candidate := filepath.Join(baseDir, "i18n.json")
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				i18nSource = JSONSource{FilePath: candidate}
			}
		}
	}

	if !i18nSource.IsSet() {
		return i18n.New(locale)
	}

	var source map[string]any
	if err := i18nSource.DecodeInto(&source, "i18n"); err != nil {
		return nil, err
	}
	return i18n.NewFromSource(locale, source)
}

func resolveFontRegistrations(input RenderInput) ([]FontRegistration, error) {
	if len(input.FontRegistrations) > 0 {
		out := make([]FontRegistration, 0, len(input.FontRegistrations))
		for _, registration := range input.FontRegistrations {
			out = append(out, normalizeFontRegistration(registration))
		}
		return dedupeFontRegistrations(out), nil
	}
	baseDir := strings.TrimSpace(input.AssetBaseDir)
	if baseDir == "" {
		return nil, nil
	}
	fontDirs := []string{
		filepath.Join(baseDir, "fonts"),
		filepath.Join(baseDir, "..", "fonts"),
		filepath.Join(baseDir, "..", "..", "fonts"),
	}

	var entries []os.DirEntry
	selectedFontDir := ""
	for _, candidate := range fontDirs {
		candidate = filepath.Clean(candidate)
		scanned, err := os.ReadDir(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to scan font directory %q: %w", candidate, err)
		}
		entries = scanned
		selectedFontDir = candidate
		break
	}
	if len(entries) == 0 {
		return nil, nil
	}

	registrations := make([]FontRegistration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
			continue
		}
		family := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
		if family == "" {
			continue
		}
		registrations = append(registrations, normalizeFontRegistration(FontRegistration{
			Family:  family,
			Style:   "",
			Sources: []string{filepath.Join(selectedFontDir, name)},
		}))
	}
	return dedupeFontRegistrations(registrations), nil
}

func normalizeFontRegistration(reg FontRegistration) FontRegistration {
	family := strings.TrimSpace(reg.Family)
	style := normalizeFontStyleCode(reg.Style)
	if style == "" {
		family, style = splitFamilyAndStyleSuffix(family)
	}
	out := reg
	out.Family = family
	out.Style = style
	return out
}

func dedupeFontRegistrations(registrations []FontRegistration) []FontRegistration {
	if len(registrations) == 0 {
		return nil
	}
	out := make([]FontRegistration, 0, len(registrations))
	seen := make(map[string]struct{}, len(registrations))
	for _, registration := range registrations {
		key := strings.ToLower(strings.TrimSpace(registration.Family)) + "|" + strings.ToUpper(strings.TrimSpace(registration.Style))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, registration)
	}
	return out
}

func splitFamilyAndStyleSuffix(family string) (string, string) {
	f := strings.TrimSpace(family)
	if f == "" {
		return "", ""
	}
	lower := strings.ToLower(f)
	type suffixStyle struct {
		suffix string
		style  string
	}
	replacements := []suffixStyle{
		{"-bold-italic", "BI"},
		{"_bold_italic", "BI"},
		{" bold italic", "BI"},
		{"-bold-oblique", "BI"},
		{"_bold_oblique", "BI"},
		{" bold oblique", "BI"},
		{"-bolditalic", "BI"},
		{"_bolditalic", "BI"},
		{" bolditalic", "BI"},
		{"-boldoblique", "BI"},
		{"_boldoblique", "BI"},
		{" boldoblique", "BI"},
		{"-italic", "I"},
		{"_italic", "I"},
		{" italic", "I"},
		{"-oblique", "I"},
		{"_oblique", "I"},
		{" oblique", "I"},
		{"-bold", "B"},
		{"_bold", "B"},
		{" bold", "B"},
		{"-regular", ""},
		{"_regular", ""},
		{" regular", ""},
		{"-normal", ""},
		{"_normal", ""},
		{" normal", ""},
		{"-roman", ""},
		{"_roman", ""},
		{" roman", ""},
	}
	for _, replacement := range replacements {
		suffix := replacement.suffix
		style := replacement.style
		if strings.HasSuffix(lower, suffix) {
			base := strings.TrimSpace(f[:len(f)-len(suffix)])
			if base == "" {
				return f, ""
			}
			return base, style
		}
	}
	return f, ""
}

func normalizeFontStyleCode(style string) string {
	s := strings.ToUpper(strings.TrimSpace(style))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "IB":
		return "BI"
	case "", "B", "I", "BI":
		return s
	case "BOLD":
		return "B"
	case "ITALIC", "OBLIQUE":
		return "I"
	case "BOLDITALIC", "ITALICBOLD", "BOLDOBLIQUE", "OBLIQUEBOLD":
		return "BI"
	default:
		return s
	}
}

func resolveImageSearchDirs(input RenderInput) []string {
	baseDir := strings.TrimSpace(input.AssetBaseDir)
	if baseDir == "" {
		return nil
	}
	return []string{
		filepath.Join(baseDir, "images"),
		filepath.Join(baseDir, "..", "images"),
	}
}

func toLayoutFontRegistrations(registrations []FontRegistration) []pdfrender.FontRegistration {
	if len(registrations) == 0 {
		return nil
	}
	out := make([]pdfrender.FontRegistration, 0, len(registrations))
	for _, registration := range registrations {
		sources := make([]string, 0, len(registration.Sources))
		sources = append(sources, registration.Sources...)
		out = append(out, pdfrender.FontRegistration{
			Family:  registration.Family,
			Style:   registration.Style,
			Sources: sources,
		})
	}
	return out
}

func renderMainFlow(layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput) error {
	ctx := transformContext{
		Layout: layout,
		Source: source,
		Input:  input,
	}
	templateSources := templateSourcesInRenderOrder(assets)
	for _, section := range assets.Flow.MainFlow {
		payload, err := transformSectionPayload(section, ctx)
		if err != nil {
			return fmt.Errorf("failed to transform section %q via %q: %w", section.Template, section.Transformer, err)
		}
		jsonData, err := asJSONObject(payload)
		if err != nil {
			return fmt.Errorf("invalid JSON section payload for template %q: %w", section.Template, err)
		}
		funcs := buildFuncMap(input, strings.TrimSpace(input.DefaultLocale), requiredString(jsonData, "locale"))

		elements, err := flowrender.BuildFlowElementsFromSourcesWithFuncs(templateSources, section.Template, assets.CSS, jsonData, funcs)
		if err != nil {
			return err
		}
		if err := pdfrender.RenderDocTemplateFlow(layout, elements); err != nil {
			return &fatalFlowRenderError{err: fmt.Errorf("failed to render section %q flow: %w", section.Template, err)}
		}
	}
	return nil
}

func pageNumberTemplateFlowElements(layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, page, total int, input RenderInput) ([]pdfdom.PDFElementNode, error) {
	templateSources := templateSourcesInRenderOrder(assets)
	payload, err := transformSectionPayload(assets.Flow.PageNumber, transformContext{Layout: layout, Source: source, Page: page, Total: total, Input: input})
	if err != nil {
		return nil, err
	}
	data, err := asJSONObject(payload)
	if err != nil {
		return nil, err
	}
	funcs := buildFuncMap(input, layout.I18n.Locale(), requiredString(data, "locale"))

	return flowrender.BuildFlowElementsFromSourcesWithFuncs(templateSources, assets.Flow.PageNumber.Template, assets.CSS, data, funcs)
}

func templateSourcesInRenderOrder(assets Assets) []string {
	sources := make([]string, 0, len(assets.HTMLLayers)+1)
	for _, layer := range assets.HTMLLayers {
		if strings.TrimSpace(layer.HTML) == "" {
			continue
		}
		sources = append(sources, layer.HTML)
	}
	if strings.TrimSpace(assets.HTML) != "" {
		sources = append(sources, assets.HTML)
	}
	return sources
}

func buildFuncMap(input RenderInput, defaultLocale, payloadLocale string) htmltmpl.FuncMap {
	if defaultLocale == "" {
		defaultLocale = DefaultLocale
	}
	defaultCurrencyCode := strings.TrimSpace(input.DefaultCurrencyCode)
	if defaultCurrencyCode == "" {
		defaultCurrencyCode = DefaultCurrencyCode
	}
	nowFn := input.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	if input.FuncMapFactoryEx != nil {
		return input.FuncMapFactoryEx(FuncContext{
			DefaultLocale:       defaultLocale,
			PayloadLocale:       payloadLocale,
			DefaultCurrencyCode: defaultCurrencyCode,
			Now:                 nowFn,
		})
	}
	if input.FuncMapFactory != nil {
		return input.FuncMapFactory(defaultLocale, payloadLocale)
	}
	return nil
}

type transformContext struct {
	Layout *pdfrender.LayoutPDF
	Source map[string]any
	Page   int
	Total  int
	Input  RenderInput
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
	payload["page"] = buildImplicitPagePayload(ctx)
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
		i18nData := ctx.Layout.I18n.TemplateData(vars)
		if ctx.Input.EnableI18nTemplateMacros {
			macroPayload, err := asJSONObject(payload)
			if err != nil {
				return nil, fmt.Errorf("failed to normalize macro payload: %w", err)
			}
			funcs := buildFuncMap(ctx.Input, strings.TrimSpace(ctx.Input.DefaultLocale), locale)
			rendered, err := renderI18nTemplateNode(i18nData, funcs, map[string]any{
				"Source":  ctx.Source,
				"Payload": macroPayload,
				"locale":  locale,
			})
			if err != nil {
				details := formatI18nMacroErrorDetails(err, ctx.Input)
				return nil, fmt.Errorf("%w in section %q: %s", errI18nMacroExpansion, section.Template, details)
			}
			renderedMap, ok := rendered.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected rendered i18n payload type %T", rendered)
			}
			i18nData = renderedMap
		}
		payload["i18n"] = i18nData
	}

	return payload, nil
}

func buildImplicitPagePayload(ctx transformContext) map[string]any {
	if ctx.Layout != nil {
		return ctx.Layout.PageTemplateData(ctx.Page, ctx.Total)
	}
	orientation := strings.ToLower(strings.TrimSpace(ctx.Input.PageOrientation))
	if orientation == "" {
		orientation = PageOrientationPortrait
	}
	return map[string]any{
		"pageNumber":      ctx.Page,
		"pageNumberTotal": ctx.Total,
		"width":           ctx.Input.PageWidth,
		"height":          ctx.Input.PageHeight,
		"orientation":     orientation,
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
}

func renderI18nTemplateNode(node any, funcs htmltmpl.FuncMap, data any) (any, error) {
	return renderI18nTemplateNodeAtPath(node, funcs, data, "root")
}

func formatI18nMacroErrorDetails(err error, input RenderInput) string {
	base := err.Error()
	var valueErr *i18nTemplateValueError
	if !errors.As(err, &valueErr) {
		return base
	}

	filePath := resolveI18nSourceFilePath(input)
	if strings.TrimSpace(filePath) == "" {
		return base
	}

	annotation, ok := annotateI18nErrorLine(filePath, valueErr)
	if !ok {
		return base
	}

	return base + "\n" + annotation
}

func resolveI18nSourceFilePath(input RenderInput) string {
	if filePath := strings.TrimSpace(input.I18nSource.FilePath); filePath != "" {
		return filePath
	}
	if input.I18nSource.IsSet() {
		return ""
	}
	baseDir := strings.TrimSpace(input.AssetBaseDir)
	if baseDir == "" {
		return ""
	}
	candidate := filepath.Join(baseDir, "i18n.json")
	if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
		return candidate
	}
	return ""
}

func annotateI18nErrorLine(filePath string, valueErr *i18nTemplateValueError) (string, bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(content), "\n")
	token := extractI18nErrorToken(valueErr.Err)
	lineNum, lineText := findLineByValue(lines, valueErr.Value)
	if lineNum == 0 {
		if token == "" {
			return "", false
		}
		lineNum, lineText = findLineByValue(lines, token)
		if lineNum == 0 {
			return "", false
		}
	}

	highlightedLine, markerLine := highlightMacroInLine(lineText, token)
	if markerLine == "" {
		idx := strings.Index(lineText, valueErr.Value)
		if idx < 0 {
			idx = 0
		}
		markerLine = "  " + strings.Repeat(" ", idx) + "^"
	}

	return fmt.Sprintf("%s:%d\n%s\n%s", filePath, lineNum, highlightedLine, markerLine), true
}

func findLineByValue(lines []string, value string) (int, string) {
	needle := strings.TrimSpace(value)
	if needle == "" {
		return 0, ""
	}
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i + 1, line
		}
	}
	return 0, ""
}

func extractUndefinedMacroToken(err error) string {
	if err == nil {
		return ""
	}
	matches := undefinedTemplateFunctionPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}
	return "{{" + matches[1] + "}}"
}

func extractI18nErrorToken(err error) string {
	if token := extractUndefinedMacroToken(err); token != "" {
		return token
	}
	if err == nil {
		return ""
	}
	matches := templateExecutionTokenPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}
	expr := strings.TrimSpace(matches[1])
	if expr == "" {
		return ""
	}
	if strings.HasPrefix(expr, "{{") {
		return expr
	}
	return "{{" + expr + "}}"
}

func highlightMacroInLine(line, macro string) (string, string) {
	const (
		reset = "\x1b[0m"
		cyan  = "\x1b[36m"
		green = "\x1b[32m"
		red   = "\x1b[31;1m"
	)

	colored := line
	if key, rest, ok := splitJSONLine(line); ok {
		colored = cyan + key + reset + rest
		colon := strings.Index(rest, ":")
		if colon >= 0 {
			prefix := rest[:colon+1]
			value := rest[colon+1:]
			trimmed := strings.TrimSpace(value)
			if strings.HasPrefix(trimmed, "\"") {
				colored = cyan + key + reset + prefix + green + value + reset
			}
		}
	}

	if strings.TrimSpace(macro) == "" {
		return "  " + colored, ""
	}
	idx := strings.Index(line, macro)
	if idx < 0 {
		return "  " + colored, ""
	}
	colored = strings.Replace(colored, macro, red+macro+reset, 1)
	marker := "  " + strings.Repeat(" ", idx) + red + strings.Repeat("^", len(macro)) + reset
	return "  " + colored, marker
}

func splitJSONLine(line string) (string, string, bool) {
	trimmedLeft := strings.TrimLeft(line, " \t")
	indentLen := len(line) - len(trimmedLeft)
	if !strings.HasPrefix(trimmedLeft, "\"") {
		return "", "", false
	}
	end := strings.Index(trimmedLeft[1:], "\"")
	if end < 0 {
		return "", "", false
	}
	end++
	key := line[:indentLen] + trimmedLeft[:end+1]
	rest := trimmedLeft[end+1:]
	return key, rest, true
}

func renderI18nTemplateNodeAtPath(node any, funcs htmltmpl.FuncMap, data any, path string) (any, error) {
	switch typed := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			childPath := path + "." + key
			rendered, err := renderI18nTemplateNodeAtPath(value, funcs, data, childPath)
			if err != nil {
				return nil, err
			}
			out[key] = rendered
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			rendered, err := renderI18nTemplateNodeAtPath(value, funcs, data, childPath)
			if err != nil {
				return nil, err
			}
			out[i] = rendered
		}
		return out, nil
	case string:
		tmpl := htmltmpl.New("i18n-value").Option("missingkey=error")
		if len(funcs) > 0 {
			tmpl = tmpl.Funcs(funcs)
		}
		parsed, err := tmpl.Parse(typed)
		if err != nil {
			return nil, &i18nTemplateValueError{
				Path:  path,
				Value: typed,
				Err:   fmt.Errorf("failed to parse i18n template value %q: %w", typed, err),
			}
		}
		var buf bytes.Buffer
		if err := parsed.Execute(&buf, data); err != nil {
			return nil, &i18nTemplateValueError{
				Path:  path,
				Value: typed,
				Err:   fmt.Errorf("failed to execute i18n template value %q: %w", typed, err),
			}
		}
		return buf.String(), nil
	default:
		return typed, nil
	}
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
	if err := decodeJSON(buf, &out, false); err != nil {
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
