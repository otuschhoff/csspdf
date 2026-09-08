package pdfdom

import (
	"strings"
	"testing"
)

const numColCSS = `
table thead th.num-col,
table tbody td.num-col {
	text-align: right;
}
`

func TestParseHTMLTableElem_AppliesNumColHeaderAlignment(t *testing.T) {
	table, err := ParseHTMLTableElem(
		"<table><thead><tr><th class=\"num-col\"><span>Amount</span></th></tr></thead></table>",
		numColCSS)
	if err != nil {
		t.Fatalf("ParseHTMLTableElem returned error: %v", err)
	}
	sections := table.ElementChildren()
	if len(sections) != 1 {
		t.Fatalf("expected 1 table section, got %d", len(sections))
	}
	thead, ok := sections[0].(*ElemThead)
	if !ok {
		t.Fatalf("expected thead section, got %T", sections[0])
	}
	rows := thead.ElementChildren()
	if len(rows) != 1 {
		t.Fatalf("expected 1 header row, got %d", len(rows))
	}
	row, ok := rows[0].(*ElemTr)
	if !ok {
		t.Fatalf("expected header row, got %T", rows[0])
	}
	cells := row.ElementChildren()
	if len(cells) != 1 {
		t.Fatalf("expected 1 header cell, got %d", len(cells))
	}
	cell, ok := cells[0].(*ElemTh)
	if !ok {
		t.Fatalf("expected th cell, got %T", cells[0])
	}
	if got, ok := cell.Attribute("align"); !ok || got != "right" {
		t.Fatalf("expected th align=right, got %q (present=%v)", got, ok)
	}
}

func TestParseHTMLTableElem_AppliesWhiteSpaceNoWrapToHeaderCell(t *testing.T) {
	css := `table th.nowrap-col { white-space: nowrap; }`
	table, err := ParseHTMLTableElem(
		"<table><thead><tr><th class=\"nowrap-col\"><span>Date</span></th></tr></thead></table>",
		css,
	)
	if err != nil {
		t.Fatalf("ParseHTMLTableElem returned error: %v", err)
	}
	thead := table.ElementChildren()[0].(*ElemThead)
	row := thead.ElementChildren()[0].(*ElemTr)
	cell := row.ElementChildren()[0].(*ElemTh)
	if got, ok := cell.Attribute("whiteSpace"); !ok || got != "nowrap" {
		t.Fatalf("expected th whiteSpace=nowrap, got %q (present=%v)", got, ok)
	}
}

func TestParseHTMLTableElem_AppliesNthChildNoWrapToLeadingColumns(t *testing.T) {
	css := `
#timesheet-table th:nth-child(-n+4),
#timesheet-table td:nth-child(-n+4) {
	white-space: nowrap;
}`
	html := `<table id="timesheet-table"><thead><tr><th>A</th><th>B</th><th>C</th><th>D</th><th>E</th></tr></thead></table>`
	table, err := ParseHTMLTableElem(html, css)
	if err != nil {
		t.Fatalf("ParseHTMLTableElem returned error: %v", err)
	}
	thead := table.ElementChildren()[0].(*ElemThead)
	row := thead.ElementChildren()[0].(*ElemTr)
	cells := row.ElementChildren()
	if len(cells) != 5 {
		t.Fatalf("expected 5 header cells, got %d", len(cells))
	}
	for i := 0; i < 4; i++ {
		cell := cells[i].(*ElemTh)
		if got, ok := cell.Attribute("whiteSpace"); !ok || got != "nowrap" {
			t.Fatalf("expected cell %d whiteSpace=nowrap, got %q (present=%v)", i+1, got, ok)
		}
	}
	cell5 := cells[4].(*ElemTh)
	if _, ok := cell5.Attribute("whiteSpace"); ok {
		t.Fatalf("expected 5th header cell not to have whiteSpace override")
	}
}

func TestParseHTMLTableElem_AppliesTableLayoutAuto(t *testing.T) {
	css := `#timesheet-table { table-layout: auto; }`
	html := `<table id="timesheet-table"><thead><tr><th>A</th></tr></thead></table>`
	table, err := ParseHTMLTableElem(html, css)
	if err != nil {
		t.Fatalf("ParseHTMLTableElem returned error: %v", err)
	}
	if got, ok := table.Attribute("tableLayout"); !ok || got != "auto" {
		t.Fatalf("expected tableLayout=auto, got %q (present=%v)", got, ok)
	}
}

