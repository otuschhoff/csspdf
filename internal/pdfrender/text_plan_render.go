package pdfrender

import (
	"bytes"

	"github.com/otuschhoff/gofpdf"
)

func (e *PDFTextEngine) renderPlan(plan *textPlan) {
	if plan == nil {
		return
	}
	e.renderPlanBackground(plan)
	e.renderPlanBorder(plan)
	e.renderPlanImage(plan)
	e.renderPlanText(plan)
	for _, child := range plan.children {
		e.renderPlan(child)
	}
}

func (e *PDFTextEngine) renderPlanBackground(plan *textPlan) {
	if plan.backgroundColor == "" || plan.backgroundHeight <= 0 {
		return
	}
	r, g, b := hexToRGB(plan.backgroundColor)
	e.pdf.SetFillColor(r, g, b)
	width := plan.backgroundWidth
	if width <= 0 {
		width = plan.metrics.Width
	}
	if width > 0 {
		e.pdf.Rect(plan.box.X, plan.box.Y, width, plan.backgroundHeight, "F")
	}
}

func (e *PDFTextEngine) renderPlanBorder(plan *textPlan) {
	if plan.borderWidth <= 0 || plan.borderStyle == "none" || plan.backgroundHeight <= 0 {
		return
	}
	r, g, b := hexToRGB(plan.borderColor)
	e.pdf.SetDrawColor(r, g, b)
	e.pdf.SetLineWidth(plan.borderWidth)
	width := plan.backgroundWidth
	if width <= 0 {
		width = plan.metrics.Width
	}
	if width > 0 {
		e.pdf.Rect(plan.box.X, plan.box.Y, width, plan.backgroundHeight, "D")
	}
}

func (e *PDFTextEngine) renderPlanImage(plan *textPlan) {
	if plan.imagePath == "" || plan.imageWidth <= 0 || plan.imageHeight <= 0 {
		return
	}
	options := gofpdf.ImageOptions{ImageType: plan.imageType}
	if len(plan.imageData) > 0 {
		e.pdf.RegisterImageOptionsReader(plan.imagePath, options, bytes.NewReader(plan.imageData))
	}
	e.pdf.ImageOptions(plan.imagePath, plan.box.X, plan.box.Y, plan.imageWidth, plan.imageHeight, false, options, 0, "")
}

func (e *PDFTextEngine) renderPlanText(plan *textPlan) {
	e.applyStyle(plan.style)
	y := plan.box.Y + plan.style.FontSize
	for _, line := range plan.lines {
		lineWidth := e.pdf.GetStringWidth(line)
		x := alignedX(plan.style.Align, plan.box.X, plan.box.Width, lineWidth)
		e.pdf.Text(x, y, line)
		y += plan.lineHeight
	}
}
