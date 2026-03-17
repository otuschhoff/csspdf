package pdfrender

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/otuschhoff/gofpdf"
	"github.com/otuschhoff/invoice-gen/internal/format"
	"github.com/otuschhoff/invoice-gen/internal/i18n"
	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
	templateload "github.com/otuschhoff/invoice-gen/internal/template"
)

// Company holds the business owner's company information used for PDF headers and footers.
type Company struct {
	Name     string
	Suffix   string
	FullName string
	Street   string
	PLZ      string
	City     string
	Country  string
	Tel      string
	Mail     string
	VAT      string
	Bank     BankDetails
	FiscalID string
	FiscalNo string
}

// BankDetails holds the bank account details for a company.
type BankDetails struct {
	Name string
	IBAN string
	BIC  string
}

// Style holds font and colour styling used across PDF rendering.
type Style struct {
	FontFace           string
	FontColor          string
	FontColorSub       string
	FontSize           int
	FontSizeSmall      int
	FontSizeTitle      int
	TableCellYOffset   float64
	Normal             pdfdom.StyleVariant
	Small              pdfdom.StyleVariant
	SmallGreyed        pdfdom.StyleVariant
	Sub                pdfdom.StyleVariant
	PageNum            pdfdom.StyleVariant
	PageTot            pdfdom.StyleVariant
	Footer             pdfdom.StyleVariant
	Title              pdfdom.StyleVariant
	CompanyName        pdfdom.StyleVariant
	CompanyNameSmall   pdfdom.StyleVariant
	CompanySuffix      pdfdom.StyleVariant
	CompanySuffixSmall pdfdom.StyleVariant
	DocType            pdfdom.StyleVariant
}

const (
	footerLogoRadius = 4.8
	footerLine1Y     = 20.0
	footerLine2Y     = 36.0
	footerLine3Y     = 45.0
	footerHeight     = 52.0
	footerLogoGap    = 6.0
	footerBulletSep  = " \x95 "
)

// LayoutPDF holds the shared PDF document and pre-built templates used by all
// standalone rendering subcommands. Build one with NewLayoutPDF; use the
// exported methods to compose pages.
type LayoutPDF struct {
	PDF               *gofpdf.Fpdf
	Style             *Style
	Formatter         *format.Formatter
	I18n              *i18n.I18n
	TableRdr          *TableRenderer
	DeferFlowPageNum  bool
	CurrentPage       int
	TotalPages        int
	pageWidth         float64
	pageHeight        float64
	currentMargins    templateload.PageMargins
	defaultPage       templateload.PageSettings
	firstPage         templateload.PageSettings
	defaultAssets     layoutPageAssets
	firstAssets       layoutPageAssets
	footerTpl         gofpdf.Template
	footerSize        gofpdf.SizeType
	footerPos         gofpdf.PointType
	logoHeaderTpl     gofpdf.Template
	logoHeaderSize    gofpdf.SizeType
	runningFooterTpl  gofpdf.Template
	runningFooterSize gofpdf.SizeType
}

type layoutPageAssets struct {
	pageWidth      float64
	pageHeight     float64
	footerTpl      gofpdf.Template
	footerSize     gofpdf.SizeType
	footerPos      gofpdf.PointType
	logoHeaderTpl  gofpdf.Template
	logoHeaderSize gofpdf.SizeType
}

