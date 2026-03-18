package template

import (
	"testing"

	css "github.com/aymerick/douceur/css"
)

func TestCSSDeclarationToAttr_MapsBreakProperties(t *testing.T) {
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

	attr, value, ok = CSSDeclarationToAttr(&css.Declaration{Property: "white-space", Value: "nowrap"})
	if !ok || attr != "white-space" || value != "nowrap" {
		t.Fatalf("expected white-space=nowrap mapping, got attr=%q value=%q ok=%v", attr, value, ok)
	}
}

func TestParseLengthValue(t *testing.T) {
	if got, ok := ParseLengthValue("12px"); !ok || got != 12 {
		t.Fatalf("expected 12px to parse to 12, got %f (ok=%v)", got, ok)
	}
	if got, ok := ParseLengthValue("7.5pt"); !ok || got != 7.5 {
		t.Fatalf("expected 7.5pt to parse to 7.5, got %f (ok=%v)", got, ok)
	}
	if _, ok := ParseLengthValue("abc"); ok {
		t.Fatalf("expected invalid length parsing to fail")
	}
}
