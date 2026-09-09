package pdfrender

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

var runningPositionRe = regexp.MustCompile(`^running\(\s*([a-z0-9_-]+)\s*\)$`)

// ExtractRunningFooterElement finds the first element with
// position="running(name)" and returns the matched name, the element, and a
// filtered slice without it. When name is empty, the first element with any
// position="running(...)" value is matched. Returns ("", nil, elements) when
// no match is found.
func ExtractRunningFooterElement(elements []pdfdom.PDFElementNode, name string) (matchedName string, elem pdfdom.PDFElementNode, remaining []pdfdom.PDFElementNode) {
	if len(elements) == 0 {
		return "", nil, elements
	}
	want := strings.ToLower(strings.TrimSpace(name))
	filtered := make([]pdfdom.PDFElementNode, 0, len(elements))
	var footer pdfdom.PDFElementNode
	var foundName string

	for _, el := range elements {
		if footer == nil {
			if raw, ok := el.Attribute("position"); ok {
				match := runningPositionRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(raw)))
				if len(match) == 2 && (want == "" || match[1] == want) {
					foundName = match[1]
					footer = el
					continue
				}
			}
		}
		filtered = append(filtered, el)
	}

	if footer == nil {
		return "", nil, elements
	}
	return foundName, footer, filtered
}

// RenderDocTemplateFlow renders a document flow from pre-built PDFDOM elements.
// Elements with CSS position:running(name) are extracted, registered as
// running footer templates, and stamped on every page via BeginPage.
func RenderDocTemplateFlow(l *LayoutPDF, elements []pdfdom.PDFElementNode) error {
	return renderDocTemplateFlow(l, elements, true)
}

// RenderDocTemplateOverlay renders page-local content without advancing the
// persistent normal-flow cursor.
func RenderDocTemplateOverlay(l *LayoutPDF, elements []pdfdom.PDFElementNode) error {
	return renderDocTemplateFlow(l, elements, false)
}

func renderDocTemplateFlow(l *LayoutPDF, elements []pdfdom.PDFElementNode, persistCursor bool) error {
	if name, footerElem, remaining := ExtractRunningFooterElement(elements, ""); footerElem != nil {
		elements = remaining
		if err := l.SetRunningFooterTemplateFromElement(name, footerElem); err != nil {
			if err := l.recoverableRenderError("failed to register running footer %q", err, name); err != nil {
				return err
			}
		}
	}

	state := newDocFlowState(l, persistCursor)
	for _, elem := range elements {
		if err := l.ctx.Err(); err != nil {
			return fmt.Errorf("layout canceled: %w", err)
		}
		if err := state.applyPageBreak(ShouldBreakPageBefore(elem), "before"); err != nil {
			return err
		}
		skipPageAfter, err := state.renderElement(elem)
		if err != nil {
			return err
		}
		if skipPageAfter {
			continue
		}

		if err := state.applyPageBreak(ShouldBreakPageAfter(elem), "after"); err != nil {
			return err
		}
	}
	state.persist()
	return nil
}

func (state *docFlowState) applyPageBreak(required bool, position string) error {
	if !required {
		return nil
	}
	if !state.persistCursor {
		return fmt.Errorf("page-local overlay cannot contain break-%s: page", position)
	}
	return state.nextPage()
}

type docFlowState struct {
	layout              *LayoutPDF
	engine              *PDFTextEngine
	x, y, maxWidth      float64
	currentY            float64
	pendingBottomMargin float64
	persistCursor       bool
}

func newDocFlowState(layout *LayoutPDF, persistCursor bool) *docFlowState {
	x, y, maxWidth := layout.CurrentFlowBox()
	currentY := y
	pendingBottomMargin := 0.0
	if persistCursor && layout.flowCursorValid {
		currentY = layout.flowCursorY
		pendingBottomMargin = layout.flowBottomMargin
	}
	return &docFlowState{
		layout:              layout,
		engine:              layout.newTextEngine(layout.PDF, layout.I18n),
		x:                   x,
		y:                   y,
		maxWidth:            maxWidth,
		currentY:            currentY,
		pendingBottomMargin: pendingBottomMargin,
		persistCursor:       persistCursor,
	}
}

func (state *docFlowState) persist() {
	if !state.persistCursor {
		return
	}
	state.layout.flowCursorY = state.currentY
	state.layout.flowBottomMargin = state.pendingBottomMargin
	state.layout.flowCursorValid = true
}

func (state *docFlowState) nextPage() error {
	if err := state.layout.NextFlowPage(); err != nil {
		return err
	}
	state.x, state.y, state.maxWidth = state.layout.CurrentFlowBox()
	state.currentY = state.y
	state.pendingBottomMargin = 0
	return nil
}

func (state *docFlowState) renderElement(elem pdfdom.PDFElementNode) (bool, error) {
	switch node := elem.(type) {
	case *pdfdom.ElemDiv, *pdfdom.ElemH1, *pdfdom.ElemH2, *pdfdom.ElemH3:
		return state.renderBlock(node)
	case *pdfdom.ElemTable:
		return false, state.renderTable(node)
	case *pdfdom.ElemImg:
		return state.renderImage(node)
	case *pdfdom.ElemUseTemplate:
		return false, state.renderUseTemplate(node)
	case *pdfdom.ElemCreateTemplate:
		return false, state.renderCreateTemplate(node)
	default:
		return false, nil
	}
}