func TestParseHTMLDocFlow_IncludesTopLevelImage(t *testing.T) {
	h := "<div id=\"closing\">Regards</div>" +
		"<img id=\"signature\" src=\"Unterschrift.png\" width=\"100\" height=\"53\">"
	elements, err := ParseHTMLDocFlow(h, "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 top-level elements, got %d", len(elements))
	}
	img, ok := elements[1].(*ElemImg)
	if !ok {
		t.Fatalf("expected second element to be *ElemImg, got %T", elements[1])
	}
	if got, ok := img.Attribute("src"); !ok || got != "Unterschrift.png" {
		t.Fatalf("expected src=Unterschrift.png, got %q (present=%v)", got, ok)
	}
	if got, ok := img.Attribute("width"); !ok || got != "100" {
		t.Fatalf("expected width=100, got %q (present=%v)", got, ok)
	}
	if got, ok := img.Attribute("height"); !ok || got != "53" {
		t.Fatalf("expected height=53, got %q (present=%v)", got, ok)
	}
}

func TestParseHTMLDocFlow_IncludesNestedImageInDiv(t *testing.T) {
	h := "<div>Regards<img src=\"Unterschrift.png\" width=\"100\" height=\"53\"></div>"
	elements, err := ParseHTMLDocFlow(h, "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	div, ok := elements[0].(*ElemDiv)
	if !ok {
		t.Fatalf("expected first element to be *ElemDiv, got %T", elements[0])
	}
	children := div.ElementChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 nested children, got %d", len(children))
	}
	if _, ok := children[1].(*ElemImg); !ok {
		t.Fatalf("expected second child to be *ElemImg, got %T", children[1])
	}
}

func TestParseHTMLDocFlow_IncludesTopLevelUseTemplate(t *testing.T) {
	h := "<use-template name=\"Logo1\" x=\"75\" y=\"67\" width=\"56.1\" height=\"56.1\"></use-template>"
	elements, err := ParseHTMLDocFlow(h, "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	useTpl, ok := elements[0].(*ElemUseTemplate)
	if !ok {
		t.Fatalf("expected first element to be *ElemUseTemplate, got %T", elements[0])
	}
	if got, ok := useTpl.Attribute("name"); !ok || got != "Logo1" {
		t.Fatalf("expected name=Logo1, got %q (present=%v)", got, ok)
	}
	if got, ok := useTpl.Attribute("x"); !ok || got != "75" {
		t.Fatalf("expected x=75, got %q (present=%v)", got, ok)
	}
	if got, ok := useTpl.Attribute("y"); !ok || got != "67" {
		t.Fatalf("expected y=67, got %q (present=%v)", got, ok)
	}
	if got, ok := useTpl.Attribute("width"); !ok || got != "56.1" {
		t.Fatalf("expected width=56.1, got %q (present=%v)", got, ok)
	}
	if got, ok := useTpl.Attribute("height"); !ok || got != "56.1" {
		t.Fatalf("expected height=56.1, got %q (present=%v)", got, ok)
	}
}

func TestParseHTMLDocFlow_IncludesTopLevelFooterAsContainer(t *testing.T) {
	h := `<footer><span>Footer Line 1</span><br><span>Footer Line 2</span></footer>`
	css := `footer { position: running(site-footer); }`
	elements, err := ParseHTMLDocFlow(h, css)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	footer, ok := elements[0].(*ElemDiv)
	if !ok {
		t.Fatalf("expected footer to map to *ElemDiv container, got %T", elements[0])
	}
	if got, ok := footer.Attribute("position"); !ok || got != "running(site-footer)" {
		t.Fatalf("expected footer position=running(site-footer), got %q (present=%v)", got, ok)
	}
	if len(footer.ElementChildren()) == 0 {
		t.Fatalf("expected footer container to keep children")
	}
}

func TestParseHTMLDocFlow_IncludesTopLevelHeading(t *testing.T) {
	const h1CSS = "h1 { font-size: 20; font-weight: normal; }"
	elements, err := ParseHTMLDocFlow("<h1>Leistungsnachweis</h1>", h1CSS)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	heading, ok := elements[0].(*ElemH1)
	if !ok {
		t.Fatalf("expected first element to be *ElemH1, got %T", elements[0])
	}
	children := heading.ElementChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 heading child, got %d", len(children))
	}
	textNode, ok := children[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected heading child to be *PDFTextNode, got %T", children[0])
	}
	if textNode.Text != "Leistungsnachweis" {
		t.Fatalf("expected heading text Leistungsnachweis, got %q", textNode.Text)
	}
	style := heading.ElementStyle()
	if style == nil {
		t.Fatalf("expected h1 element to carry a default heading style")
	}
	if style.FontSize != 20 {
		t.Fatalf("expected default h1 font size 20, got %v", style.FontSize)
	}
	if style.FontStyle != "B" {
		t.Fatalf("expected default h1 font style B, got %q", style.FontStyle)
	}
}

