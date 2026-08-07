package docflowpdf

import (
	htmltmpl "html/template"
	"strconv"
	"strings"
	"time"

	"github.com/goodsign/monday"
)

type currencyMeta struct {
	Code   string
	Symbol string
	Name   string
}

var currencyMetaByCode = map[string]currencyMeta{
	"AUD": {Code: "AUD", Symbol: "$", Name: "Australian Dollar"},
	"CAD": {Code: "CAD", Symbol: "$", Name: "Canadian Dollar"},
	"CHF": {Code: "CHF", Symbol: "CHF", Name: "Swiss Franc"},
	"CNY": {Code: "CNY", Symbol: "\u00a5", Name: "Chinese Yuan"},
	"EUR": {Code: "EUR", Symbol: "\u20ac", Name: "Euro"},
	"GBP": {Code: "GBP", Symbol: "\u00a3", Name: "British Pound"},
	"JPY": {Code: "JPY", Symbol: "\u00a5", Name: "Japanese Yen"},
	"NOK": {Code: "NOK", Symbol: "kr", Name: "Norwegian Krone"},
	"SEK": {Code: "SEK", Symbol: "kr", Name: "Swedish Krona"},
	"USD": {Code: "USD", Symbol: "$", Name: "US Dollar"},
}

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

	return htmltmpl.FuncMap{
		"now": func() time.Time {
			return nowFn()
		},
		"formatLocalizedDate": func(value any, style string, localeOverride ...string) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				return ""
			}
			locale := resolveTemplateLocale(localeOverride, ctx.PayloadLocale, ctx.DefaultLocale)
			return FormatLocalizedTime(t, locale, style)
		},
		"formatLocalizedDateOrNow": func(value any, style string, localeOverride ...string) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				t = nowFn()
			}
			locale := resolveTemplateLocale(localeOverride, ctx.PayloadLocale, ctx.DefaultLocale)
			return FormatLocalizedTime(t, locale, style)
		},
		"formatDate": func(value any, localeOverride ...string) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				return ""
			}
			return formatTemplateDate(t, resolveTemplateLocale(localeOverride, ctx.PayloadLocale, ctx.DefaultLocale))
		},
		"formatDateTime": func(value any, localeOverride ...string) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				return ""
			}
			return formatTemplateDateTime(t, resolveTemplateLocale(localeOverride, ctx.PayloadLocale, ctx.DefaultLocale))
		},
		"dateLocalizedOrNow": func(value any, localeOverride ...string) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				t = nowFn()
			}
			return formatTemplateDate(t, resolveTemplateLocale(localeOverride, ctx.PayloadLocale, ctx.DefaultLocale))
		},
		"firstDate": func(values ...any) any {
			for _, value := range values {
				if _, ok := parseTemplateTime(value); ok {
					return value
				}
			}
			return ""
		},
		"workWeek": func(value any) string {
			t, ok := parseTemplateTime(value)
			if !ok {
				return ""
			}
			_, week := t.ISOWeek()
			weekday := int(t.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			return strings.TrimSpace(strings.Join([]string{strconv.Itoa(week), strconv.Itoa(weekday)}, "."))
		},
		"sumNumbers": func(rows any, key string) float64 {
			items, ok := rows.([]any)
			if !ok {
				return 0
			}
			total := 0.0
			for _, item := range items {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				total += asFloat64(obj[key])
			}
			return total
		},
		"currency": func(attribute ...string) string {
			if len(attribute) == 0 {
				return currency.Code
			}
			switch strings.ToLower(strings.TrimSpace(attribute[0])) {
			case "", "code", "iso", "iso4217", "currency", "id":
				return currency.Code
			case "symbol", "sign", "glyph":
				return currency.Symbol
			case "name", "label", "display", "pretty":
				return currency.Name
			default:
				return currency.Code
			}
		},
		"currencyCode":   func() string { return currency.Code },
		"currencySymbol": func() string { return currency.Symbol },
		"currencyName":   func() string { return currency.Name },
	}
}

func resolveCurrencyMeta(code string) currencyMeta {
	resolvedCode := strings.ToUpper(strings.TrimSpace(code))
	if resolvedCode == "" {
		resolvedCode = DefaultCurrencyCode
	}
	if meta, ok := currencyMetaByCode[resolvedCode]; ok {
		return meta
	}
	return currencyMeta{Code: resolvedCode, Symbol: resolvedCode, Name: resolvedCode}
}

func resolveTemplateLocale(overrides []string, payloadLocale, fallback string) string {
	if len(overrides) > 0 {
		if locale := strings.TrimSpace(overrides[0]); locale != "" {
			return locale
		}
	}
	if locale := strings.TrimSpace(payloadLocale); locale != "" {
		return locale
	}
	if locale := strings.TrimSpace(fallback); locale != "" {
		return locale
	}
	return "en"
}

func parseTemplateTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01",
			"2006-01-02",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
		} {
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func mondayLocale(locale string) monday.Locale {
	switch locale {
	case "de":
		return monday.LocaleDeDE
	case "en":
		return monday.LocaleEnUS
	default:
		return monday.LocaleEnUS
	}
}

func formatTemplateDate(t time.Time, locale string) string {
	loc := mondayLocale(locale)
	if locale == "de" {
		return monday.Format(t, "2. January 2006", loc)
	}
	return monday.Format(t, "01/02/2006", loc)
}

func formatTemplateDateTime(t time.Time, locale string) string {
	loc := mondayLocale(locale)
	if locale == "de" {
		return monday.Format(t, "02. January 2006 15:04", loc)
	}
	return monday.Format(t, "01/02/2006 15:04", loc)
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

func asFloat64(v any) float64 {
	switch typed := v.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}
