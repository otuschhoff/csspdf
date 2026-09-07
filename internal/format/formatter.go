// Package format provides locale-aware formatting for numbers, currency, and dates.
// It depends on internal/i18n but has no PDF/layout dependencies.
package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/otuschhoff/csspdf/internal/i18n"
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

var currencyMinorUnitsByCode = map[string]int{
	"BHD": 3,
	"JPY": 0,
	"KWD": 3,
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
	currencyCode := strings.ToUpper(strings.TrimSpace(f.currency))
	if currencyCode == "" {
		currencyCode = "EUR"
	}
	decimals := 2
	if configured, ok := currencyMinorUnitsByCode[currencyCode]; ok {
		decimals = configured
	}
	result := f.FormatFloat(value, decimals)
	currency := displayCurrency(currencyCode)
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
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "+Inf"
	}
	if math.IsInf(value, -1) {
		return "-Inf"
	}
	if decimals < 0 {
		decimals = 0
	}

	rounded := strconv.FormatFloat(math.Abs(value), 'f', decimals, 64)
	parts := strings.SplitN(rounded, ".", 2)
	result := f.formatDigitsWithSeparator(parts[0])
	if len(parts) == 2 {
		result += f.i18n.FloatSeparator() + parts[1]
	}
	if value < 0 && !isFormattedZero(parts) {
		result = "-" + result
	}
	return result
}

func (f *Formatter) formatIntWithSeparator(value int64) string {
	return f.formatDigitsWithSeparator(fmt.Sprintf("%d", value))
}

func (f *Formatter) formatDigitsWithSeparator(str string) string {

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

func isFormattedZero(parts []string) bool {
	for _, part := range parts {
		if strings.Trim(part, "0") != "" {
			return false
		}
	}
	return true
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