func TestParseHTMLDocFlow_H1FontWeightNormalClearsDefaultBold(t *testing.T) {
	const h1CSS = "h1 { font-size: 20; font-weight: normal; }"
	elements, err := ParseHTMLDocFlow("<h1>Leistungsnachweis</h1>", h1CSS)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	heading := elements[0].(*ElemH1)
	children := heading.ElementChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 heading child, got %d", len(children))
	}
	textNode, ok := children[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected heading child to be *PDFTextNode, got %T", children[0])
	}
	if textNode.Style == nil {
		t.Fatalf("expected heading text node to carry CSS-derived style override")
	}
	if !textNode.Style.FontStyleSet {
		t.Fatalf("expected heading text node to mark font-style as explicitly set")
	}
	if textNode.Style.FontStyle != "" {
		t.Fatalf("expected font-weight normal to clear bold style, got %q", textNode.Style.FontStyle)
	}
	if textNode.Style.FontSize != 20 {
		t.Fatalf("expected CSS h1 font size 20, got %v", textNode.Style.FontSize)
	}
}

func TestParseHTMLDocFlow_ComposesBoldAndItalicIndependently(t *testing.T) {
	for _, css := range []string{
		`span { font-style: italic; font-weight: bold; }`,
		`span { font-weight: bold; font-style: italic; }`,
	} {
		elements, err := ParseHTMLDocFlow(`<div><span>combined</span></div>`, css)
		if err != nil {
			t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
		}
		children := elements[0].ElementChildren()
		if len(children) != 1 {
			t.Fatalf("expected one styled child, got %d", len(children))
		}
		text, ok := children[0].(*PDFTextNode)
		if !ok || text.Style == nil || text.Style.FontStyle != "IB" {
			t.Fatalf("expected independent italic+bold style IB, got %#v", children[0])
		}
	}
}

