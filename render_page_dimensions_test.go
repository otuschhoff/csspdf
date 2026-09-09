package csspdf

import (
	"math"
	"strings"
	"testing"
)

func TestResolvePageDimensions_DefaultsToA4Portrait(t *testing.T) {
	width, height, err := resolvePageDimensions(RenderInput{})
	if err != nil {
		t.Fatalf("resolvePageDimensions returned error: %v", err)
	}
	if width != 595.28 || height != 841.89 {
		t.Fatalf("unexpected A4 portrait defaults: got %fx%f", width, height)
	}
}

func TestResolvePageDimensions_NamedFormatLandscape(t *testing.T) {
	width, height, err := resolvePageDimensions(RenderInput{
		PageFormat:      "A5",
		PageOrientation: "landscape",
	})
	if err != nil {
		t.Fatalf("resolvePageDimensions returned error: %v", err)
	}
	if width != 595.28 || height != 419.53 {
		t.Fatalf("unexpected A5 landscape dimensions: got %fx%f", width, height)
	}
}

func TestResolvePageDimensions_InvalidFormatOrOrientation(t *testing.T) {
	if _, _, err := resolvePageDimensions(RenderInput{PageFormat: "bogus"}); err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if _, _, err := resolvePageDimensions(RenderInput{PageOrientation: "sideways"}); err == nil {
		t.Fatalf("expected error for unsupported orientation")
	}
}

func TestResolvePageDimensions_RejectsNonFiniteOverrides(t *testing.T) {
	testCases := []struct {
		name  string
		input RenderInput
	}{
		{name: "NaN width", input: RenderInput{PageWidth: math.NaN()}},
		{name: "positive infinite width", input: RenderInput{PageWidth: math.Inf(1)}},
		{name: "negative infinite height", input: RenderInput{PageHeight: math.Inf(-1)}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := resolvePageDimensions(testCase.input); err == nil || !strings.Contains(err.Error(), "finite") {
				t.Fatalf("expected finite-dimension error, got %v", err)
			}
		})
	}
}
