package pdfrender

import (
	"fmt"
	"math"

	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func validatePageSettings(name string, settings templateload.PageSettings) error {
	values := []struct {
		name  string
		value float64
	}{
		{"width", settings.Width},
		{"height", settings.Height},
		{"top margin", settings.Margins.Top},
		{"right margin", settings.Margins.Right},
		{"bottom margin", settings.Margins.Bottom},
		{"left margin", settings.Margins.Left},
	}
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) {
			return fmt.Errorf("%s %s must be finite", name, item.name)
		}
		if item.value < 0 {
			return fmt.Errorf("%s %s must be zero or greater", name, item.name)
		}
	}
	if settings.Width <= 0 || settings.Height <= 0 {
		return fmt.Errorf("%s dimensions must be greater than zero", name)
	}
	contentWidth := settings.Width - settings.Margins.Left - settings.Margins.Right
	contentHeight := settings.Height - settings.Margins.Top - settings.Margins.Bottom
	if contentWidth <= 0 || contentHeight <= 0 {
		return fmt.Errorf("%s margins leave a non-positive content box (width %.2f, height %.2f)", name, contentWidth, contentHeight)
	}
	return nil
}

func validateAbsoluteElementBounds(kind string, x, y, width, height, pageWidth, pageHeight float64) error {
	values := []float64{x, y, width, height, pageWidth, pageHeight}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("absolute %s has non-finite bounds", kind)
		}
	}
	if x < 0 || y < 0 || width < 0 || height < 0 || x+width > pageWidth || y+height > pageHeight {
		return fmt.Errorf("absolute %s bounds (x %.2f, y %.2f, width %.2f, height %.2f) exceed page %.2fx%.2f", kind, x, y, width, height, pageWidth, pageHeight)
	}
	return nil
}

func validateWholeBlockHeight(kind string, height, available float64) error {
	if math.IsNaN(height) || math.IsInf(height, 0) || height < 0 {
		return fmt.Errorf("%s has invalid height %.2f", kind, height)
	}
	if height > available {
		return fmt.Errorf("%s height %.2f exceeds available content height %.2f", kind, height, available)
	}
	return nil
}
