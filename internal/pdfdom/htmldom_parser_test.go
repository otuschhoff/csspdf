package pdfdom

import (
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
		"<table><thead><tr><th class="nowrap-col"><span>Date</span></th></tr></thead></table>",
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
