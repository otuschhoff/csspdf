package templating

import (
	"strings"
	"testing"

	css "github.com/aymerick/douceur/css"
	"golang.org/x/net/html"
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

func TestCSSDeclarationToAttrMapsMinimumTableRowHeight(t *testing.T) {
	attr, value, ok := CSSDeclarationToAttr(&css.Declaration{Property: "row-height-min", Value: "10"})
	if !ok || attr != "row-height-min" || value != "10" {
		t.Fatalf("expected row-height-min=10 mapping, got attr=%q value=%q ok=%v", attr, value, ok)
	}
}

func TestStyledFragmentAppliesSelectorsWithoutOverwritingInlineAttributes(t *testing.T) {
	doc, err := ParseStyledFragment(`<div id="card" color="inline"><span class="note"> hello </span></div>`, `
#card { color: css; background-color: #eee; }
.note { font-weight: 700; text-align: RIGHT; }
`)
	if err != nil {
		t.Fatalf("ParseStyledFragment returned error: %v", err)
	}
	card := FindFirstByID(doc, "div", "card")
	if card == nil || AttrVal(card, "color") != "inline" || AttrVal(card, "background-color") != "#eee" {
		t.Fatalf("unexpected card attributes: %#v", card)
	}
	span := FindFirst(card, "span")
	if span == nil || AttrVal(span, "font-weight") != "bold" || AttrVal(span, "align") != "right" {
		t.Fatalf("unexpected span attributes: %#v", span)
	}
	if got := CollectText(span); got != "hello" {
		t.Fatalf("CollectText = %q, want hello", got)
	}
	if children := ElemChildren(card); len(children) != 1 || children[0] != span {
		t.Fatalf("ElemChildren = %#v, want the span", children)
	}
}

func TestPreparedStylesheetAndDOMHelpers(t *testing.T) {
	stylesheet, err := PrepareStylesheet(`div, span { margin-top: 4pt; } ignored { unknown: value; }`)
	if err != nil {
		t.Fatalf("PrepareStylesheet returned error: %v", err)
	}
	doc, err := ParsePreparedStyledFragment(`<div id="outer"><span data-state="old">x</span></div>`, stylesheet)
	if err != nil {
		t.Fatalf("ParsePreparedStyledFragment returned error: %v", err)
	}
	outer := FindFirstByID(doc, "div", "outer")
	span := FindFirst(outer, "span")
	if AttrVal(outer, "margin-top") != "4pt" || AttrVal(span, "margin-top") != "4pt" {
		t.Fatalf("prepared stylesheet was not applied: outer=%#v span=%#v", outer.Attr, span.Attr)
	}
	attrs := CaptureAttrNames(doc)
	if _, ok := attrs[span]["data-state"]; !ok {
		t.Fatalf("CaptureAttrNames omitted data-state: %#v", attrs[span])
	}
	SetOrReplaceAttr(span, "data-state", "new")
	SetOrReplaceAttr(span, "data-extra", "added")
	if AttrVal(span, "data-state") != "new" || AttrVal(span, "data-extra") != "added" || AttrVal(span, "missing") != "" {
		t.Fatalf("unexpected attributes after replacement: %#v", span.Attr)
	}
	if err := ApplyPreparedStylesheet(doc, nil); err != nil {
		t.Fatalf("nil stylesheet returned error: %v", err)
	}
}

func TestApplyCSSDeclarationIgnoresInvalidInputsAndUnsupportedProperties(t *testing.T) {
	node := &html.Node{Type: html.ElementNode, Data: "div"}
	ApplyCSSDeclaration(nil, &css.Declaration{Property: "color", Value: "red"}, nil)
	ApplyCSSDeclaration(node, nil, nil)
	ApplyCSSDeclaration(node, &css.Declaration{Property: "display", Value: "grid"}, nil)
	ApplyCSSDeclaration(node, &css.Declaration{Property: "font-weight", Value: "900"}, nil)
	if len(node.Attr) != 0 {
		t.Fatalf("unsupported declarations changed node: %#v", node.Attr)
	}
	ApplyCSSDeclaration(node, &css.Declaration{Property: "color", Value: "red"}, map[string]struct{}{"font-color": {}})
	if len(node.Attr) != 0 {
		t.Fatalf("inline attribute was overwritten: %#v", node.Attr)
	}
}

func TestInlineTextAndBorderParsing(t *testing.T) {
	textTests := map[string]string{
		"\n alpha\t beta \r": " alpha beta ",
		"   ":                "",
		"plain":              "plain",
	}
	for input, want := range textTests {
		if got := NormaliseInlineTextNode(input); got != want {
			t.Errorf("NormaliseInlineTextNode(%q) = %q, want %q", input, got, want)
		}
	}
	width, style, color := ParseBorderShorthand("2pt DASHED #AABBCC")
	if width != 2 || style != "dashed" || color != "#aabbcc" {
		t.Fatalf("ParseBorderShorthand = %v %q %q", width, style, color)
	}
	width, style, color = ParseBorderShorthand("none")
	if width != 0 || style != "none" || color != "" {
		t.Fatalf("ParseBorderShorthand(none) = %v %q %q", width, style, color)
	}
}

func TestPrepareStylesheetRejectsInvalidCSSAndSelector(t *testing.T) {
	if _, err := PrepareStylesheet("div:unknown( { color: red; }"); err == nil || !strings.Contains(err.Error(), "selector") {
		t.Fatalf("invalid selector error = %v", err)
	}
}

func TestCSSSupportContract(t *testing.T) {
	properties := SupportedCSSProperties()
	for _, want := range []string{"font-weight", "row-height-min", "text-align"} {
		if !containsString(properties, want) {
			t.Errorf("SupportedCSSProperties omitted %q", want)
		}
	}
	diagnostics, err := AnalyzeCSSSupport(`.card { color: red; display: grid; mystery: yes; }`)
	if err != nil {
		t.Fatalf("AnalyzeCSSSupport returned error: %v", err)
	}
	if len(diagnostics) != 2 || diagnostics[0].Code != UnsupportedCSSPropertyCode || diagnostics[0].Selector != ".card" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if got := diagnostics[0].Error(); !strings.Contains(got, "CSS001") || !strings.Contains(got, "display") {
		t.Fatalf("diagnostic Error() = %q", got)
	}
	if diagnostics, err := AnalyzeCSSSupport(" "); err != nil || diagnostics != nil {
		t.Fatalf("blank CSS = %#v, %v", diagnostics, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
