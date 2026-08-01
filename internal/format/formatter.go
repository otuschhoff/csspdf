// Package format provides locale-aware formatting for numbers, currency, and dates.
// It depends on internal/i18n but has no PDF/layout dependencies.
package format

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/otuschhoff/go-dom2pdf/internal/i18n"
)

// Formatter handles formatting of numbers, currency, and dates.
type Formatter struct {
	i18n     *i18n.I18n
	currency string
}

var currencySymbolByCode = map[string]string{
	"AUD": "$",
	"CAD": "$",
	"CHF": "CHF",
	"CNY": "\u00a5",
	"EUR": "\u20ac",
	"GBP": "\u00a3",
	"JPY": "\u00a5",
	"NOK": "kr",
	"SEK": "kr",
	"USD": "$",
}

// New creates a new Formatter with the given i18n instance and currency code.
func New(i *i18n.I18n, currency string) *Formatter {
	return &Formatter{
		i18n:     i,
		currency: currency,
	}
}

// FormatCurrency formats a float as currency with proper separators.
func (f *Formatter) FormatCurrency(value float64) string {
	intPart := int64(math.Abs(value))
	fracPart := int64(math.Round((math.Abs(value) - float64(intPart)) * 100))

	intStr := f.formatIntWithSeparator(intPart)

	sign := ""
	if value < 0 {
		sign = "-"
	}

	result := fmt.Sprintf("%s%s%s%02d",
		sign,
		intStr,
		f.i18n.FloatSeparator(),
		fracPart,
	)

	currency := displayCurrency(strings.TrimSpace(f.currency))
	if currency == "" {
		currency = displayCurrency("EUR")
	}

	if f.i18n.Locale() == "de" {
		return result + " " + currency
	}
	return currency + " " + result
}

func displayCurrency(currency string) string {
	if currency == "" {
		return ""
	}
	if symbol, ok := currencySymbolByCode[strings.ToUpper(currency)]; ok {
		return symbol
	}
	return currency
}

// FormatFloat formats a float with the specified number of decimal places.
func (f *Formatter) FormatFloat(value float64, decimals int) string {
	intPart := int64(math.Abs(value))

	multiplier := math.Pow(10, float64(decimals))
	fracPart := int64(math.Round((math.Abs(value) - float64(intPart)) * multiplier))

	intStr := f.formatIntWithSeparator(intPart)

	sign := ""
	if value < 0 {
		sign = "-"
	}

	if decimals == 0 {
		return sign + intStr
	}

	formatStr := fmt.Sprintf("%%s%%s%%s%%0%dd", decimals)
	return fmt.Sprintf(formatStr,
		sign,
		intStr,
		f.i18n.FloatSeparator(),
		fracPart,
	)
}

func (f *Formatter) formatIntWithSeparator(value int64) string {
	str := fmt.Sprintf("%d", value)

	sep := f.i18n.KiloSeparator()
	length := len(str)

	if length <= 3 {
		return str
	}

	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (length-i)%3 == 0 {
			result.WriteString(sep)
		}
		result.WriteRune(digit)
	}

	return result.String()
}

// FormatDate formats an ISO date string to a localized format.
// Input: "2026-03-02"; output depends on locale and longFormat flag.
func (f *Formatter) FormatDate(dateStr string, longFormat bool) string {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}

	if longFormat {
		if f.i18n.Locale() == "de" {
			months := []string{
				"", "Januar", "Februar", "M\u00e4rz", "April", "Mai", "Juni",
				"Juli", "August", "September", "Oktober", "November", "Dezember",
			}
			return fmt.Sprintf("%d. %s %d", date.Day(), months[date.Month()], date.Year())
		}
		return date.Format("January 2, 2006")
	}

	if f.i18n.Locale() == "de" {
		return date.Format("02.01.2006")
	}
	return date.Format("01/02/2006")
}

// FormatDuration formats hours as "HH:MM".
func (f *Formatter) FormatDuration(hours float64) string {
	h := int(hours)
	m := int((hours - float64(h)) * 60)
	return fmt.Sprintf("%d:%02d", h, m)
}

// FormatWorkWeek formats a date as ISO work-week notation "W.D".
func (f *Formatter) FormatWorkWeek(dateStr string) string {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ""
	}

	_, week := date.ISOWeek()
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday is 7 in ISO
	}

	return fmt.Sprintf("%d.%d", week, weekday)
}

// FormatPercent formats a percentage value.
func (f *Formatter) FormatPercent(value float64) string {
	return f.FormatFloat(value, 0) + "%"
}

// ParseDate parses a date string in various formats.
func ParseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"02.01.2006",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
