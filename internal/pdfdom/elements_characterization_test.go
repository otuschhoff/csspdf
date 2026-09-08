package pdfdom

import (
	"strings"
	"testing"
)

type invalidPDFNode struct{}

func (*invalidPDFNode) isPDFNode() {}

type characterizationFormatter struct{}

func (characterizationFormatter) FormatCurrency(value float64) string { return "currency" }
func (characterizationFormatter) FormatDate(value string, long bool) string {
	if long {
		return "long:" + value
	}
	return "short:" + value
}
func (characterizationFormatter) FormatDuration(value float64) string { return "duration" }
func (characterizationFormatter) FormatFloat(value float64, decimals int) string {
	return "float"
}

func TestElementConstructorsAndTypes(t *testing.T) {
	elements := []struct {
		want string
		node PDFElementNode
	}{
		{"div", NewElemDiv()}, {"span", NewElemSpan()}, {"br", NewElemBr()},
		{"h1", NewElemH1()}, {"h2", NewElemH2()}, {"h3", NewElemH3()},
		{"ul", NewElemUl()}, {"ol", NewElemOl()}, {"li", NewElemLi()},
		{"img", NewElemImg()}, {"use-template", NewElemUseTemplate()}, {"create-template", NewElemCreateTemplate()},
		{"currency-value", NewElemCurrencyValue(1)}, {"date-value", NewElemDateValue("2026-09-08")},
		{"duration-value", NewElemDurationValue(1)}, {"man-days-value", NewElemManDaysValue(1)},
		{"table", NewElemTable()}, {"colgroup", NewElemColgroup()}, {"col", NewElemCol()},
		{"thead", NewElemThead()}, {"tbody", NewElemTbody()}, {"tr", NewElemTr()},
		{"td", NewElemTd()}, {"th", NewElemTh()},
	}
	for _, test := range elements {
		t.Run(test.want, func(t *testing.T) {
			if got := test.node.ElementType(); got != test.want {
				t.Fatalf("ElementType = %q, want %q", got, test.want)
			}
			if got := test.node.SetAttribute("id", "first").SetAttribute("id", "second"); got != test.node {
				t.Fatal("SetAttribute did not return the owning element")
			}
			if value, ok := test.node.Attribute("id"); !ok || value != "second" {
				t.Fatalf("Attribute(id) = %q, %v", value, ok)
			}
			if _, ok := test.node.Attribute(""); ok {
				t.Fatal("empty attribute name matched")
			}
		})
	}
	for _, heading := range []PDFElementNode{NewElemH1(), NewElemH2(), NewElemH3()} {
		if heading.ElementStyle() == nil || heading.ElementStyle().FontStyle != "B" {
			t.Fatalf("heading %s lacks bold default style: %#v", heading.ElementType(), heading.ElementStyle())
		}
	}
}

func TestElementChildValidationMatrix(t *testing.T) {
	text := &PDFTextNode{Text: "text"}
	tests := []struct {
		name    string
		parent  PDFElementNode
		valid   PDFNode
		invalid PDFNode
	}{
		{"div", NewElemDiv(), text, &invalidPDFNode{}},
		{"span", NewElemSpan(), text, &invalidPDFNode{}},
		{"h1", NewElemH1(), text, &invalidPDFNode{}},
		{"h2", NewElemH2(), text, &invalidPDFNode{}},
		{"h3", NewElemH3(), text, &invalidPDFNode{}},
		{"ul", NewElemUl(), NewElemLi(), text},
		{"ol", NewElemOl(), NewElemLi(), text},
		{"li", NewElemLi(), text, &invalidPDFNode{}},
		{"create-template", NewElemCreateTemplate(), NewElemDiv(), text},
		{"table", NewElemTable(), NewElemTbody(), NewElemDiv()},
		{"colgroup", NewElemColgroup(), NewElemCol(), text},
		{"thead", NewElemThead(), NewElemTr(), text},
		{"tbody", NewElemTbody(), NewElemTr(), text},
		{"tr", NewElemTr(), NewElemTd(), text},
		{"td", NewElemTd(), text, &invalidPDFNode{}},
		{"th", NewElemTh(), text, &invalidPDFNode{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.parent.Add(test.valid); err != nil {
				t.Fatalf("valid child rejected: %v", err)
			}
			if err := test.parent.AddLine(test.valid); err != nil {
				t.Fatalf("valid line child rejected: %v", err)
			}
			if err := test.parent.Add(test.invalid); err == nil {
				t.Fatal("invalid child accepted")
			}
			if len(test.parent.ElementChildren()) != 2 || test.parent.ElementChildLineBreaks()[0] || !test.parent.ElementChildLineBreaks()[1] {
				t.Fatalf("unexpected children/breaks: %#v %#v", test.parent.ElementChildren(), test.parent.ElementChildLineBreaks())
			}
		})
	}
}

func TestLeafElementsRejectChildren(t *testing.T) {
	for _, element := range []PDFElementNode{
		NewElemBr(), NewElemImg(), NewElemUseTemplate(), NewElemCurrencyValue(1),
		NewElemDateValue("2026-09-08"), NewElemDurationValue(1), NewElemManDaysValue(1), NewElemCol(),
	} {
		t.Run(element.ElementType(), func(t *testing.T) {
			if err := element.Add(&PDFTextNode{Text: "invalid"}); err == nil {
				t.Fatal("leaf element accepted a child")
			}
		})
	}
}

