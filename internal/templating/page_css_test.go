package templating

import (
	"testing"

	css "github.com/aymerick/douceur/css"
)

func testDefaultPageSettings() PageSettings {
	return PageSettings{
		Width:  595.28,
		Height: 841.89,
		Margins: PageMargins{
			Top:    30,
			Right:  55,
			Bottom: 30,
			Left:   55,
		},
	}
}

func TestPageCSS_CSSDeclarationToAttr_MapsBreakProperties(t *testing.T) {
	attr, value, ok := CSSDeclarationToAttr(&css.Declaration{Property: "break-before", Value: "page"})
	if !ok || attr != "break-before" || value != "page" {
		t.Fatalf("expected break-before=page mapping, got attr=%q value=%q ok=%v", attr, value, ok)
	}

	attr, value, ok = CSSDeclarationToAttr(&css.Declaration{Property: "break-after", Value: "page"})
	if !ok || attr != "break-after" || value != "page" {
		t.Fatalf("expected break-after=page mapping, got attr=%q value=%q ok=%v", attr, value, ok)
	}

	attr, value, ok = CSSDeclarationToAttr(&css.Declaration{Property: "fill", Value: "#DDDDDD"})
	if !ok || attr != "fill" || value != "#DDDDDD" {
		t.Fatalf("expected fill mapping, got attr=%q value=%q ok=%v", attr, value, ok)
	}
}

func TestParseCSSPageSettings_Defaults(t *testing.T) {
	defaultPage, firstPage, err := ParseCSSPageSettings("", testDefaultPageSettings(), ParseLengthValue)
	if err != nil {
		t.Fatalf("ParseCSSPageSettings returned error: %v", err)
	}

	if defaultPage.Width != 595.28 || defaultPage.Height != 841.89 {
		t.Fatalf("expected default A4 portrait size, got %fx%f", defaultPage.Width, defaultPage.Height)
	}
	if defaultPage.Margins.Left != 55 || defaultPage.Margins.Right != 55 || defaultPage.Margins.Top != 30 || defaultPage.Margins.Bottom != 30 {
		t.Fatalf("unexpected default margins: %+v", defaultPage.Margins)
	}
	if firstPage != defaultPage {
		t.Fatalf("expected first page defaults to match default page, got default=%+v first=%+v", defaultPage, firstPage)
	}
}

func TestParseCSSPageSettings_PageAndFirstOverride(t *testing.T) {
	defaultPage, firstPage, err := ParseCSSPageSettings(`
@page {
	size: A4 landscape;
	margin: 40 50;
}

@page :first {
	margin-top: 300;
	margin-left: 60;
}
`, testDefaultPageSettings(), ParseLengthValue)
	if err != nil {
		t.Fatalf("ParseCSSPageSettings returned error: %v", err)
	}

	if defaultPage.Width != 841.89 || defaultPage.Height != 595.28 {
		t.Fatalf("expected landscape A4 size, got %fx%f", defaultPage.Width, defaultPage.Height)
	}
	if defaultPage.Margins.Top != 40 || defaultPage.Margins.Right != 50 || defaultPage.Margins.Bottom != 40 || defaultPage.Margins.Left != 50 {
		t.Fatalf("unexpected default margins: %+v", defaultPage.Margins)
	}
	if firstPage.Width != defaultPage.Width || firstPage.Height != defaultPage.Height {
		t.Fatalf("expected first page size to inherit default size, got %fx%f", firstPage.Width, firstPage.Height)
	}
	if firstPage.Margins.Top != 300 || firstPage.Margins.Left != 60 || firstPage.Margins.Right != 50 || firstPage.Margins.Bottom != 40 {
		t.Fatalf("unexpected first-page margins: %+v", firstPage.Margins)
	}
}

func TestParseCSSPageSettings_NamedSizeLetter(t *testing.T) {
	defaultPage, firstPage, err := ParseCSSPageSettings(`
@page {
	size: letter;
}
`, testDefaultPageSettings(), ParseLengthValue)
	if err != nil {
		t.Fatalf("ParseCSSPageSettings returned error: %v", err)
	}

	if defaultPage.Width != 612 || defaultPage.Height != 792 {
		t.Fatalf("expected letter portrait size, got %fx%f", defaultPage.Width, defaultPage.Height)
	}
	if firstPage.Width != defaultPage.Width || firstPage.Height != defaultPage.Height {
		t.Fatalf("expected first page size to inherit default size, got %fx%f", firstPage.Width, firstPage.Height)
	}
}

func TestParseCSSPageSettings_NamedSizeLandscapeOrder(t *testing.T) {
	defaultPage, _, err := ParseCSSPageSettings(`
@page {
	size: landscape legal;
}
`, testDefaultPageSettings(), ParseLengthValue)
	if err != nil {
		t.Fatalf("ParseCSSPageSettings returned error: %v", err)
	}

	if defaultPage.Width != 1008 || defaultPage.Height != 612 {
		t.Fatalf("expected legal landscape size, got %fx%f", defaultPage.Width, defaultPage.Height)
	}
}

func TestParseRunningFooterName_MatchingRules(t *testing.T) {
	cssText := `
footer {
	position: running(site-footer);
}

@page {
	@bottom-center {
		content: element(site-footer);
	}
}
`
	name, ok := ParseRunningFooterName(cssText)
	if !ok {
		t.Fatalf("expected running footer name to be detected")
	}
	if name != "site-footer" {
		t.Fatalf("expected site-footer, got %q", name)
	}
}

func TestParseRunningFooterName_MismatchedRules(t *testing.T) {
	cssText := `
footer { position: running(site-footer); }
@page { @bottom-center { content: element(other-footer); } }
`
	if name, ok := ParseRunningFooterName(cssText); ok {
		t.Fatalf("expected no running footer match, got %q", name)
	}
}