// NewLayoutPDF creates a LayoutPDF from pre-loaded company and style data.
func NewLayoutPDF(company *Company, style *Style, defaultPage, firstPage templateload.PageSettings, i18nInst *i18n.I18n, formatter *format.Formatter) (*LayoutPDF, error) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: firstPage.Width, Ht: firstPage.Height})

	ringTpl := CreateRingLogoTemplate(pdf, LogoBaseRadius, LogoTplCenter, LogoTplCenter)
	if err := addFuturaMediumFont(pdf); err != nil {
		return nil, err
	}

	defaultAssets := buildLayoutPageAssets(pdf, ringTpl, company, defaultPage.Width, defaultPage.Height)
	firstAssets := buildLayoutPageAssets(pdf, ringTpl, company, firstPage.Width, firstPage.Height)

	tableRdr := NewTableRenderer(pdf, &TableStyle{
		Title:  style.Title,
		Normal: style.Normal,
		Small:  style.Small,
	}, formatter)

	return &LayoutPDF{
		PDF:            pdf,
		Style:          style,
		Formatter:      formatter,
		I18n:           i18nInst,
		TableRdr:       tableRdr,
		pageWidth:      firstPage.Width,
		pageHeight:     firstPage.Height,
		currentMargins: firstPage.Margins,
		defaultPage:    defaultPage,
		firstPage:      firstPage,
		defaultAssets:  defaultAssets,
		firstAssets:    firstAssets,
		footerTpl:      firstAssets.footerTpl,
		footerSize:     firstAssets.footerSize,
		footerPos:      firstAssets.footerPos,
		logoHeaderTpl:  firstAssets.logoHeaderTpl,
		logoHeaderSize: firstAssets.logoHeaderSize,
	}, nil
}

func buildLayoutPageAssets(pdf *gofpdf.Fpdf, ringTpl gofpdf.Template, company *Company, pageWidth, pageHeight float64) layoutPageAssets {
	line1Right := company.Name
	line1Left := company.Suffix
	line2 := fmt.Sprintf("%s, %s %s, %s%s%s%s%s",
		company.Street, company.PLZ, company.City, company.Country,
		footerBulletSep, company.Tel, footerBulletSep, company.Mail,
	)
	line3 := fmt.Sprintf("Bankkonto: %s%s%s%sBIC: %s",
		company.Bank.IBAN, footerBulletSep, company.Bank.Name, footerBulletSep, company.Bank.BIC,
	)

	footerTpl := pdf.CreateTemplateCustomNamed(
		gofpdf.PointType{X: 0, Y: 0},
		gofpdf.SizeType{Wd: pageWidth, Ht: footerHeight},
		"INV-FOOTER",
		func(t *gofpdf.Tpl) {
			t.SetTextColor(0x22, 0x22, 0x22)
			logoCx := pageWidth / 2.0
			centerLineY := footerLine1Y

			rightFontSize := 10.0
			t.SetFont("Helvetica", "", rightFontSize)
			rightTextW := t.GetStringWidth(line1Right)
			rightEdge := logoCx - footerLogoRadius - footerLogoGap
			t.Text(rightEdge-rightTextW, centerLineY+rightFontSize*0.33, line1Right)

			logoScale := footerLogoRadius / LogoBaseRadius
			t.UseTemplateScaled(ringTpl,
				gofpdf.PointType{X: logoCx - LogoTplCenter*logoScale, Y: centerLineY - LogoTplCenter*logoScale},
				gofpdf.SizeType{Wd: LogoTplExtent * logoScale, Ht: LogoTplExtent * logoScale},
			)

			leftFontSize := 7.0
			t.SetFont("Helvetica", "", leftFontSize)
			t.SetTextColor(0x44, 0x44, 0x44)
			t.Text(logoCx+footerLogoRadius+footerLogoGap, centerLineY+leftFontSize*0.33, line1Left)
			t.SetTextColor(0x22, 0x22, 0x22)

			t.SetFont("Helvetica", "", 7)
			line2W := t.GetStringWidth(line2)
			t.Text((pageWidth-line2W)/2.0, footerLine2Y, line2)

			line3W := t.GetStringWidth(line3)
			t.Text((pageWidth-line3W)/2.0, footerLine3Y, line3)
		},
	)

	logoHeaderTpl := pdf.CreateTemplateCustomNamed(
		gofpdf.PointType{X: 0, Y: 0},
		gofpdf.SizeType{Wd: pageWidth, Ht: 120},
		"Logo1",
		func(t *gofpdf.Tpl) {
			scale := 16.5 / LogoBaseRadius
			t.UseTemplateScaled(ringTpl,
				gofpdf.PointType{X: 75 - LogoTplCenter*scale, Y: 67 - LogoTplCenter*scale},
				gofpdf.SizeType{Wd: LogoTplExtent * scale, Ht: LogoTplExtent * scale},
			)
			t.SetTextColor(0x22, 0x22, 0x22)
			t.SetFont("Futura-Medium", "", 20)
			t.Text(106, 43+20, "Oliver Tuschhoff")
			t.SetTextColor(0x44, 0x44, 0x44)
			t.SetFont("Helvetica", "", 15)
			t.Text(106, 73+12, "Beratung und Training")
		},
	)

	_, footerSize := footerTpl.Size()
	_, logoHeaderSize := logoHeaderTpl.Size()

	return layoutPageAssets{
		pageWidth:      pageWidth,
		pageHeight:     pageHeight,
		footerTpl:      footerTpl,
		footerSize:     footerSize,
		footerPos:      gofpdf.PointType{X: 0, Y: pageHeight - footerHeight - 20},
		logoHeaderTpl:  logoHeaderTpl,
		logoHeaderSize: logoHeaderSize,
	}
}

