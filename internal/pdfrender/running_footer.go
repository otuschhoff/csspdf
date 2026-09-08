package pdfrender

import (
	"fmt"
	"strings"

	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/gofpdf"
)

func (l *LayoutPDF) SetRunningFooterTemplateFromElement(name string, footerElem pdfdom.PDFElementNode) error {
	if footerElem == nil {
		return fmt.Errorf("running footer element is nil")
	}
	width := floatAttributeOrDefault(footerElem, "width", l.pageWidth, true)
	height := floatAttributeOrDefault(footerElem, "height", 52, true)
	safeName := strings.ToUpper(strings.TrimSpace(name))
	if safeName == "" {
		safeName = "RUNNING-FOOTER"
	} else {
		safeName = "RUNNING-" + safeName
	}
	var templateErr error
	template := l.PDF.CreateTemplateCustomNamed(
		gofpdf.PointType{}, gofpdf.SizeType{Wd: width, Ht: height}, safeName,
		func(t *gofpdf.Tpl) {
			templateErr = l.renderRunningFooterChildren(t, footerElem.ElementChildren(), width, name, l.I18n)
		},
	)
	if templateErr != nil {
		return templateErr
	}
	l.runningFooterTemplate = template
	l.runningFooterSize = gofpdf.SizeType{Wd: width, Ht: height}
	l.renderRunningFooterTemplate()
	return nil
}

func (l *LayoutPDF) renderRunningFooterChildren(t *gofpdf.Tpl, children []pdfdom.PDFNode, width float64, name string, translations *i18n.I18n) error {
	engine := l.newTextEngine(&t.Fpdf, translations)
	currentY := 0.0
	for _, child := range children {
		element, ok := child.(pdfdom.PDFElementNode)
		if !ok {
			continue
		}
		x, y, childWidth, absolute := resolveFlowPlacementWith(element, 0, currentY, width, width)
		if useTemplate, ok := element.(*pdfdom.ElemUseTemplate); ok {
			height, err := l.renderRunningFooterUseTemplate(t, useTemplate, x, y, absolute, name)
			if err != nil {
				return err
			}
			if !absolute {
				currentY += height
			}
			continue
		}
		metrics, err := engine.RenderInBox(element, &pdfdom.PDFTextBox{X: x, Y: y, Width: childWidth, Fit: pdfdom.TextFitWrap})
		if err != nil {
			return fmt.Errorf("running footer %q: failed to render child: %w", name, err)
		}
		if !absolute {
			currentY += metrics.Height
		}
	}
	return nil
}

func (l *LayoutPDF) renderRunningFooterUseTemplate(t *gofpdf.Tpl, element *pdfdom.ElemUseTemplate, x, y float64, absolute bool, footerName string) (float64, error) {
	name, ok := element.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return 0, nil
	}
	template, err := l.TemplateByName(name)
	if err != nil {
		return 0, fmt.Errorf("running footer %q: %w", footerName, err)
	}
	_, nativeSize := template.Size()
	width := floatAttributeOrDefault(element, "width", nativeSize.Wd, true)
	height := floatAttributeOrDefault(element, "height", nativeSize.Ht, true)
	if !absolute && width == nativeSize.Wd && height == nativeSize.Ht {
		t.UseTemplate(template)
	} else {
		t.UseTemplateScaled(template, gofpdf.PointType{X: x, Y: y}, gofpdf.SizeType{Wd: width, Ht: height})
	}
	return height, nil
}
