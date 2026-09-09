package csspdf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	htmltmpl "html/template"
)

// Logger receives non-fatal render warnings.
type Logger interface {
	Warnf(format string, args ...any)
}

// FuncContext provides deterministic context to template function factories.
type FuncContext struct {
	DefaultLocale       string
	PayloadLocale       string
	DefaultCurrencyCode string
	Now                 func() time.Time
}

// FuncMapFactoryWithContext builds template functions using contextual runtime
// values including a clock hook.
type FuncMapFactoryWithContext func(ctx FuncContext) htmltmpl.FuncMap

// FuncMapFactory is the legacy template function factory signature.
type FuncMapFactory func(defaultLocale, payloadLocale string) htmltmpl.FuncMap

// PageMargins defines default page margins in PDF points.
type PageMargins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

type Section struct {
	Template    string        `json:"template"`
	Transformer string        `json:"transformer"`
	Payload     PayloadConfig `json:"payload"`
}

type PayloadConfig struct {
	IncludeSource bool              `json:"includeSource"`
	Runtime       map[string]string `json:"runtime"`
	I18nVars      map[string]string `json:"i18nVars"`
	Static        map[string]any    `json:"static"`
	LocalePath    string            `json:"localePath"`
}

type Flow struct {
	MainFlow   []Section `json:"mainFlow"`
	PageNumber Section   `json:"pageNumber"`
}

// CSSLayer is a resolved stylesheet layer.
// Layers are applied in order and then legacy Assets.CSS is applied last.
type CSSLayer struct {
	Name     string
	CSS      string
	Optional bool
}

// HTMLLayer is a resolved template layer.
// Layers are parsed in order and then legacy Assets.HTML is parsed last,
// allowing legacy HTML to override shared block defaults.
type HTMLLayer struct {
	Name     string
	HTML     string
	Optional bool
}

type Assets struct {
	HTML       string
	HTMLLayers []HTMLLayer
	CSS        string
	CSSLayers  []CSSLayer
	Flow       Flow
	SourceData map[string]any
}

func (a Assets) Validate() error {
	if _, err := effectiveTemplateHTML(a); err != nil {
		return err
	}
	if _, err := effectiveTemplateCSS(a); err != nil {
		return err
	}
	if err := a.Flow.Validate(); err != nil {
		return err
	}
	return nil
}

func effectiveTemplateHTML(assets Assets) (string, error) {
	return composeTemplateHTML(assets.HTMLLayers, assets.HTML)
}

func composeTemplateHTML(layers []HTMLLayer, legacyHTML string) (string, error) {
	var b strings.Builder

	appendPart := func(html string) {
		trimmed := strings.TrimSpace(html)
		if trimmed == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(trimmed)
		if !strings.HasSuffix(trimmed, "\n") {
			b.WriteString("\n")
		}
	}

	for _, layer := range layers {
		appendPart(layer.HTML)
	}
	appendPart(legacyHTML)

	composed := strings.TrimSpace(b.String())
	if composed == "" {
		return "", fmt.Errorf("template HTML must not be empty")
	}
	return composed, nil
}

func effectiveTemplateCSS(assets Assets) (string, error) {
	return composeTemplateCSS(assets.CSSLayers, assets.CSS)
}

func composeTemplateCSS(layers []CSSLayer, legacyCSS string) (string, error) {
	var b strings.Builder

	appendPart := func(css string) {
		trimmed := strings.TrimSpace(css)
		if trimmed == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(trimmed)
		if !strings.HasSuffix(trimmed, "\n") {
			b.WriteString("\n")
		}
	}

	for _, layer := range layers {
		appendPart(layer.CSS)
	}
	appendPart(legacyCSS)

	composed := strings.TrimSpace(b.String())
	if composed == "" {
		return "", fmt.Errorf("template CSS must not be empty")
	}
	return composed, nil
}

var payloadPathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

var reservedPayloadRoots = map[string]struct{}{
	"Payload": {},
	"Source":  {},
	"i18n":    {},
	"locale":  {},
	"page":    {},
}