func (state *docFlowState) renderBlock(node pdfdom.PDFElementNode) (bool, error) {
	topMargin, bottomMargin := flowBlockMargins(node, nil)
	collapseMargins := isVerticalMarginCollapsible(node)
	xPos, yPos, width, absolute, err := state.prepareBlockPlacement(node, topMargin, bottomMargin, collapseMargins)
	if err != nil {
		return false, err
	}
	box := pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: width, Fit: pdfdom.TextFitWrap}
	measured, err := state.engine.MeasureInBox(node, &box)
	if err != nil {
		return true, state.layout.recoverableRenderError("failed to measure doc flow element", err)
	}
	if !absolute {
		if measured.Height > state.layout.CurrentFlowBottom()-yPos {
			if !state.persistCursor {
				return false, validateWholeBlockHeight("page-local overlay", measured.Height, state.layout.CurrentFlowBottom()-yPos)
			}
			return false, state.renderBlockContinuation(node, box, bottomMargin)
		}
	} else if err := validateAbsoluteElementBounds(node.ElementType(), xPos, yPos, measured.Width, measured.Height, state.layout.pageWidth, state.layout.pageHeight); err != nil {
		return false, err
	}
	metrics, err := state.engine.RenderInBox(node, &box)
	if err != nil {
		return true, state.layout.recoverableRenderError("failed to render doc flow element", err)
	}
	if !absolute {
		state.currentY = yPos + topMargin + max(0, metrics.Height-topMargin-bottomMargin)
		state.pendingBottomMargin = bottomMargin
	}
	return false, nil
}

func (state *docFlowState) prepareBlockPlacement(node pdfdom.PDFElementNode, topMargin, bottomMargin float64, collapseMargins bool) (float64, float64, float64, bool, error) {
	xPos, yPos, width, absolute := state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	if absolute {
		return xPos, yPos, width, true, nil
	}
	contentTop := state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, collapseMargins)
	yPos = contentTop - topMargin
	metrics, err := state.engine.MeasureInBox(node, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: width, Fit: pdfdom.TextFitWrap})
	if err != nil {
		return 0, 0, 0, false, state.layout.recoverableRenderError("failed to measure doc flow element", err)
	}
	contentHeight := max(0, metrics.Height-topMargin-bottomMargin)
	if state.currentY <= state.y || contentTop+contentHeight+bottomMargin <= state.layout.CurrentFlowBottom() {
		return xPos, yPos, width, false, nil
	}
	if err := state.nextPage(); err != nil {
		return 0, 0, 0, false, err
	}
	xPos, _, width, absolute = state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	contentTop = state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, collapseMargins)
	return xPos, contentTop - topMargin, width, absolute, nil
}

func (state *docFlowState) renderImage(node *pdfdom.ElemImg) (bool, error) {
	topMargin, bottomMargin := flowBlockMargins(node, nil)
	xPos, yPos, width, absolute := state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	if !absolute {
		contentTop := state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, false)
		yPos = contentTop - topMargin
		metrics, err := state.engine.MeasureInBox(node, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: width, Fit: pdfdom.TextFitWrap})
		if err != nil {
			return true, state.layout.recoverableRenderError("failed to measure doc image", err)
		}
		contentHeight := max(0, metrics.Height-topMargin-bottomMargin)
		if state.currentY > state.y && contentTop+contentHeight+bottomMargin > state.layout.CurrentFlowBottom() {
			if err := state.nextPage(); err != nil {
				return false, err
			}
			xPos, _, width, absolute = state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
			contentTop = state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, false)
			yPos = contentTop - topMargin
		}
	}
	measured, err := state.engine.MeasureInBox(node, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: width, Fit: pdfdom.TextFitWrap})
	if err != nil {
		return true, state.layout.recoverableRenderError("failed to measure doc image", err)
	}
	if absolute {
		if err := validateAbsoluteElementBounds("image", xPos, yPos, measured.Width, measured.Height, state.layout.pageWidth, state.layout.pageHeight); err != nil {
			return false, err
		}
	} else if err := validateWholeBlockHeight("image", measured.Height, state.layout.CurrentFlowBottom()-yPos); err != nil {
		return false, err
	}
	metrics, err := state.engine.RenderInBox(node, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: width, Fit: pdfdom.TextFitWrap})
	if err != nil {
		return true, state.layout.recoverableRenderError("failed to render doc image", err)
	}
	if !absolute {
		state.currentY = yPos + topMargin + max(0, metrics.Height-topMargin-bottomMargin)
		state.pendingBottomMargin = bottomMargin
	}
	return false, nil
}

