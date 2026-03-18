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
	footerBulletSep = " • "
)

// LayoutPDF holds the shared PDF document and pre-built templates used by all
// standalone rendering subcommands. Build one with NewLayoutPDF; use the
// exported methods to compose pages.
type LayoutPDF struct {
	PDF               *gofpdf.Fpdf
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
	ringTpl           gofpdf.Template
	runningFooterTpl  gofpdf.Template
	runningFooterSize gofpdf.SizeType
	userTemplates     map[string]gofpdf.Template
}

type layoutPageAssets struct {
	pageWidth  float64
	pageHeight float64
}

// NewLayoutPDF creates a LayoutPDF from page settings and locale/format helpers.
func NewLayoutPDF(defaultPage, firstPage templateload.PageSettings, i18nInst *i18n.I18n, formatter *format.Formatter) (*LayoutPDF, error) {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: firstPage.Width, Ht: firstPage.Height})

	ringTpl := CreateRingLogoTemplate(pdf, LogoBaseRadius, LogoTplCenter, LogoTplCenter)
	if err := addFuturaMediumFont(pdf); err != nil {
		return nil, err
	}

	defaultAssets := buildLayoutPageAssets(defaultPage.Width, defaultPage.Height)
	firstAssets := buildLayoutPageAssets(firstPage.Width, firstPage.Height)

	tableRdr := NewTableRenderer(pdf, formatter)

	return &LayoutPDF{
		PDF:            pdf,
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
		ringTpl:        ringTpl,
	}, nil
}

func buildLayoutPageAssets(pageWidth, pageHeight float64) layoutPageAssets {
	return layoutPageAssets{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
	}
}

// BeginPage handles pagination (AddPage on page > 1) and stamps the running
// footer template on every page.
func (l *LayoutPDF) BeginPage(page int) {
	settings, assets := l.pageConfigFor(page)
	if page > 1 {
		l.PDF.AddPageFormat("P", gofpdf.SizeType{Wd: assets.pageWidth, Ht: assets.pageHeight})
	}
	l.pageWidth = assets.pageWidth
	l.pageHeight = assets.pageHeight
	l.currentMargins = settings.Margins
	l.renderRunningFooterTemplate()
}

// SetRunningFooterTemplateFromElement creates and stores a reusable footer
// template from a running footer element and applies it to the current page.
// The footer element's children are rendered into a gofpdf template via the
// full element rendering pipeline: ElemUseTemplate children stamp named
// templates; all other children are rendered by PDFTextEngine.
func (l *LayoutPDF) SetRunningFooterTemplateFromElement(name string, footerElem pdfdom.PDFElementNode) error {
	if footerElem == nil {
		return fmt.Errorf("running footer element is nil")
	}

	// Read template dimensions from CSS-baked attributes.
	tplWidth := l.pageWidth
	if raw, ok := footerElem.Attribute("width"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil && v > 0 {
			tplWidth = v
		}
	}
	const defaultFooterHeight = 52.0
	tplHeight := defaultFooterHeight
	if raw, ok := footerElem.Attribute("height"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil && v > 0 {
			tplHeight = v
		}
	}

	safeName := strings.ToUpper(strings.TrimSpace(name))
	if safeName == "" {
		safeName = "RUNNING-FOOTER"
	} else {
		safeName = "RUNNING-" + safeName
	}

	children := footerElem.ElementChildren()
	i18nInst := l.I18n

	tpl := l.PDF.CreateTemplateCustomNamed(
		gofpdf.PointType{X: 0, Y: 0},
		gofpdf.SizeType{Wd: tplWidth, Ht: tplHeight},
		safeName,
		func(t *gofpdf.Tpl) {
			engine := NewPDFTextEngine(&t.Fpdf, i18nInst)
			currentY := 0.0
			for _, child := range children {
				childElem, ok := child.(pdfdom.PDFElementNode)
				if !ok {
					continue
				}
				x, y, w, isAbsolute := resolveFlowPlacementWith(childElem, 0, currentY, tplWidth, tplWidth)
				switch cn := childElem.(type) {
				case *pdfdom.ElemUseTemplate:
					tplName, hasName := cn.Attribute("name")
					if !hasName || strings.TrimSpace(tplName) == "" {
						continue
					}
					childTpl, err := l.TemplateByName(tplName)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: running footer %q: %v\n", name, err)
						continue
					}
					_, nativeSize := childTpl.Size()
					cw, ch := nativeSize.Wd, nativeSize.Ht
					if raw, ok := cn.Attribute("width"); ok {
						if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
							cw = v
						}
					}
					if raw, ok := cn.Attribute("height"); ok {
						if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
							ch = v
						}
					}
					if !isAbsolute && cw == nativeSize.Wd && ch == nativeSize.Ht {
						t.UseTemplate(childTpl)
					} else {
						t.UseTemplateScaled(childTpl,
							gofpdf.PointType{X: x, Y: y},
							gofpdf.SizeType{Wd: cw, Ht: ch},
						)
					}
					if !isAbsolute {
						currentY += ch
					}
				default:
					metrics, err := engine.RenderInBox(childElem, &pdfdom.PDFTextBox{
						X: x, Y: y, Width: w, Fit: pdfdom.TextFitWrap,
					})
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: running footer %q: failed to render child: %v\n", name, err)
						continue
					}
					if !isAbsolute {
						currentY += metrics.Height
					}
				}
			}
		},
	)

	l.runningFooterTpl = tpl
	l.runningFooterSize = gofpdf.SizeType{Wd: tplWidth, Ht: tplHeight}
	l.renderRunningFooterTemplate()
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