// BeginPage handles pagination (AddPage on page > 1), renders the Logo1 header
// on page 1, and applies the footer on every page.
func (l *LayoutPDF) BeginPage(page int) {
	settings, assets := l.pageConfigFor(page)
	if page > 1 {
		l.PDF.AddPageFormat("P", gofpdf.SizeType{Wd: assets.pageWidth, Ht: assets.pageHeight})
	}
	l.pageWidth = assets.pageWidth
	l.pageHeight = assets.pageHeight
	l.currentMargins = settings.Margins
	l.footerTpl = assets.footerTpl
	l.footerSize = assets.footerSize
	l.footerPos = assets.footerPos
	l.logoHeaderTpl = assets.logoHeaderTpl
	l.logoHeaderSize = assets.logoHeaderSize
	if page == 1 {
		l.PDF.UseTemplateScaled(l.logoHeaderTpl, gofpdf.PointType{X: 0, Y: 0}, l.logoHeaderSize)
	}
	l.PDF.UseTemplateScaled(l.footerTpl, l.footerPos, l.footerSize)
	l.renderRunningFooterTemplate()
}

// SetRunningFooterTemplateFromElement creates and stores a reusable footer
// template from a running footer element and applies it to the current page.
func (l *LayoutPDF) SetRunningFooterTemplateFromElement(name string, footerElem pdfdom.PDFElementNode) error {
	if footerElem == nil {
		return fmt.Errorf("running footer element is nil")
	}

	flowX, _, flowW := l.CurrentFlowBox()
	lines := runningFooterTextLines(footerElem)
	if len(lines) == 0 {
		return fmt.Errorf("running footer %q has no renderable content", name)
	}

	fontFace := "Helvetica"
	fontSize := 9.0
	if l.Style != nil {
		if strings.TrimSpace(l.Style.Normal.FontFace) != "" {
			fontFace = l.Style.Normal.FontFace
		}
		if l.Style.Normal.FontSize > 0 {
			fontSize = float64(l.Style.Normal.FontSize)
		}
	}

	lineHeight := fontSize * 1.2
	padding := 4.0
	height := padding*2 + lineHeight*float64(len(lines))
	if height > l.pageHeight {
		height = l.pageHeight
	}

	safeName := strings.ToUpper(strings.TrimSpace(name))
	if safeName == "" {
		safeName = "RUNNING-FOOTER"
	} else {
		safeName = "RUNNING-" + safeName
	}

	tpl := l.PDF.CreateTemplateCustomNamed(
		gofpdf.PointType{X: 0, Y: 0},
		gofpdf.SizeType{Wd: flowW, Ht: height},
		safeName,
		func(t *gofpdf.Tpl) {
			t.SetTextColor(0x22, 0x22, 0x22)
			t.SetFont(fontFace, "", fontSize)
			y := padding + fontSize
			for _, line := range lines {
				text := strings.TrimSpace(line)
				if text == "" {
					y += lineHeight
					continue
				}
				lineW := t.GetStringWidth(text)
				x := (flowW - lineW) / 2
				if x < 0 {
					x = 0
				}
				t.Text(x, y, text)
				y += lineHeight
			}
		},
	)

	l.runningFooterTpl = tpl
	l.runningFooterSize = gofpdf.SizeType{Wd: flowW, Ht: height}

	if flowX >= 0 {
		l.renderRunningFooterTemplate()
	}

	return nil
}

