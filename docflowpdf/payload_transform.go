package docflowpdf

import (
	"bytes"
	"context"
	"fmt"
	htmltmpl "html/template"
	"math"
	"strconv"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

type transformContext struct {
	Layout  *pdfrender.LayoutPDF
	Source  map[string]any
	Page    int
	Total   int
	Input   RenderInput
	Context context.Context
	Limits  RenderLimits
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
			rendered, err := renderI18nTemplateNodeWithOptions(i18nData, funcs, map[string]any{
				"Source": ctx.Source, "Payload": macroPayload, "locale": locale,
			}, ctx.Context, ctx.Limits.TemplateOutputBytes)
			if err != nil {
				return nil, &DiagnosticError{Code: diagnosticCode(err, DiagnosticTemplate), Stage: "i18n-template", Section: section.Template, Err: fmt.Errorf("%w: %w", errI18nMacroExpansion, err)}
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
		"pageNumber": ctx.Page, "pageNumberTotal": ctx.Total, "width": ctx.Input.PageWidth,
		"height": ctx.Input.PageHeight, "orientation": orientation, "marginLeft": 0.0,
		"marginRight": 0.0, "marginTop": 0.0, "marginBottom": 0.0, "contentX": 0.0,
		"contentY": 0.0, "contentWidth": 0.0, "contentHeight": 0.0, "contentBottom": 0.0,
	}
}

func renderI18nTemplateNode(node any, funcs htmltmpl.FuncMap, data any) (any, error) {
	return renderI18nTemplateNodeWithOptions(node, funcs, data, context.Background(), 0)
}

func renderI18nTemplateNodeWithOptions(node any, funcs htmltmpl.FuncMap, data any, ctx context.Context, maxBytes int64) (any, error) {
	remaining := maxBytes
	return renderI18nTemplateNodeAtPath(node, funcs, data, "root", ctx, &remaining, maxBytes)
}

func renderI18nTemplateNodeAtPath(node any, funcs htmltmpl.FuncMap, data any, path string, ctx context.Context, remaining *int64, limit int64) (any, error) {
	switch typed := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			rendered, err := renderI18nTemplateNodeAtPath(value, funcs, data, path+"."+key, ctx, remaining, limit)
			if err != nil {
				return nil, err
			}
			out[key] = rendered
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for index, value := range typed {
			rendered, err := renderI18nTemplateNodeAtPath(value, funcs, data, fmt.Sprintf("%s[%d]", path, index), ctx, remaining, limit)
			if err != nil {
				return nil, err
			}
			out[index] = rendered
		}
		return out, nil
	case string:
		tmpl := htmltmpl.New("i18n-value").Option("missingkey=error")
		if len(funcs) > 0 {
			tmpl = tmpl.Funcs(funcs)
		}
		parsed, err := tmpl.Parse(typed)
		if err != nil {
			return nil, &i18nTemplateValueError{Path: path, Err: fmt.Errorf("failed to parse i18n template value: %w", err)}
		}
		var buf bytes.Buffer
		writer := &budgetWriter{writer: &buf, remaining: *remaining, limit: limit, stage: "i18n template output bytes", ctx: ctx}
		if limit <= 0 {
			writer.remaining = math.MaxInt64
		}
		if err := parsed.Execute(writer, data); err != nil {
			return nil, &i18nTemplateValueError{Path: path, Err: fmt.Errorf("failed to execute i18n template value: %w", err)}
		}
		if limit > 0 {
			*remaining = writer.remaining
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
		for _, part := range strings.Split(strings.TrimPrefix(expr, "flow.remainingWidth:"), ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid remaining width part %q in %q: %w", value, expr, err)
			}
			width -= number
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
	current := target
	for index, part := range parts {
		if index == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
}

func getValueAtPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func getStringAtPath(root map[string]any, path string) (string, bool) {
	value, ok := getValueAtPath(root, path)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	return text, ok && text != ""
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