// resolveFlowPlacementWith is the pure-function core of placement resolution.
// pageW is the reference page/container width used for right-edge calculations.
func resolveFlowPlacementWith(node pdfdom.PDFElementNode, flowX, flowY, flowW, pageW float64) (x, y, width float64, absolute bool) {
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
		width = max(1, pageW-left-right)
	}

	if left > 0 {
		x = left
	} else if right > 0 {
		x = pageW - right - width
	}

	return x, y, width, absolute
}

// ResolveFlowPlacement determines the final position and width of a flow element,
// applying absolute positioning when declared.
func (l *LayoutPDF) ResolveFlowPlacement(node pdfdom.PDFElementNode, flowX, flowY, flowW float64) (x, y, width float64, absolute bool) {
	return resolveFlowPlacementWith(node, flowX, flowY, flowW, l.pageWidth)
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
	if face, ok := cellElem.Attribute("fontFace"); ok {
		cell.FontFace = strings.TrimSpace(face)
	} else if face, ok := cellElem.Attribute("font-face"); ok {
		cell.FontFace = strings.TrimSpace(face)
	}
	if fontStyle, ok := cellElem.Attribute("fontStyle"); ok {
		cell.FontStyle = strings.TrimSpace(fontStyle)
	} else if fontStyle, ok := cellElem.Attribute("font-style"); ok {
		cell.FontStyle = strings.TrimSpace(fontStyle)
	}
	if fontColor, ok := cellElem.Attribute("fontColor"); ok {
		cell.FontColor = strings.TrimSpace(fontColor)
	} else if fontColor, ok := cellElem.Attribute("font-color"); ok {
		cell.FontColor = strings.TrimSpace(fontColor)
	}
	if fontSize, ok := cellElem.Attribute("fontSize"); ok {
		cell.FontSize = parseTableFloat(fontSize)
	} else if fontSize, ok := cellElem.Attribute("font-size"); ok {
		cell.FontSize = parseTableFloat(fontSize)
	}
	if lineHeight, ok := cellElem.Attribute("lineHeight"); ok {
		cell.LineHeight = parseTableFloat(lineHeight)
	} else if lineHeight, ok := cellElem.Attribute("line-height"); ok {
		cell.LineHeight = parseTableFloat(lineHeight)
	}
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
		if main.Style.FontFace != "" {
			cell.FontFace = main.Style.FontFace
		}
		if main.Style.FontStyle != "" {
			cell.FontStyle = main.Style.FontStyle
		}
		if main.Style.FontColor != "" {
			cell.FontColor = main.Style.FontColor
		}
		if main.Style.FontSize > 0 {
			cell.FontSize = main.Style.FontSize
		}
		if main.Style.LineHeight > 0 {
			cell.LineHeight = main.Style.LineHeight
		}
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
	// User-defined templates (created by <create-template> elements) take
	// precedence over the built-in named slots.
	if l.userTemplates != nil {
		if tpl, ok := l.userTemplates[name]; ok {
			return tpl, nil
		}
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ringlogo", "ring-logo", "ring_logo":
		if l.ringTpl == nil {
			return nil, fmt.Errorf("ring logo template not initialised")
		}
		return l.ringTpl, nil
	default:
		return nil, fmt.Errorf("unknown template name %q", name)
	}
}