func (l *LayoutPDF) renderRunningFooterTemplate() {
	if l.runningFooterTpl == nil || l.runningFooterSize.Wd <= 0 || l.runningFooterSize.Ht <= 0 {
		return
	}
	x := (l.pageWidth - l.runningFooterSize.Wd) / 2
	if x < 0 {
		x = 0
	}
	y := l.pageHeight - l.currentMargins.Bottom - l.runningFooterSize.Ht
	if y < 0 {
		y = l.pageHeight - l.runningFooterSize.Ht
	}
	if y < 0 {
		y = 0
	}
	l.PDF.UseTemplateScaled(l.runningFooterTpl, gofpdf.PointType{X: x, Y: y}, l.runningFooterSize)
}

func runningFooterTextLines(node pdfdom.PDFNode) []string {
	lines := []string{""}
	appendToken := func(token string) {
		t := strings.TrimSpace(token)
		if t == "" {
			return
		}
		last := len(lines) - 1
		if strings.TrimSpace(lines[last]) == "" {
			lines[last] = t
			return
		}
		lines[last] += " " + t
	}

	var walk func(pdfdom.PDFNode)
	walk = func(n pdfdom.PDFNode) {
		switch v := n.(type) {
		case *pdfdom.PDFTextNode:
			appendToken(v.Text)
		case *pdfdom.ElemBr:
			lines = append(lines, "")
		case pdfdom.PDFElementNode:
			children := v.ElementChildren()
			lineBreaks := v.ElementChildLineBreaks()
			for idx, child := range children {
				if idx < len(lineBreaks) && lineBreaks[idx] {
					lines = append(lines, "")
				}
				walk(child)
			}
		}
	}

	walk(node)
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" {
			trimmed = append(trimmed, t)
		}
	}
	return trimmed
}

func (l *LayoutPDF) pageConfigFor(page int) (templateload.PageSettings, layoutPageAssets) {
	if page == 1 {
		return l.firstPage, l.firstAssets
	}
	return l.defaultPage, l.defaultAssets
}

// CurrentFlowBox returns the current page's content box origin and width.
func (l *LayoutPDF) CurrentFlowBox() (x, y, width float64) {
	x = l.currentMargins.Left
	y = l.currentMargins.Top
	width = l.pageWidth - l.currentMargins.Left - l.currentMargins.Right
	if width <= 0 {
		width = l.pageWidth
	}
	return x, y, width
}

// CurrentFlowBottom returns the y coordinate of the bottom of the content area.
func (l *LayoutPDF) CurrentFlowBottom() float64 {
	bottom := l.pageHeight - l.currentMargins.Bottom
	if bottom <= 0 {
		return l.pageHeight
	}
	return bottom
}

// StartFlow initialises page tracking for a multi-page flow.
func (l *LayoutPDF) StartFlow(pageCount int) {
	l.CurrentPage = 1
	if pageCount < 1 {
		pageCount = 1
	}
	l.TotalPages = pageCount
}

// NextFlowPage advances to the next page in the flow.
func (l *LayoutPDF) NextFlowPage() {
	if l.CurrentPage < 1 {
		l.CurrentPage = 1
	}
	l.CurrentPage++
	if l.CurrentPage > l.TotalPages {
		l.TotalPages = l.CurrentPage
	}
	l.BeginPage(l.CurrentPage)
	if !l.DeferFlowPageNum {
		l.RenderPageNum(l.CurrentPage, l.TotalPages)
	}
}

// RenderFinalFlowPageNums re-renders page numbers on all non-first pages after
// the total page count is known.
func (l *LayoutPDF) RenderFinalFlowPageNums() {
	finalTotal := l.TotalPages
	for page := 2; page <= finalTotal; page++ {
		settings, assets := l.pageConfigFor(page)
		l.pageWidth = assets.pageWidth
		l.pageHeight = assets.pageHeight
		l.currentMargins = settings.Margins
		l.PDF.SetPage(page)
		l.RenderPageNum(page, finalTotal)
	}
}

