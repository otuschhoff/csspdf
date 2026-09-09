package csspdf

import (
	htmltmpl "html/template"
	"strings"
	"time"

	"github.com/goodsign/monday"
)

// DefaultTemplateFuncMap returns a reusable set of generic template helper
// functions that can be embedded into profile-specific function maps.
func DefaultTemplateFuncMap(defaultLocale, payloadLocale string) htmltmpl.FuncMap {
	return DefaultTemplateFuncMapWithContext(FuncContext{
		DefaultLocale:       defaultLocale,
		PayloadLocale:       payloadLocale,
		DefaultCurrencyCode: DefaultCurrencyCode,
		Now:                 time.Now,
	})
}

// DefaultTemplateFuncMapWithContext returns the same generic helper set as
// DefaultTemplateFuncMap but uses FuncContext to support deterministic clocks.
func DefaultTemplateFuncMapWithContext(ctx FuncContext) htmltmpl.FuncMap {
	nowFn := ctx.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	currency := resolveCurrencyMeta(ctx.DefaultCurrencyCode)
	funcs := dateTemplateFuncs(ctx, nowFn)
	for name, function := range aggregateTemplateFuncs() {
		funcs[name] = function
	}
	for name, function := range currencyTemplateFuncs(currency) {
		funcs[name] = function
	}
	return funcs
}

// FormatLocalizedTime formats a time value using locale-aware styles.
//
// Supported styles:
//   - "written-month" (or "month", "month-name"): month name only
//   - "local" (or "locale", "default"): locale-default full date
//   - "short": short numeric date format
//   - "datetime": locale-default date + time
//   - "layout:<go time layout>": custom Go layout using localized month/day names
func FormatLocalizedTime(t time.Time, locale, style string) string {
	loc := mondayLocale(locale)
	resolvedStyle := strings.ToLower(strings.TrimSpace(style))

	if strings.HasPrefix(resolvedStyle, "layout:") {
		layout := strings.TrimSpace(strings.TrimPrefix(style, "layout:"))
		if layout == "" {
			return ""
		}
		return monday.Format(t, layout, loc)
	}

	switch resolvedStyle {
	case "written-month", "month", "month-name":
		return monday.Format(t, "January", loc)
	case "short":
		if locale == "de" {
			return monday.Format(t, "02.01.2006", loc)
		}
		return monday.Format(t, "01/02/2006", loc)
	case "datetime":
		if locale == "de" {
			return monday.Format(t, "02. January 2006 15:04", loc)
		}
		return monday.Format(t, "January 2 2006 15:04", loc)
	case "", "local", "locale", "default":
		if locale == "de" {
			return monday.Format(t, "02. January 2006", loc)
		}
		return monday.Format(t, "January 2 2006", loc)
	default:
		if locale == "de" {
			return monday.Format(t, "02. January 2006", loc)
		}
		return monday.Format(t, "January 2 2006", loc)
	}
}
