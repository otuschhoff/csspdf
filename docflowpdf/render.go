package docflowpdf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	htmltmpl "html/template"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/otuschhoff/csspdf/internal/flowrender"
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
	Path string
	Err  error
}

func (e *i18nTemplateValueError) Error() string {
	return fmt.Sprintf("path %s: %v", e.Path, e.Err)
}

func (e *i18nTemplateValueError) Unwrap() error {
	return e.Err
}

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
	OutputPath string
	// AssetBaseDir is the profile directory. Automatic font discovery checks
	// fonts below this directory, then its parent and grandparent, in that order.
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
	DefaultMargins           PageMargins
	// Now supplies template time and, when non-nil, fixes PDF creation and
	// modification metadata for byte-reproducible rendering.
	Now              func() time.Time
	FuncMapFactoryEx FuncMapFactoryWithContext
	FuncMapFactory   FuncMapFactory
	Logger           Logger
	ResourceResolver ResourceResolver
	Limits           RenderLimits
	// AllowPartialRender preserves the legacy behavior of logging recoverable
	// template and element errors while emitting a potentially incomplete PDF.
	AllowPartialRender bool
	// Deprecated: use Logger. Scheduled for removal in v0.3.0.
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
	return RenderWithInputContext(context.Background(), input)
}

// RenderWithInputContext renders using a fully specified input and context.
func RenderWithInputContext(ctx context.Context, input RenderInput) error {
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	return RenderToFileContext(ctx, input, input.OutputPath)
}

// RenderToWriter renders and writes a PDF to an io.Writer.
func RenderToWriter(input RenderInput, out io.Writer) error {
	return RenderToWriterContext(context.Background(), input, out)
}

// RenderToWriterContext renders a PDF to out with cancellation and budgets.
func RenderToWriterContext(ctx context.Context, input RenderInput, out io.Writer) error {
	if out == nil {
		return fmt.Errorf("output writer is required")
	}
	if ctx == nil {
		return fmt.Errorf("render context is required")
	}
	limits, err := normalizeRenderLimits(input.Limits)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before preparation: %w", err)
	}
	artifact, err := buildArtifactContext(ctx, input, limits)
	if err != nil {
		return diagnostic(diagnosticCode(err, DiagnosticInvalidInput), "preparation", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before emission: %w", err)
	}
	if err := emitArtifactContext(ctx, artifact, out, limits.OutputBytes); err != nil {
		return diagnostic(diagnosticCode(err, DiagnosticOutput), "emission", err)
	}
	return nil
}

func emitArtifactContext(ctx context.Context, artifact *renderArtifact, out io.Writer, outputLimit int64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before emission: %w", err)
	}
	limited := &budgetWriter{writer: out, remaining: outputLimit, limit: outputLimit, stage: "PDF output bytes", ctx: ctx}
	if err := artifact.layout.PDF.Output(limited); err != nil {
		return fmt.Errorf("failed to output PDF: %w", err)
	}
	return nil
}

// RenderToBytes renders and returns a complete PDF byte slice.
func RenderToBytes(input RenderInput) ([]byte, error) {
	return RenderToBytesContext(context.Background(), input)
}