// ShouldBreakPageBefore reports whether node carries a break-before:page attribute.
func ShouldBreakPageBefore(node pdfdom.PDFElementNode) bool {
	value := flowBreakValue(node, "breakBefore")
	if value == "" {
		value = flowBreakValue(node, "break-before")
	}
	return value == "page"
}

// ShouldBreakPageAfter reports whether node carries a break-after:page attribute.
func ShouldBreakPageAfter(node pdfdom.PDFElementNode) bool {
	value := flowBreakValue(node, "breakAfter")
	if value == "" {
		value = flowBreakValue(node, "break-after")
	}
	return value == "page"
}

func flowBreakValue(node pdfdom.PDFElementNode, attr string) string {
	if node == nil {
		return ""
	}
	if value, ok := node.Attribute(attr); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

// RenderPageNum draws the current/total page number in the top-right corner.
// It is a no-op on page 1.
func (l *LayoutPDF) RenderPageNum(page, pageCount int) {
	if page <= 1 {
		return
	}
	l.PDF.SetTextColor(0x22, 0x22, 0x22)
	l.PDF.SetFont("Helvetica", "", 10)
	l.PDF.Text(l.pageWidth-98, 43, fmt.Sprintf("%d ", page))
	l.PDF.SetTextColor(0x66, 0x66, 0x66)
	l.PDF.SetFont("Helvetica", "", 7)
	l.PDF.Text(l.pageWidth-89, 43, fmt.Sprintf("/ %d", pageCount))
}

// ResolveFlowPlacement determines the final position and width of a flow element,
// applying absolute positioning when declared.
func (l *LayoutPDF) ResolveFlowPlacement(node pdfdom.PDFElementNode, flowX, flowY, flowW float64) (x, y, width float64, absolute bool) {
	x = flowX
	y = flowY
	width = flowW

	positionRaw, hasPosition := node.Attribute("position")
	if !hasPosition || !strings.EqualFold(strings.TrimSpace(positionRaw), "absolute") {
		return x, y, width, false
	}

	absolute = true

	left := htmlLengthToFloat(node, "left")
	right := htmlLengthToFloat(node, "right")
	top := htmlLengthToFloat(node, "top")
	declaredWidth := htmlLengthToFloat(node, "width")

	if top > 0 {
		y = top
	}

	if declaredWidth > 0 {
		width = declaredWidth
	} else if left > 0 && right > 0 {
		width = max(1, l.pageWidth-left-right)
	}

	if left > 0 {
		x = left
	} else if right > 0 {
		x = l.pageWidth - right - width
	}

	return x, y, width, absolute
}

// TableDefFromElement builds a TableDef from an HTML table element.
func (l *LayoutPDF) TableDefFromElement(table *pdfdom.ElemTable) (*TableDef, error) {
	tableWidth, err := parseLayoutFloatAttr(table, "width")
	if err != nil {
		return nil, err
	}
	padding, err := parseLayoutFloatAttr(table, "padding")
	if err != nil {
		return nil, err
	}
	rowHeightMin, err := parseLayoutFloatAttr(table, "rowHeightMin")
	if err != nil {
		return nil, err
	}

	tableDef := &TableDef{Width: tableWidth, Padding: padding, RowHeightMin: rowHeightMin}
	if bg, ok := table.Attribute("backgroundColor"); ok {
		tableDef.Background = strings.TrimSpace(bg)
	} else if bg, ok := table.Attribute("background-color"); ok {
		tableDef.Background = strings.TrimSpace(bg)
	}

	if borderColor, ok := table.Attribute("borderColor"); ok {
		tableDef.BorderColor = strings.TrimSpace(borderColor)
	} else if borderColor, ok := table.Attribute("border-color"); ok {
		tableDef.BorderColor = strings.TrimSpace(borderColor)
	}

	if borderStyle, ok := table.Attribute("borderStyle"); ok {
		tableDef.BorderStyle = strings.ToLower(strings.TrimSpace(borderStyle))
	} else if borderStyle, ok := table.Attribute("border-style"); ok {
		tableDef.BorderStyle = strings.ToLower(strings.TrimSpace(borderStyle))
	}

	tableDef.BorderWidth = htmlLengthToFloat(table, "borderWidth", "border-width")
	if borderRaw, ok := table.Attribute("border"); ok {
		bw, bs, bc := templateload.ParseBorderShorthand(borderRaw)
		if tableDef.BorderWidth <= 0 {
			tableDef.BorderWidth = bw
		}
		if tableDef.BorderStyle == "" {
			tableDef.BorderStyle = bs
		}
		if tableDef.BorderColor == "" {
			tableDef.BorderColor = bc
		}
	}
	columnsDefined := false
	colgroupCount := 0

	for _, child := range table.ElementChildren() {
		colgroup, ok := child.(*pdfdom.ElemColgroup)
		if !ok {
			continue
		}
		colgroupCount++
		if colgroupCount > 1 {
			return nil, fmt.Errorf("table may contain at most one colgroup")
		}
		cols := colgroup.ElementChildren()
		tableDef.Columns = make([]ColumnDef, 0, len(cols))
		for _, colNode := range cols {
			col, ok := colNode.(*pdfdom.ElemCol)
			if !ok {
				return nil, fmt.Errorf("colgroup contains non-col child")
			}
			width, ok := col.Attribute("width")
			if !ok {
				return nil, fmt.Errorf("col element missing width attribute")
			}
			var colWidth float64
			if _, err := fmt.Sscanf(width, "%f", &colWidth); err != nil {
				return nil, fmt.Errorf("invalid col width attribute: %q", width)
			}
			tableDef.Columns = append(tableDef.Columns, ColumnDef{Width: colWidth})
		}
		columnsDefined = true
	}

	processRow := func(row *pdfdom.ElemTr, isHeader bool) error {
		cells := row.ElementChildren()
		if !columnsDefined {
			tableDef.Columns = make([]ColumnDef, 0, len(cells))
			for _, child := range cells {
				cell, err := tableCellFromNode(child)
				if err != nil {
					return err
				}
				tableDef.Columns = append(tableDef.Columns, columnDefFromCell(cell))
			}
			columnsDefined = true
		}

		rowDef := RowDef{}
		if fill, ok := row.Attribute("fill"); ok {
			rowDef.Fill = fill
		}
		if stroke, ok := row.Attribute("stroke"); ok {
			strokeVal := stroke == "true"
			rowDef.Stroke = &strokeVal
		}
		if height, ok := row.Attribute("height"); ok {
			fmt.Sscanf(height, "%f", &rowDef.Height)
		}

		for _, child := range cells {
			cell, err := tableCellFromNode(child)
			if err != nil {
				return err
			}
			rowDef.Cells = append(rowDef.Cells, l.CellDefFromTableCell(cell, isHeader))
		}
		tableDef.Rows = append(tableDef.Rows, rowDef)
		return nil
	}

	for _, child := range table.ElementChildren() {
		switch elem := child.(type) {
		case *pdfdom.ElemColgroup:
			continue
		case *pdfdom.ElemThead:
			for _, rowNode := range elem.ElementChildren() {
				tr, ok := rowNode.(*pdfdom.ElemTr)
				if !ok {
					return nil, fmt.Errorf("thead contains non-tr child")
				}
				if err := processRow(tr, true); err != nil {
					return nil, err
				}
			}
		case *pdfdom.ElemTbody:
			for _, rowNode := range elem.ElementChildren() {
				tr, ok := rowNode.(*pdfdom.ElemTr)
				if !ok {
					return nil, fmt.Errorf("tbody contains non-tr child")
				}
				if err := processRow(tr, false); err != nil {
					return nil, err
				}
			}
		case *pdfdom.ElemTr:
			if err := processRow(elem, false); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported table child element %T", child)
		}
	}

	return tableDef, nil
}

func parseLayoutFloatAttr(node pdfdom.PDFElementNode, attr string) (float64, error) {
	value, ok := node.Attribute(attr)
	if !ok {
		return 0, fmt.Errorf("missing required %s attribute on %s", attr, node.ElementType())
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return 0, fmt.Errorf("invalid %s attribute on %s: %q", attr, node.ElementType(), value)
	}
	return parsed, nil
}

func tableCellFromNode(node pdfdom.PDFNode) (pdfdom.PDFElementNode, error) {
	switch cell := node.(type) {
	case *pdfdom.ElemTd:
		return cell, nil
	case *pdfdom.ElemTh:
		return cell, nil
	default:
		return nil, fmt.Errorf("table row contains non-cell child %T", node)
	}
}

func columnDefFromCell(cell pdfdom.PDFElementNode) ColumnDef {
	col := ColumnDef{}
	if width, ok := cell.Attribute("width"); ok {
		fmt.Sscanf(width, "%f", &col.Width)
	}
	if align, ok := cell.Attribute("align"); ok {
		col.Align = tableAlignFromAttr(align)
	}
	return col
}

// CellDefFromTableCell builds a CellDef from an HTML table cell element.
func (l *LayoutPDF) CellDefFromTableCell(cellElem pdfdom.PDFElementNode, isHeader bool) CellDef {
	cell := CellDef{}
	cellHasExplicitAlign := false
	if pad := htmlLengthToFloat(cellElem, "padding"); pad > 0 {
		cell.PaddingTop = pad
		cell.PaddingRight = pad
		cell.PaddingBottom = pad
		cell.PaddingLeft = pad
	}
	if pad := htmlLengthToFloat(cellElem, "paddingTop", "padding-top"); pad > 0 {
		cell.PaddingTop = pad
	}
	if pad := htmlLengthToFloat(cellElem, "paddingRight", "padding-right"); pad > 0 {
		cell.PaddingRight = pad
	}
	if pad := htmlLengthToFloat(cellElem, "paddingBottom", "padding-bottom"); pad > 0 {
		cell.PaddingBottom = pad
	}
	if pad := htmlLengthToFloat(cellElem, "paddingLeft", "padding-left"); pad > 0 {
		cell.PaddingLeft = pad
	}
	if align, ok := cellElem.Attribute("align"); ok {
		cell.Align = tableAlignFromAttr(align)
		cellHasExplicitAlign = true
	} else if isHeader {
		cell.Align = "C"
	}
	if colspan, ok := cellElem.Attribute("colspan"); ok {
		fmt.Sscanf(colspan, "%d", &cell.Colspan)
	}
	if _, ok := cellElem.(*pdfdom.ElemTh); ok {
		cell.Bold = true
	}

	// Check for a value element first (currency, dates, durations, man-days).
	for _, child := range cellElem.ElementChildren() {
		switch v := child.(type) {
		case *pdfdom.ElemCurrencyValue:
			cell.Text = v.Format(l.Formatter)
			if isHeader {
				cell.Bold = true
			}
			return cell
		case *pdfdom.ElemDateValue:
			cell.Text = v.Format(l.Formatter)
			if isHeader {
				cell.Bold = true
			}
			return cell
		case *pdfdom.ElemDurationValue:
			cell.Text = v.Format(l.Formatter)
			if isHeader {
				cell.Bold = true
			}
			return cell
		case *pdfdom.ElemManDaysValue:
			cell.Text = v.Format(l.Formatter)
			if isHeader {
				cell.Bold = true
			}
			return cell
		}
	}

	textChildren := make([]*pdfdom.PDFTextNode, 0, len(cellElem.ElementChildren()))
	for _, child := range cellElem.ElementChildren() {
		if textNode, ok := child.(*pdfdom.PDFTextNode); ok {
			textChildren = append(textChildren, textNode)
		}
	}
	if len(textChildren) == 0 {
		return cell
	}

	main := textChildren[0]
	cell.Text = l.ResolvePDFTextNode(main)
	if main.Style != nil {
		if main.Style.Align != "" && !cellHasExplicitAlign {
			cell.Align = textAlignToTableAlign(main.Style.Align)
		}
		if strings.Contains(main.Style.FontStyle, "B") {
			cell.Bold = true
		}
		if main.Style.FontSize > 0 && main.Style.FontSize <= 7 {
			cell.Small = true
		}
	}

	if len(textChildren) > 1 {
		sub := textChildren[1]
		cell.SubText = l.ResolvePDFTextNode(sub)
		if sub.Style != nil {
			cell.SubFontFace = sub.Style.FontFace
			cell.SubFontColor = sub.Style.FontColor
			cell.SubFontSize = sub.Style.FontSize
		}
	}

	return cell
}

// ResolvePDFTextNode resolves the text content of a PDFTextNode, applying i18n lookups.
func (l *LayoutPDF) ResolvePDFTextNode(node *pdfdom.PDFTextNode) string {
	if node == nil {
		return ""
	}
	if node.I18nKey == "" {
		return node.Text
	}
	if len(node.I18nVars) > 0 {
		return l.I18n.TWithVars(node.I18nKey, node.I18nVars)
	}
	return l.I18n.T(node.I18nKey)
}

func textAlignToTableAlign(align pdfdom.TextAlign) string {
	switch align {
	case pdfdom.TextAlignCenter:
		return "C"
	case pdfdom.TextAlignRight:
		return "R"
	default:
		return "L"
	}
}

func tableAlignFromAttr(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "center", "c":
		return "C"
	case "right", "r":
		return "R"
	default:
		return "L"
	}
}