// RenderCreateTemplateElement captures the child elements of an
// ElemCreateTemplate into a named gofpdf template via CreateTemplateCustomNamed.
// The template is stored in l.userTemplates and can be referenced by name from
// any subsequent <use-template name="..."> element.
func (l *LayoutPDF) RenderCreateTemplateElement(elem *pdfdom.ElemCreateTemplate) error {
	if elem == nil {
		return fmt.Errorf("create-template element is nil")
	}
	name, ok := elem.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("create-template missing name attribute")
	}

	x, y := 0.0, 0.0
	if raw, ok := elem.Attribute("x"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			x = v
		}
	}
	if raw, ok := elem.Attribute("y"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			y = v
		}
	}
	var width, height float64
	if raw, ok := elem.Attribute("width"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			width = v
		}
	}
	if raw, ok := elem.Attribute("height"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			height = v
		}
	}

	children := elem.ElementChildren()
	i18nInst := l.I18n

	tpl := l.PDF.CreateTemplateCustomNamed(
		gofpdf.PointType{X: x, Y: y},
		gofpdf.SizeType{Wd: width, Ht: height},
		name,
		func(t *gofpdf.Tpl) {
			engine := NewPDFTextEngine(&t.Fpdf, i18nInst)
			currentY := y
			for _, child := range children {
				childElem, ok := child.(pdfdom.PDFElementNode)
				if !ok {
					continue
				}
				metrics, err := engine.RenderInBox(childElem, &pdfdom.PDFTextBox{
					X: x, Y: currentY, Width: width, Fit: pdfdom.TextFitWrap,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: create-template %q: failed to render child: %v\n", name, err)
					continue
				}
				currentY += metrics.Height
			}
		},
	)

	if l.userTemplates == nil {
		l.userTemplates = make(map[string]gofpdf.Template)
	}
	l.userTemplates[name] = tpl
	return nil
}

// RenderUseTemplateElement renders a <use-template> element using the named template.
// fallbackX and fallbackY are the current document-flow coordinates; they are
// used when the element does not carry explicit x or y attributes.
// Returns the rendered height, whether an explicit position was given (absolute=true
// means the caller should NOT advance the flow cursor), and any error.
func (l *LayoutPDF) RenderUseTemplateElement(elem *pdfdom.ElemUseTemplate, fallbackX, fallbackY float64) (renderedHeight float64, absolute bool, err error) {
	if elem == nil {
		return 0, false, fmt.Errorf("use-template element is nil")
	}

	name, ok := elem.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return 0, false, fmt.Errorf("use-template missing name attribute")
	}

	x, y := fallbackX, fallbackY
	hasExplicitPos := false
	if raw, hasX := elem.Attribute("x"); hasX {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			x = v
			absolute = true
			hasExplicitPos = true
		}
	}
	if raw, hasY := elem.Attribute("y"); hasY {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			y = v
			absolute = true
			hasExplicitPos = true
		}
	}

	tpl, err := l.TemplateByName(name)
	if err != nil {
		return 0, false, err
	}

	// width and height may come from HTML attributes or from CSS (which
	// CSSDeclarationToAttr has already baked into the element's attributes).
	_, nativeSize := tpl.Size()
	width, height := nativeSize.Wd, nativeSize.Ht
	hasExplicitSize := false
	if raw, ok := elem.Attribute("width"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			width = v
			hasExplicitSize = true
		}
	}
	if raw, ok := elem.Attribute("height"); ok {
		if v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64); e == nil {
			height = v
			hasExplicitSize = true
		}
	}

	// When no explicit position or size was given, let gofpdf place the
	// template at its own registered corner/size (UseTemplate).  Otherwise
	// use UseTemplateScaled so the caller's coordinates are respected.
	if !hasExplicitPos && !hasExplicitSize {
		l.PDF.UseTemplate(tpl)
	} else {
		l.PDF.UseTemplateScaled(
			tpl,
			gofpdf.PointType{X: x, Y: y},
			gofpdf.SizeType{Wd: width, Ht: height},
		)
	}

	return height, absolute, nil
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