func (f Flow) Validate() error {
	if len(f.MainFlow) == 0 {
		return fmt.Errorf("flow must include at least one mainFlow section")
	}
	if f.PageNumber.Template == "" || f.PageNumber.Transformer == "" {
		return fmt.Errorf("flow must define pageNumber template and transformer")
	}
	for i, section := range f.MainFlow {
		if section.Template == "" || section.Transformer == "" {
			return fmt.Errorf("every mainFlow section must define template and transformer")
		}
		if err := section.Validate(); err != nil {
			return fmt.Errorf("mainFlow[%d] (%s): %w", i, section.Template, err)
		}
	}
	if err := f.PageNumber.Validate(); err != nil {
		return fmt.Errorf("pageNumber (%s): %w", f.PageNumber.Template, err)
	}
	return nil
}

func (s Section) Validate() error {
	if s.Template == "" {
		return fmt.Errorf("template must not be empty")
	}
	if s.Transformer != "generic" {
		return fmt.Errorf("unsupported transformer %q", s.Transformer)
	}
	if err := validatePayloadPathMap(s.Payload.Runtime, "runtime"); err != nil {
		return err
	}
	if err := validatePayloadPathMap(s.Payload.Static, "static"); err != nil {
		return err
	}
	if err := validatePayloadTargetPaths(s.Payload.Runtime, s.Payload.Static); err != nil {
		return err
	}
	for name, path := range s.Payload.I18nVars {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid i18n var path %q for key %q", path, name)
		}
	}
	if s.Payload.LocalePath != "" && !payloadPathPattern.MatchString(s.Payload.LocalePath) {
		return fmt.Errorf("invalid localePath %q", s.Payload.LocalePath)
	}
	for _, expr := range s.Payload.Runtime {
		if err := validateRuntimeExpression(expr); err != nil {
			return err
		}
	}
	return nil
}

func validatePayloadPathMap[T any](values map[string]T, kind string) error {
	for path := range values {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid %s target path %q", kind, path)
		}
	}
	return nil
}

func validatePayloadTargetPaths(runtime map[string]string, static map[string]any) error {
	paths := make([]string, 0, len(runtime)+len(static))
	owners := make(map[string]string, len(runtime)+len(static))
	for path := range runtime {
		paths = append(paths, path)
		owners[path] = "runtime"
	}
	for path := range static {
		if owner, exists := owners[path]; exists {
			return fmt.Errorf("payload target path %q is defined by both %s and static values", path, owner)
		}
		paths = append(paths, path)
		owners[path] = "static"
	}
	sort.Strings(paths)
	for index, path := range paths {
		root := strings.SplitN(path, ".", 2)[0]
		if _, reserved := reservedPayloadRoots[root]; reserved {
			return fmt.Errorf("payload target path %q uses reserved root %q", path, root)
		}
		if index > 0 && strings.HasPrefix(path, paths[index-1]+".") {
			return fmt.Errorf("payload target paths %q and %q conflict", paths[index-1], path)
		}
	}
	return nil
}

func cloneFlow(flow Flow) Flow {
	return Flow{
		MainFlow:   cloneSections(flow.MainFlow),
		PageNumber: cloneSection(flow.PageNumber),
	}
}

func cloneSections(sections []Section) []Section {
	if sections == nil {
		return nil
	}
	out := make([]Section, len(sections))
	for index, section := range sections {
		out[index] = cloneSection(section)
	}
	return out
}

func cloneSection(section Section) Section {
	section.Payload.Runtime = cloneStringMap(section.Payload.Runtime)
	section.Payload.I18nVars = cloneStringMap(section.Payload.I18nVars)
	static := section.Payload.Static
	if static != nil {
		section.Payload.Static = make(map[string]any, len(static))
		for key, value := range static {
			section.Payload.Static[key] = cloneJSONValue(value)
		}
	}
	return section
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = cloneJSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			out[index] = cloneJSONValue(child)
		}
		return out
	default:
		return value
	}
}

func validateRuntimeExpression(expr string) error {
	switch {
	case expr == "flow.tableWidth":
		return nil
	case strings.HasPrefix(expr, "flow.remainingWidth:"):
		spec := strings.TrimPrefix(expr, "flow.remainingWidth:")
		if strings.TrimSpace(spec) == "" {
			return fmt.Errorf("invalid runtime expression %q: missing width offsets", expr)
		}
		for _, part := range strings.Split(spec, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				return fmt.Errorf("invalid runtime expression %q: empty width offset", expr)
			}
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return fmt.Errorf("invalid runtime expression %q: %w", expr, err)
			}
		}
		return nil
	case expr == "page.number":
		return nil
	case expr == "page.total":
		return nil
	default:
		return fmt.Errorf("unsupported runtime expression %q", expr)
	}
}
