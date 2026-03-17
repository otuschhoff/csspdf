package pdfrender

import (
	"fmt"
	"os"

	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
)

// RenderDocTemplateFlow renders a document flow from pre-built PDFDOM elements.
func RenderDocTemplateFlow(l *LayoutPDF, elements []pdfdom.PDFElementNode) {
	x, y, maxW := l.CurrentFlowBox()
	engine := NewPDFTextEngine(l.PDF, l.I18n)

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
					fmt.Fprintf(os.Stderr, "Warning: failed to measure doc flow element: %v\n", err)
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
				fmt.Fprintf(os.Stderr, "Warning: failed to render doc flow element: %v\n", err)
				continue
			}
			if !absolute {
				currentY = yPos + metrics.Height
			}
		case *pdfdom.ElemTable:
			tableDef, err := l.TableDefFromElement(n)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to build table definition from doc flow: %v\n", err)
				continue
			}
			xPos, yPos, _, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				tableHeight, err := l.TableRdr.MeasureTableHeight(tableDef)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to measure table from doc flow: %v\n", err)
					continue
				}
				if currentY > y && yPos+tableHeight > l.CurrentFlowBottom() {
					l.NextFlowPage()
					x, y, maxW = l.CurrentFlowBox()
					currentY = y
					xPos, yPos, _, absolute = l.ResolveFlowPlacement(n, x, currentY, maxW)
				}
			}
			l.PDF.SetXY(xPos, yPos)
			if err := l.TableRdr.RenderTable(tableDef); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to render table from doc flow: %v\n", err)
				continue
			}
			if !absolute {
				currentY = l.PDF.GetY()
			}
		case *pdfdom.ElemImg:
			xPos, yPos, w, absolute := l.ResolveFlowPlacement(n, x, currentY, maxW)
			if !absolute {
				metrics, err := engine.MeasureInBox(n, &pdfdom.PDFTextBox{X: xPos, Y: yPos, Width: w, Fit: pdfdom.TextFitWrap})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to measure doc image: %v\n", err)
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
				fmt.Fprintf(os.Stderr, "Warning: failed to render doc image: %v\n", err)
				continue
			}
			if !absolute {
				currentY = yPos + metrics.Height
			}
		case *pdfdom.ElemUseTemplate:
			if err := l.RenderUseTemplateElement(n); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to render use-template element: %v\n", err)
			}
		}

		if ShouldBreakPageAfter(elem) {
			l.NextFlowPage()
			x, y, maxW = l.CurrentFlowBox()
			currentY = y
		}
	}
}

// RenderTimesheetTemplateFlow renders a timesheet flow from pre-built PDFDOM elements.
func RenderTimesheetTemplateFlow(l *LayoutPDF, elements []pdfdom.PDFElementNode) {
	x, y, maxW := l.CurrentFlowBox()
	engine := NewPDFTextEngine(l.PDF, l.I18n)

	currentY := y
	for _, elem := range elements {
		switch n := elem.(type) {
		case *pdfdom.ElemDiv, *pdfdom.ElemH1, *pdfdom.ElemH2, *pdfdom.ElemH3:
			metrics, err := engine.RenderInBox(n, &pdfdom.PDFTextBox{X: x, Y: currentY, Width: maxW, Fit: pdfdom.TextFitWrap})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to render timesheet text flow element: %v\n", err)
				continue
			}
			currentY += metrics.Height
		case *pdfdom.ElemTable:
			tableDef, err := l.TableDefFromElement(n)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to build timesheet table definition from flow: %v\n", err)
				continue
			}
			l.PDF.SetXY(x, currentY)
			if err := l.TableRdr.RenderTable(tableDef); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to render timesheet table from flow: %v\n", err)
				continue
			}
			currentY = l.PDF.GetY()
		case *pdfdom.ElemUseTemplate:
			if err := l.RenderUseTemplateElement(n); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to render use-template element: %v\n", err)
			}
		}
	}
}