func (state *docFlowState) renderTable(node *pdfdom.ElemTable) error {
	xPos, yPos, width, absolute := state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	tableDef, err := state.layout.TableDefFromElement(node, width)
	if err != nil {
		return fmt.Errorf("failed to build table definition from doc flow: %w", err)
	}
	topMargin, bottomMargin := tableBlockMargins(tableDef)
	if !absolute {
		yPos = state.currentY + interElementSpacing(state.pendingBottomMargin, topMargin, isVerticalMarginCollapsible(node))
		if !state.persistCursor {
			tableHeight, err := state.layout.tableRenderer.MeasureTableHeight(tableDef)
			if err != nil {
				return fmt.Errorf("failed to measure page-local table: %w", err)
			}
			if err := validateWholeBlockHeight("page-local table", tableHeight+bottomMargin, state.layout.CurrentFlowBottom()-yPos); err != nil {
				return err
			}
			state.layout.PDF.SetXY(xPos, yPos)
			return state.layout.tableRenderer.RenderTable(tableDef)
		}
		return state.renderPaginatedTable(node, tableDef, xPos, yPos)
	}
	tableHeight, err := state.layout.tableRenderer.MeasureTableHeight(tableDef)
	if err != nil {
		return fmt.Errorf("failed to measure absolute table: %w", err)
	}
	if err := validateAbsoluteElementBounds("table", xPos, yPos, width, tableHeight, state.layout.pageWidth, state.layout.pageHeight); err != nil {
		return err
	}
	state.layout.PDF.SetXY(xPos, yPos)
	if err := state.layout.tableRenderer.RenderTable(tableDef); err != nil {
		return fmt.Errorf("failed to render table from doc flow: %w", err)
	}
	return nil
}

func (state *docFlowState) renderUseTemplate(node *pdfdom.ElemUseTemplate) error {
	xPos, yPos, _, absolute := state.layout.ResolveFlowPlacement(node, state.x, state.currentY, state.maxWidth)
	height, _, err := state.layout.RenderUseTemplateElement(node, xPos, yPos)
	if err != nil {
		return state.layout.recoverableRenderError("failed to render use-template element", err)
	}
	if !absolute {
		state.currentY += height
		state.pendingBottomMargin = 0
	}
	return nil
}

func (state *docFlowState) renderCreateTemplate(node *pdfdom.ElemCreateTemplate) error {
	if err := state.layout.RenderCreateTemplateElement(node); err != nil {
		return state.layout.recoverableRenderError("failed to create template", err)
	}
	return nil
}

func interElementSpacing(previousBottom, currentTop float64, collapse bool) float64 {
	if !collapse {
		return previousBottom + currentTop
	}
	if previousBottom > currentTop {
		return previousBottom
	}
	return currentTop
}

func flowBlockMargins(node pdfdom.PDFElementNode, table *TableDef) (top, bottom float64) {
	if node == nil {
		return 0, 0
	}
	switch n := node.(type) {
	case *pdfdom.ElemDiv:
		top = htmlLengthToFloat(n, "marginTop", "margin-top")
		bottom = htmlLengthToFloat(n, "marginBottom", "margin-bottom")
	case *pdfdom.ElemH1:
		top = headingDefaultMargin(n, true)
		bottom = headingDefaultMargin(n, false)
		if _, ok := n.Attribute("marginTop"); ok {
			top = htmlLengthToFloat(n, "marginTop", "margin-top")
		} else if _, ok := n.Attribute("margin-top"); ok {
			top = htmlLengthToFloat(n, "marginTop", "margin-top")
		}
		if _, ok := n.Attribute("marginBottom"); ok {
			bottom = htmlLengthToFloat(n, "marginBottom", "margin-bottom")
		} else if _, ok := n.Attribute("margin-bottom"); ok {
			bottom = htmlLengthToFloat(n, "marginBottom", "margin-bottom")
		}
	case *pdfdom.ElemH2, *pdfdom.ElemH3:
		top = htmlLengthToFloat(n, "marginTop", "margin-top")
		bottom = htmlLengthToFloat(n, "marginBottom", "margin-bottom")
	case *pdfdom.ElemImg:
		top = htmlLengthToFloat(n, "marginTop", "margin-top")
		bottom = htmlLengthToFloat(n, "marginBottom", "margin-bottom")
	case *pdfdom.ElemTable:
		if table != nil {
			top, bottom = tableBlockMargins(table)
		}
	}
	return top, bottom
}

func headingDefaultMargin(node pdfdom.PDFElementNode, top bool) float64 {
	if node == nil {
		return 0
	}
	style := node.ElementStyle()
	if style == nil || style.FontSize <= 0 {
		return 0
	}
	return style.FontSize * 0.67
}

func isVerticalMarginCollapsible(node pdfdom.PDFElementNode) bool {
	if node == nil {
		return false
	}
	switch node.(type) {
	case *pdfdom.ElemDiv, *pdfdom.ElemH1, *pdfdom.ElemH2, *pdfdom.ElemH3, *pdfdom.ElemTable:
		return true
	default:
		return false
	}
}

func tableBlockMargins(table *TableDef) (top, bottom float64) {
	if table == nil {
		return 0, 0
	}
	top = table.MarginTop
	if !table.MarginTopSet {
		top = defaultTableFontSize
	}
	bottom = table.MarginBottom
	if !table.MarginBottomSet {
		bottom = defaultTableFontSize
	}
	return top, bottom
}
