package docflowpdf

import (
	"fmt"
	"regexp"
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

type Assets struct {
	HTML       string
	CSS        string
	Flow       Flow
	SourceData map[string]any
}

func (a Assets) Validate() error {
	if a.HTML == "" {
		return fmt.Errorf("template HTML must not be empty")
	}
	if a.CSS == "" {
		return fmt.Errorf("template CSS must not be empty")
	}
	if err := a.Flow.Validate(); err != nil {
		return err
	}
	return nil
}

var payloadPathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

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
	for path := range s.Payload.Runtime {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid runtime target path %q", path)
		}
	}
	for path := range s.Payload.Static {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid static target path %q", path)
		}
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
