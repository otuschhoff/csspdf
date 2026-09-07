package pdfrender

import (
	"fmt"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/gofpdf"
)

// RenderCreateTemplateElement captures an element's children in a named template.
func (l *LayoutPDF) RenderCreateTemplateElement(elem *pdfdom.ElemCreateTemplate) error {
	if elem == nil {
		return fmt.Errorf("create-template element is nil")
	}
	name, ok := elem.Attribute("name")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("create-template missing name attribute")
	}

	x := floatAttributeOrDefault(elem, "x", 0, false)
	y := floatAttributeOrDefault(elem, "y", 0, false)
	width := floatAttributeOrDefault(elem, "width", 0, false)
	height := floatAttributeOrDefault(elem, "height", 0, false)

	children := elem.ElementChildren()
	i18nInst := l.I18n
	var templateErr error
	template := l.PDF.CreateTemplateCustomNamed(
		gofpdf.PointType{X: x, Y: y},
		gofpdf.SizeType{Wd: width, Ht: height},
		name,
		func(template *gofpdf.Tpl) {
			engine := l.newTextEngine(&template.Fpdf, i18nInst)
			currentY := y
			for _, child := range children {
				if templateErr != nil {
					return
				}
				childElem, ok := child.(pdfdom.PDFElementNode)
				if !ok {
					continue
				}
				metrics, err := engine.RenderInBox(childElem, &pdfdom.PDFTextBox{
					X: x, Y: currentY, Width: width, Fit: pdfdom.TextFitWrap,
				})
				if err != nil {
					templateErr = l.recoverableRenderError("create-template %q: failed to render child", err, name)
					continue
				}
				currentY += metrics.Height
			}
		},
	)
	if templateErr != nil {
		return templateErr
	}

	if l.userTemplates == nil {
		l.userTemplates = make(map[string]gofpdf.Template)
	}
	l.userTemplates[name] = template
	return nil
}
