package csspdf

import (
	"fmt"
	"math"
	"strings"

	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func resolvePageDimensions(input RenderInput) (float64, float64, error) {
	if err := validatePageDimensionOverride("pageWidth", input.PageWidth); err != nil {
		return 0, 0, err
	}
	if err := validatePageDimensionOverride("pageHeight", input.PageHeight); err != nil {
		return 0, 0, err
	}

	formatName := strings.TrimSpace(input.PageFormat)
	if formatName == "" {
		formatName = DefaultPageFormat
	}
	orientation, err := normalizedPageOrientation(input.PageOrientation)
	if err != nil {
		return 0, 0, err
	}

	width, height, ok := templateload.ResolveNamedPageSize(formatName)
	if !ok {
		return 0, 0, fmt.Errorf("unsupported page format %q", input.PageFormat)
	}
	width, height = orientPageDimensions(width, height, orientation)
	if input.PageWidth > 0 {
		width = input.PageWidth
	}
	if input.PageHeight > 0 {
		height = input.PageHeight
	}
	return width, height, nil
}

func normalizedPageOrientation(raw string) (string, error) {
	orientation := strings.ToLower(strings.TrimSpace(raw))
	if orientation == "" {
		return PageOrientationPortrait, nil
	}
	if orientation != PageOrientationPortrait && orientation != PageOrientationLandscape {
		return "", fmt.Errorf("unsupported page orientation %q (expected %q or %q)", raw, PageOrientationPortrait, PageOrientationLandscape)
	}
	return orientation, nil
}

func orientPageDimensions(width, height float64, orientation string) (float64, float64) {
	landscapeSwap := orientation == PageOrientationLandscape && height > width
	portraitSwap := orientation == PageOrientationPortrait && width > height
	if landscapeSwap || portraitSwap {
		return height, width
	}
	return width, height
}

func validatePageDimensionOverride(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("%s must be finite and zero or greater", name)
	}
	return nil
}
