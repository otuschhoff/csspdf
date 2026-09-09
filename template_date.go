package csspdf

import (
	htmltmpl "html/template"
	"strconv"
	"strings"
	"time"

	"github.com/goodsign/monday"
)

func dateTemplateFuncs(ctx FuncContext, nowFn func() time.Time) htmltmpl.FuncMap {
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
	}
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
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
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
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed, true
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

func formatTemplateDate(value time.Time, locale string) string {
	loc := mondayLocale(locale)
	if locale == "de" {
		return monday.Format(value, "2. January 2006", loc)
	}
	return monday.Format(value, "01/02/2006", loc)
}

func formatTemplateDateTime(value time.Time, locale string) string {
	loc := mondayLocale(locale)
	if locale == "de" {
		return monday.Format(value, "02. January 2006 15:04", loc)
	}
	return monday.Format(value, "01/02/2006 15:04", loc)
}
