package pdfrender

import (
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

func TestInterElementSpacing_CollapseUsesMaximum(t *testing.T) {
	got := interElementSpacing(13.4, 10.2, true)
	if got != 13.4 {
		t.Fatalf("expected collapsed spacing to use max margin, got %f", got)
	}
}

func TestInterElementSpacing_AdditiveUsesSum(t *testing.T) {
	got := interElementSpacing(13.4, 10.2, false)
	if got != 23.6 {
		t.Fatalf("expected additive spacing to sum margins, got %f", got)
	}
}

func TestIsVerticalMarginCollapsible(t *testing.T) {
	if !isVerticalMarginCollapsible(pdfdom.NewElemDiv()) {
		t.Fatal("expected div margins to be collapsible")
	}
	if !isVerticalMarginCollapsible(pdfdom.NewElemH1()) {
		t.Fatal("expected h1 margins to be collapsible")
	}
	if !isVerticalMarginCollapsible(pdfdom.NewElemTable()) {
		t.Fatal("expected table margins to be collapsible")
	}
	if isVerticalMarginCollapsible(pdfdom.NewElemImg()) {
		t.Fatal("expected img margins to be non-collapsible")
	}
}

func TestCollapsedSpacingForInternalMarginsUsesContentTop(t *testing.T) {
	currentY := 100.0
	previousBottom := 13.4
	currentTop := 10.0

	contentTop := currentY + interElementSpacing(previousBottom, currentTop, true)
	boxY := contentTop - currentTop

	if contentTop != 113.4 {
		t.Fatalf("unexpected content top: %f", contentTop)
	}
	if boxY != 103.4 {
		t.Fatalf("unexpected box Y for internally-margined element: %f", boxY)
	}
}