func TestBaseElementNilAndEmptyOperations(t *testing.T) {
	var base *baseElementNode
	if got := base.SetAttribute("id", "x"); got != nil {
		t.Fatalf("nil SetAttribute = %#v", got)
	}
	if _, ok := base.Attribute("id"); ok {
		t.Fatal("nil Attribute matched")
	}
	if err := base.Add(&PDFTextNode{}); err != nil || base.AddLine(&PDFTextNode{}) != nil {
		t.Fatal("nil base add returned an error")
	}
	div := NewElemDiv()
	div.SetAttribute("", "ignored")
	if len(div.ElementAttributes()) != 0 {
		t.Fatalf("empty attribute was retained: %#v", div.ElementAttributes())
	}
	if err := div.Add(nil); err != nil || div.AddLine(nil) != nil || len(div.ElementChildren()) != 0 {
		t.Fatal("nil child changed element")
	}
}

func TestValueElementFormatting(t *testing.T) {
	formatter := characterizationFormatter{}
	if got := NewElemCurrencyValue(12.5).Format(nil); got != "12.50" {
		t.Fatalf("nil currency formatter = %q", got)
	}
	if got := NewElemCurrencyValue(12.5).Format(formatter); got != "currency" {
		t.Fatalf("currency formatter = %q", got)
	}
	date := NewElemDateValue("2026-09-08")
	if got := date.Format(nil); got != "2026-09-08" {
		t.Fatalf("nil date formatter = %q", got)
	}
	if got := date.Format(formatter); got != "short:2026-09-08" {
		t.Fatalf("short date formatter = %q", got)
	}
	date.SetAttribute("long", "true")
	if got := date.Format(formatter); got != "long:2026-09-08" {
		t.Fatalf("long date formatter = %q", got)
	}
	if got := NewElemDurationValue(1.5).Format(nil); got != "1.5" {
		t.Fatalf("nil duration formatter = %q", got)
	}
	if got := NewElemDurationValue(1.5).Format(formatter); got != "duration" {
		t.Fatalf("duration formatter = %q", got)
	}
	days := NewElemManDaysValue(2.25)
	if got := days.Format(nil); got != "2.25" {
		t.Fatalf("nil days formatter = %q", got)
	}
	days.SetAttribute("unit", "true")
	if got := days.Format(formatter); got != "float PT" {
		t.Fatalf("days formatter = %q", got)
	}
}

func TestDocumentAndTextNodeChildBreaks(t *testing.T) {
	text := (&PDFTextNode{Text: "root"}).Add(nil).Add(&PDFTextNode{Text: "inline"}).AddLine(&PDFTextNode{Text: "line"})
	if len(text.Children) != 2 || text.ChildLineBreaks[0] || !text.ChildLineBreaks[1] {
		t.Fatalf("unexpected text children: %#v %#v", text.Children, text.ChildLineBreaks)
	}
	document := (&PDFDocumentNode{}).Add(nil).Add(NewElemDiv()).AddLine(NewElemTable())
	if len(document.Children) != 2 || document.ChildLineBreaks[0] || !document.ChildLineBreaks[1] {
		t.Fatalf("unexpected document children: %#v %#v", document.Children, document.ChildLineBreaks)
	}
}

func TestTextStyleMergeAndDefaults(t *testing.T) {
	base := PDFTextStyle{FontFace: "Base", FontStyle: "B", FontStyleSet: true, FontSize: 9, FontColor: "#111", Align: TextAlignRight, LineHeight: 1.1, BackgroundColor: "#eee", BorderColor: "#222", BorderStyle: "solid", BorderWidth: 1}
	if got := base.Merge(nil); got != base {
		t.Fatalf("Merge(nil) = %#v, want %#v", got, base)
	}
	override := &PDFTextStyle{FontFace: "Override", FontStyle: "I", FontStyleSet: true, FontSize: 12, FontColor: "#333", Align: TextAlignCenter, LineHeight: 1.5, BackgroundColor: "#ddd", BorderColor: "#444", BorderStyle: "dashed", BorderWidth: 2}
	if got := base.Merge(override); got != *override {
		t.Fatalf("full override = %#v, want %#v", got, *override)
	}
	defaults := (PDFTextStyle{}).Merge(nil)
	if defaults.FontFace != "Helvetica" || defaults.FontSize != 10 || defaults.FontColor != "#000" || defaults.Align != TextAlignLeft || defaults.LineHeight != 1.2 {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}
}

func TestPreparedParserAndComplexSpanAttributes(t *testing.T) {
	elements, err := ParseHTMLDocFlowPrepared(`<div><span font-face="Courier" font-size="12pt" font-color="#123456" align="CENTER" font-style="italic" font-weight="bold" background-color="#eee" border="2pt dashed #abc">styled</span></div>`, nil)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlowPrepared returned error: %v", err)
	}
	span := elements[0].ElementChildren()[0].(*PDFTextNode)
	if span.Text != "styled" || span.Style == nil || span.Style.FontFace != "Courier" || span.Style.FontSize != 12 || span.Style.Align != TextAlignCenter || span.Style.FontStyle != "IB" || span.Style.BorderWidth != 2 || span.Style.BorderStyle != "dashed" || span.Style.BorderColor != "#abc" {
		t.Fatalf("unexpected span text=%q style=%+v", span.Text, span.Style)
	}
	var warnings []string
	elements, err = ParseHTMLDocFlowPreparedWithOptions(`<p><span border-width="-1">legacy</span></p>`, nil, ParseOptions{AllowInvalidSpanAttributes: true, Warnf: func(format string, args ...any) { warnings = append(warnings, format) }})
	if err != nil || len(elements) != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "ignored") {
		t.Fatalf("legacy parse = %#v warnings=%#v err=%v", elements, warnings, err)
	}
}