func TestParseHTMLDocFlow_IncludesTopLevelParagraph(t *testing.T) {
	elements, err := ParseHTMLDocFlow("<p id=\"lead\">Hello paragraph</p>", "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	paragraph, ok := elements[0].(*ElemDiv)
	if !ok {
		t.Fatalf("expected paragraph to map to *ElemDiv, got %T", elements[0])
	}
	if got, ok := paragraph.Attribute("id"); !ok || got != "lead" {
		t.Fatalf("expected paragraph id=lead, got %q (present=%v)", got, ok)
	}
	children := paragraph.ElementChildren()
	if len(children) != 1 {
		t.Fatalf("expected one paragraph child, got %d", len(children))
	}
	textNode, ok := children[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected paragraph child to be *PDFTextNode, got %T", children[0])
	}
	if textNode.Text != "Hello paragraph" {
		t.Fatalf("expected paragraph text Hello paragraph, got %q", textNode.Text)
	}
}

func TestParseHTMLDocFlow_NestedHeadingAndParagraphInDiv(t *testing.T) {
	html := `<div id="card"><h1>Title</h1><p>Body line</p></div>`
	elements, err := ParseHTMLDocFlow(html, "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	card, ok := elements[0].(*ElemDiv)
	if !ok {
		t.Fatalf("expected card to be *ElemDiv, got %T", elements[0])
	}
	children := card.ElementChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 nested children, got %d", len(children))
	}
	if _, ok := children[0].(*ElemH1); !ok {
		t.Fatalf("expected first nested child to be *ElemH1, got %T", children[0])
	}
	paragraph, ok := children[1].(*ElemDiv)
	if !ok {
		t.Fatalf("expected second nested child to be *ElemDiv paragraph, got %T", children[1])
	}
	paraChildren := paragraph.ElementChildren()
	if len(paraChildren) != 1 {
		t.Fatalf("expected one paragraph child, got %d", len(paraChildren))
	}
	paraText, ok := paraChildren[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected paragraph child to be *PDFTextNode, got %T", paraChildren[0])
	}
	if paraText.Text != "Body line" {
		t.Fatalf("expected paragraph text Body line, got %q", paraText.Text)
	}
}

func TestParseHTMLDocFlow_NestedParagraphInheritsParentStyle(t *testing.T) {
	html := `<div id="card"><p id="body">Body line</p></div>`
	css := `#card { color: #223; background-color: #f3f3f3; border: 1px solid #c5c5c5; } #body { font-size: 11; }`

	elements, err := ParseHTMLDocFlow(html, css)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	card := elements[0].(*ElemDiv)
	paragraph := card.ElementChildren()[0].(*ElemDiv)
	paragraphStyle := paragraph.ElementStyle()
	if paragraphStyle != nil {
		if paragraphStyle.BorderStyle != "" || paragraphStyle.BorderWidth != 0 || paragraphStyle.BorderColor != "" {
			t.Fatalf("expected paragraph to not inherit border decoration, got style=%+v", paragraphStyle)
		}
		if paragraphStyle.BackgroundColor != "" {
			t.Fatalf("expected paragraph to not inherit background decoration, got style=%+v", paragraphStyle)
		}
	}
	textNode, ok := paragraph.ElementChildren()[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected paragraph child to be *PDFTextNode, got %T", paragraph.ElementChildren()[0])
	}
	if textNode.Style == nil {
		t.Fatalf("expected inherited/merged paragraph text style to be set")
	}
	if textNode.Style.FontColor != "#223" {
		t.Fatalf("expected inherited font color #223, got %q", textNode.Style.FontColor)
	}
	if textNode.Style.FontSize != 11 {
		t.Fatalf("expected paragraph font size 11, got %v", textNode.Style.FontSize)
	}
	if textNode.Style.BorderStyle != "" || textNode.Style.BorderWidth != 0 || textNode.Style.BorderColor != "" {
		t.Fatalf("expected text node to not inherit border decoration, got style=%+v", textNode.Style)
	}
	if textNode.Style.BackgroundColor != "" {
		t.Fatalf("expected text node to not inherit background decoration, got style=%+v", textNode.Style)
	}
}

func TestParseHTMLDocFlow_NestedHeadingInheritsParentColor(t *testing.T) {
	html := `<div id="card"><h2>Section Title</h2></div>`
	css := `#card { color: #0f766e; }`

	elements, err := ParseHTMLDocFlow(html, css)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	card := elements[0].(*ElemDiv)
	heading, ok := card.ElementChildren()[0].(*ElemH2)
	if !ok {
		t.Fatalf("expected nested heading to be *ElemH2, got %T", card.ElementChildren()[0])
	}
	textNode, ok := heading.ElementChildren()[0].(*PDFTextNode)
	if !ok {
		t.Fatalf("expected heading child to be *PDFTextNode, got %T", heading.ElementChildren()[0])
	}
	if textNode.Style == nil {
		t.Fatalf("expected heading text style to be set")
	}
	if textNode.Style.FontColor != "#0f766e" {
		t.Fatalf("expected inherited heading color #0f766e, got %q", textNode.Style.FontColor)
	}
}

func TestParseHTMLDocFlow_IncludesInlineCurrencyValueInDiv(t *testing.T) {
	html := `<div><span>Total:</span> <currency-value v="22500"></currency-value></div>`
	elements, err := ParseHTMLDocFlow(html, "")
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(elements))
	}
	div, ok := elements[0].(*ElemDiv)
	if !ok {
		t.Fatalf("expected first element to be *ElemDiv, got %T", elements[0])
	}
	children := div.ElementChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 nested children, got %d", len(children))
	}
	if _, ok := children[0].(*PDFTextNode); !ok {
		t.Fatalf("expected first child to be *PDFTextNode, got %T", children[0])
	}
	currency, ok := children[1].(*ElemCurrencyValue)
	if !ok {
		t.Fatalf("expected second child to be *ElemCurrencyValue, got %T", children[1])
	}
	if currency.Value != 22500 {
		t.Fatalf("expected currency value 22500, got %v", currency.Value)
	}
}

func TestParseHTMLDocFlow_InlineCurrencyValueErrorIncludesSnippet(t *testing.T) {
	_, err := ParseHTMLDocFlow(`<div>Total: <currency-value v="abc"></currency-value></div>`, "")
	if err == nil {
		t.Fatalf("expected parse error for invalid inline currency-value")
	}
	msg := err.Error()
	if !strings.Contains(msg, `<currency-value> invalid v="abc"`) {
		t.Fatalf("expected error to include invalid currency-value details, got %q", msg)
	}
	if !strings.Contains(msg, `<currency-value v="abc"></currency-value>`) {
		t.Fatalf("expected error to include offending HTML snippet, got %q", msg)
	}
}
