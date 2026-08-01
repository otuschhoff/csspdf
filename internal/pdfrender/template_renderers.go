package pdfrender

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/otuschhoff/go-dom2pdf/internal/pdfdom"
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
	for _, elem := range elements {
		if ShouldBreakPageBefore(elem) {
			l.NextFlowPage()
			x, y, maxW = l.CurrentFlowBox()
			currentY = y
		}

		switch n := elem.(type) {
		case *pdfdom.ElemDiv, *pdfdom.ElemH1, *pdfdom.ElemH2, *pdfdom.ElemH3:
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				metrics, err := engine.MeasureInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
				if err != nil {
					l.warnf("failed to measure doc flow element: %v", err)
					continue
				}
				if currentY > y && yPos+metrics.Height > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
				}
			}
			metrics, err := engine.RenderInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
			if err != nil {
				l.warnf("failed to render doc flow element: %v", err)
				continue
			}
			if !absolute {
				currentY = yPos + metrics.Height
			}
		case *pdfdom.ElemTable:
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			tableDef, err := l.TableDefFromElement(n, w)
			if err != nil {
				return fmt.Errorf("failed to build table definition from doc flow: %w", err)
			}
			if !absolute {
				tableHeight, err := l.tableRenderer.MeasureTableHeight(tableDef)
				if err != nil {
					return fmt.Errorf("failed to measure table from doc flow: %w", err)
				}
				if currentY > y && yPos+tableHeight > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
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
			}
		case *pdfdom.ElemImg:
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				metrics, err := engine.MeasureInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
				if err != nil {
					l.warnf("failed to measure doc image: %v", err)
					continue
				}
				if currentY > y && yPos+metrics.Height > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					xPos, yPos, w, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
				}
			}
			metrics, err := engine.RenderInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
			if err != nil {
				l.warnf("failed to render doc image: %v", err)
				continue
			}
			if !absolute {
				currentY = yPos + metrics.Height
			}
		case *pdfdom.ElemUseTemplate:
			xPos, yPos, _, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			h, _, uerr := l.RenderUseTemplateElement(n, xPos, yPos)
			if uerr != nil {
				l.warnf("failed to render use-template element: %v", uerr)
			} else if !absolute {
				currentY += h
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
		}
	}
	return nil
}
