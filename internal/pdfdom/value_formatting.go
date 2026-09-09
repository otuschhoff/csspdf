package pdfdom

import "fmt"

// Format returns the locale-formatted currency string.
func (e *ElemCurrencyValue) Format(formatter ValueFormatter) string {
	if formatter == nil {
		return fmt.Sprintf("%.2f", e.Value)
	}
	return formatter.FormatCurrency(e.Value)
}

// Format returns the formatted date, using long format when requested.
func (e *ElemDateValue) Format(formatter ValueFormatter) string {
	if formatter == nil {
		return e.Value
	}
	long, _ := e.Attribute("long")
	return formatter.FormatDate(e.Value, long == "true")
}

// Format returns the duration as "H:MM".
func (e *ElemDurationValue) Format(formatter ValueFormatter) string {
	if formatter == nil {
		return fmt.Sprintf("%g", e.Value)
	}
	return formatter.FormatDuration(e.Value)
}

// Format returns man-days with an optional " PT" suffix.
func (e *ElemManDaysValue) Format(formatter ValueFormatter) string {
	formatted := fmt.Sprintf("%g", e.Value)
	if formatter != nil {
		formatted = formatter.FormatFloat(e.Value, 2)
	}
	unit, _ := e.Attribute("unit")
	if unit == "true" {
		formatted += " PT"
	}
	return formatted
}
