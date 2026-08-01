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
	// Extract and register any running-positioned footer element.
	if name, footerElem, remaining := ExtractRunningFooterElement(elements, ""); footerElem != nil {
		elements = remaining
		if err := l.SetRunningFooterTemplateFromElement(name, footerElem); err != nil {
			l.warnf("failed to register running footer %q: %v", name, err)
		}
	}

	x, y, maxW := l.CurrentFlowBox()
	engine := l.newTextEngine(l.PDF, l.I18n)

	currentY := y
	pendingBottomMargin := 0.0
	for _, elem := range elements {
		if ShouldBreakPageBefore(elem) {
			l.NextFlowPage()
			x, y, maxW = l.CurrentFlowBox()
			currentY = y
			pendingBottomMargin = 0
		}

		switch n := elem.(type) {
		case *pdfdom.ElemDiv, *pdfdom.ElemH1, *pdfdom.ElemH2, *pdfdom.ElemH3:
			topMargin, bottomMargin := flowBlockMargins(n, nil)
			collapseMargins := isVerticalMarginCollapsible(n)
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				contentTop := currentY + interElementSpacing(pendingBottomMargin, topMargin, collapseMargins)
				yPos = contentTop - topMargin
				metrics, err := engine.MeasureInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
				if err != nil {
					l.warnf("failed to measure doc flow element: %v", err)
					continue
				}
				contentHeight := metrics.Height - topMargin - bottomMargin
				if contentHeight < 0 {
					contentHeight = 0
				}
				if currentY > y && contentTop+contentHeight+bottomMargin > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					pendingBottomMargin = 0
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
					contentTop = currentY + interElementSpacing(pendingBottomMargin, topMargin, collapseMargins)
					yPos = contentTop - topMargin
				}
			}
			metrics, err := engine.RenderInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
			if err != nil {
				l.warnf("failed to render doc flow element: %v", err)
				continue
			}
			if !absolute {
				contentHeight := metrics.Height - topMargin - bottomMargin
				if contentHeight < 0 {
					contentHeight = 0
				}
				currentY = yPos + topMargin + contentHeight
				pendingBottomMargin = bottomMargin
			}
		case *pdfdom.ElemTable:
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			tableDef, err := l.TableDefFromElement(n, w)
			if err != nil {
				return fmt.Errorf("failed to build table definition from doc flow: %w", err)
			}
			tableMarginTop, tableMarginBottom := tableBlockMargins(tableDef)
			collapseMargins := isVerticalMarginCollapsible(n)
			if !absolute {
				tableHeight, err := l.tableRenderer.MeasureTableHeight(tableDef)
				if err != nil {
					return fmt.Errorf("failed to measure table from doc flow: %w", err)
				}
				yPos = currentY + interElementSpacing(pendingBottomMargin, tableMarginTop, collapseMargins)
				if currentY > y && yPos+tableHeight+tableMarginBottom > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					pendingBottomMargin = 0
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
					yPos = currentY + interElementSpacing(pendingBottomMargin, tableMarginTop, collapseMargins)
					tableDef, err = l.TableDefFromElement(n, w)
					if err != nil {
						return fmt.Errorf("failed to rebuild table definition after page break: %w", err)
					}
				}
			}
			l.PDF.SetXY(xPos, yPos)
			if err := l.tableRenderer.RenderTable(tableDef); err != nil {
				return fmt.Errorf("failed to render table from doc flow: %w", err)
			}
			if !absolute {
				currentY = l.PDF.GetY()
				pendingBottomMargin = tableMarginBottom
			}
		case *pdfdom.ElemImg:
			topMargin, bottomMargin := flowBlockMargins(n, nil)
			collapseMargins := isVerticalMarginCollapsible(n)
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				contentTop := currentY + interElementSpacing(pendingBottomMargin, topMargin, collapseMargins)
				yPos = contentTop - topMargin
				metrics, err := engine.MeasureInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
				if err != nil {
					l.warnf("failed to measure doc image: %v", err)
					continue
				}
				contentHeight := metrics.Height - topMargin - bottomMargin
				if contentHeight < 0 {
					contentHeight = 0
				}
				if currentY > y && contentTop+contentHeight+bottomMargin > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					pendingBottomMargin = 0
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
					contentTop = currentY + interElementSpacing(pendingBottomMargin, topMargin, collapseMargins)
					yPos = contentTop - topMargin
				}
			}
			metrics, err := engine.RenderInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
			if err != nil {
				l.warnf("failed to render doc image: %v", err)
				continue
			}
			if !absolute {
				contentHeight := metrics.Height - topMargin - bottomMargin
				if contentHeight < 0 {
					contentHeight = 0
				}
				currentY = yPos + topMargin + contentHeight
				pendingBottomMargin = bottomMargin
			}
		case *pdfdom.ElemUseTemplate:
			xPos, yPos, _, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			h, _, uerr := l.RenderUseTemplateElement(n, xPos, yPos)
			if uerr != nil {
				l.warnf("failed to render use-template element: %v", uerr)
			} else if !absolute {
				currentY += h
				pendingBottomMargin = 0
			}
		case *pdfdom.ElemCreateTemplate:
			if cerr := l.RenderCreateTemplateElement(n); cerr != nil {
				l.warnf("failed to create template: %v", cerr)
			}
		}

		if ShouldBreakPageAfter(elem) {
			l.NextFlowPage()
			x, y, maxW = l.CurrentFlowBox()
			currentY = y
			pendingBottomMargin = 0
		}
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
		if elem, ok := n.(pdfdom.PDFElementNode); ok {
			top = htmlLengthToFloat(elem, "marginTop", "margin-top")
			bottom = htmlLengthToFloat(elem, "marginBottom", "margin-bottom")
		}
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