// RenderToBytesContext renders and returns a complete bounded PDF byte slice.
func RenderToBytesContext(ctx context.Context, input RenderInput) ([]byte, error) {
	var buf bytes.Buffer
	if err := RenderToWriterContext(ctx, input, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

func renderMainFlow(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput) error {
	prepared, err := flowrender.PrepareFlow(templateSourcesInRenderOrder(assets), assets.CSS)
	if err != nil {
		return err
	}
	return renderMainFlowPrepared(renderCtx, limits, complexity, prepared, layout, assets, source, input)
}

func renderMainFlowPrepared(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, prepared *flowrender.PreparedFlow, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput) error {
	ctx := transformContext{
		Layout:  layout,
		Source:  source,
		Input:   input,
		Context: renderCtx,
		Limits:  limits,
	}
	for _, section := range assets.Flow.MainFlow {
		if err := renderCtx.Err(); err != nil {
			return fmt.Errorf("render canceled before section %q: %w", section.Template, err)
		}
		payload, err := transformSectionPayload(section, ctx)
		if err != nil {
			var diagnosticErr *DiagnosticError
			if errors.As(err, &diagnosticErr) {
				return err
			}
			return &DiagnosticError{Code: DiagnosticTemplate, Stage: "payload", Section: section.Template, Err: err}
		}
		jsonData, err := asJSONObject(payload)
		if err != nil {
			return &DiagnosticError{Code: DiagnosticTemplate, Stage: "payload", Section: section.Template, Err: err}
		}
		funcs := buildFuncMap(input, strings.TrimSpace(input.DefaultLocale), requiredString(jsonData, "locale"))

		elements, err := prepared.Build(section.Template, jsonData, funcs, flowrender.BuildOptions{
			Context: renderCtx, MaxTemplateOutputBytes: limits.TemplateOutputBytes, MaxNodes: limits.Nodes, MaxDepth: limits.Depth, MaxRows: limits.Rows, Complexity: complexity,
		})
		if err != nil {
			var outputLimit *templateload.OutputLimitError
			if errors.As(err, &outputLimit) {
				return &BudgetError{Stage: "template output bytes", Limit: outputLimit.Limit}
			}
			var complexityLimit *flowrender.ComplexityLimitError
			if errors.As(err, &complexityLimit) {
				return &BudgetError{Stage: complexityLimit.Kind, Limit: int64(complexityLimit.Limit), Actual: int64(complexityLimit.Actual)}
			}
			return &DiagnosticError{Code: diagnosticCode(err, DiagnosticTemplate), Stage: "template", Section: section.Template, Err: err}
		}
		if err := pdfrender.RenderDocTemplateFlow(layout, elements); err != nil {
			var pageLimit *pdfrender.PageLimitError
			if errors.As(err, &pageLimit) {
				return &BudgetError{Stage: "pages", Limit: int64(pageLimit.Limit), Actual: int64(pageLimit.Requested)}
			}
			return &fatalFlowRenderError{err: &DiagnosticError{Code: DiagnosticLayout, Stage: "layout", Section: section.Template, Err: err}}
		}
	}
	return nil
}

func pageNumberTemplateFlowElements(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, page, total int, input RenderInput) ([]pdfdom.PDFElementNode, error) {
	prepared, err := flowrender.PrepareFlow(templateSourcesInRenderOrder(assets), assets.CSS)
	if err != nil {
		return nil, err
	}
	return pageNumberTemplateFlowElementsPrepared(renderCtx, limits, complexity, prepared, layout, assets, source, page, total, input)
}

func pageNumberTemplateFlowElementsPrepared(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, prepared *flowrender.PreparedFlow, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, page, total int, input RenderInput) ([]pdfdom.PDFElementNode, error) {
	payload, err := transformSectionPayload(assets.Flow.PageNumber, transformContext{Layout: layout, Source: source, Page: page, Total: total, Input: input, Context: renderCtx, Limits: limits})
	if err != nil {
		return nil, err
	}
	data, err := asJSONObject(payload)
	if err != nil {
		return nil, err
	}
	funcs := buildFuncMap(input, layout.I18n.Locale(), requiredString(data, "locale"))

	elements, err := prepared.Build(assets.Flow.PageNumber.Template, data, funcs, flowrender.BuildOptions{
		Context: renderCtx, MaxTemplateOutputBytes: limits.TemplateOutputBytes, MaxNodes: limits.Nodes, MaxDepth: limits.Depth, MaxRows: limits.Rows, Complexity: complexity,
	})
	if err != nil {
		var outputLimit *templateload.OutputLimitError
		if errors.As(err, &outputLimit) {
			return nil, &BudgetError{Stage: "template output bytes", Limit: outputLimit.Limit}
		}
		var complexityLimit *flowrender.ComplexityLimitError
		if errors.As(err, &complexityLimit) {
			return nil, &BudgetError{Stage: complexityLimit.Kind, Limit: int64(complexityLimit.Limit), Actual: int64(complexityLimit.Actual)}
		}
		return nil, err
	}
	return elements, nil
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

func asJSONObject(data any) (map[string]any, error) {
	return asJSONObjectWithLimit(data, 0)
}

func asJSONObjectWithLimit(data any, limit int64) (map[string]any, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data to JSON: %w", err)
	}
	if limit > 0 && int64(len(buf)) > limit {
		return nil, &BudgetError{Stage: "source data bytes", Limit: limit, Actual: int64(len(buf))}
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
