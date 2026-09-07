package docflowpdf

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
	orientation := strings.ToLower(strings.TrimSpace(input.PageOrientation))
	if orientation == "" {
		orientation = PageOrientationPortrait
	}
	if orientation != PageOrientationPortrait && orientation != PageOrientationLandscape {
		return 0, 0, fmt.Errorf("unsupported page orientation %q (expected %q or %q)", input.PageOrientation, PageOrientationPortrait, PageOrientationLandscape)
	}

	width, height, ok := templateload.ResolveNamedPageSize(formatName)
	if !ok {
		return 0, 0, fmt.Errorf("unsupported page format %q", input.PageFormat)
	}
	if orientation == PageOrientationLandscape {
		if height > width {
			width, height = height, width
		}
	} else if width > height {
		width, height = height, width
	}
	if input.PageWidth > 0 {
		width = input.PageWidth
	}
	if input.PageHeight > 0 {
		height = input.PageHeight
	}
	return width, height, nil
}

func validatePageDimensionOverride(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("%s must be finite and zero or greater", name)
	}
	return nil
}