// TemplateByName returns the pre-built gofpdf.Template for a named slot.
func (l *LayoutPDF) TemplateByName(name string) (gofpdf.Template, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "logo1", "logo-header", "logo_header":
		return l.logoHeaderTpl, nil
	case "inv-footer", "inv_footer", "footer":
		return l.footerTpl, nil
	default:
		return nil, fmt.Errorf("unknown template name %q", name)
	}
}

// RenderUseTemplateElement renders a <use-template> element using the named template.
func (l *LayoutPDF) RenderUseTemplateElement(elem *pdfdom.ElemUseTemplate) error {
	if elem == nil {
		return fmt.Errorf("use-template element is nil")
	}

	name, ok := elem.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("use-template missing name attribute")
	}

	x, err := parseUseTemplateFloatAttr(elem, "x")
	if err != nil {
		return err
	}
	y, err := parseUseTemplateFloatAttr(elem, "y")
	if err != nil {
		return err
	}
	width, err := parseUseTemplateFloatAttr(elem, "width")
	if err != nil {
		return err
	}
	height, err := parseUseTemplateFloatAttr(elem, "height")
	if err != nil {
		return err
	}

	tpl, err := l.TemplateByName(name)
	if err != nil {
		return err
	}

	l.PDF.UseTemplateScaled(
		tpl,
		gofpdf.PointType{X: x, Y: y},
		gofpdf.SizeType{Wd: width, Ht: height},
	)

	return nil
}

func parseUseTemplateFloatAttr(node pdfdom.PDFElementNode, attr string) (float64, error) {
	raw, ok := node.Attribute(attr)
	if !ok {
		return 0, fmt.Errorf("use-template missing %s attribute", attr)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("use-template invalid %s value %q: %w", attr, raw, err)
	}
	return v, nil
}

func addFuturaMediumFont(pdf *gofpdf.Fpdf) error {
	for _, fontPath := range []string{"Futura-Medium.ttf", "resources/Futura-Medium.ttf", "../Futura-Medium.ttf", "../resources/Futura-Medium.ttf"} {
		if _, statErr := os.Stat(fontPath); statErr == nil {
			pdf.AddUTF8Font("Futura-Medium", "", fontPath)
			if pdf.Err() {
				return fmt.Errorf("failed to load Futura-Medium from %s: %w", fontPath, pdf.Error())
			}
			return nil
		}
	}
	return fmt.Errorf("Futura-Medium.ttf not found in known paths")
}
