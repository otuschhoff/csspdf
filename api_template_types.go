package csspdf

import (
	htmltmpl "html/template"
	"time"
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
