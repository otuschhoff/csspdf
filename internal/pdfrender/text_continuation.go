package pdfrender

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/gofpdf"
)

type verticalBand struct {
	top    float64
	bottom float64
}

func (state *docFlowState) renderBlockContinuation(node pdfdom.PDFElementNode, box pdfdom.PDFTextBox, bottomMargin float64) error {
	plan, metrics, err := state.engine.layoutNode(node, state.engine.defaultStyle, resolveBox(&box))
	if err != nil {
		return err
	}
	bands := mergedPlanBands(plan, box.Y)
	if len(bands) == 0 {
		return fmt.Errorf("oversized %s block has no splittable text or image content", node.ElementType())
	}

	start := 0.0
	targetY := box.Y
	for {
		available := state.layout.CurrentFlowBottom() - targetY
		end, next, done, ok := nextPlanWindow(bands, start, available, metrics.Height)
		if !ok {
			return fmt.Errorf("%s content at offset %.2f exceeds available page height %.2f", node.ElementType(), start, available)
		}
		state.engine.renderPlanWindow(plan, box.Y, start, end, targetY)
		if done {
			state.currentY = targetY + (metrics.Height - start) - bottomMargin
			state.pendingBottomMargin = bottomMargin
			return nil
		}
		start = next
		if err := state.nextPage(); err != nil {
			return err
		}
		targetY = state.y
	}
}

func mergedPlanBands(plan *textPlan, originY float64) []verticalBand {
	bands := make([]verticalBand, 0)
	collectPlanBands(plan, originY, &bands)
	sort.Slice(bands, func(i, j int) bool {
		if bands[i].top == bands[j].top {
			return bands[i].bottom < bands[j].bottom
		}
		return bands[i].top < bands[j].top
	})
	merged := make([]verticalBand, 0, len(bands))
	for _, band := range bands {
		if len(merged) == 0 || band.top >= merged[len(merged)-1].bottom {
			merged = append(merged, band)
			continue
		}
		if band.bottom > merged[len(merged)-1].bottom {
			merged[len(merged)-1].bottom = band.bottom
		}
	}
	return merged
}

func collectPlanBands(plan *textPlan, originY float64, bands *[]verticalBand) {
	if plan == nil {
		return
	}
	for line := range plan.lines {
		top := plan.box.Y - originY + float64(line)*plan.lineHeight
		*bands = append(*bands, verticalBand{top: top, bottom: top + plan.lineHeight})
	}
	if plan.imagePath != "" && plan.imageHeight > 0 {
		top := plan.box.Y - originY
		*bands = append(*bands, verticalBand{top: top, bottom: top + plan.imageHeight})
	}
	for _, child := range plan.children {
		collectPlanBands(child, originY, bands)
	}
}

func nextPlanWindow(bands []verticalBand, start, available, totalHeight float64) (end, next float64, done, ok bool) {
	limit := start + available
	first := sort.Search(len(bands), func(index int) bool { return bands[index].bottom > start })
	if first == len(bands) || bands[first].bottom > limit {
		return 0, 0, false, false
	}
	last := first
	for last+1 < len(bands) && bands[last+1].bottom <= limit {
		last++
	}
	end = bands[last].bottom
	if last+1 < len(bands) {
		return end, bands[last+1].top, false, true
	}
	if totalHeight-start > available {
		if last > first {
			return bands[last-1].bottom, bands[last].top, false, true
		}
		return 0, 0, false, false
	}
	return totalHeight, totalHeight, true, true
}

func (e *PDFTextEngine) renderPlanWindow(plan *textPlan, originY, start, end, targetY float64) {
	if plan == nil {
		return
	}
	shiftY := targetY - originY - start
	windowTop := originY + start
	windowBottom := originY + end
	e.renderPlanWindowDecoration(plan, windowTop, windowBottom, shiftY)

	if plan.imagePath != "" && plan.imageHeight > 0 && plan.box.Y >= windowTop && plan.box.Y+plan.imageHeight <= windowBottom {
		if len(plan.imageData) > 0 {
			e.pdf.RegisterImageOptionsReader(plan.imagePath, gofpdf.ImageOptions{ImageType: plan.imageType}, bytes.NewReader(plan.imageData))
		}
		e.pdf.ImageOptions(plan.imagePath, plan.box.X, plan.box.Y+shiftY, plan.imageWidth, plan.imageHeight, false, gofpdf.ImageOptions{ImageType: plan.imageType}, 0, "")
	}

	e.applyStyle(plan.style)
	for line, text := range plan.lines {
		top := plan.box.Y + float64(line)*plan.lineHeight
		if top < windowTop || top+plan.lineHeight > windowBottom {
			continue
		}
		lineWidth := e.pdf.GetStringWidth(text)
		x := alignedX(plan.style.Align, plan.box.X, plan.box.Width, lineWidth)
		e.pdf.Text(x, top+shiftY+plan.style.FontSize, text)
	}
	for _, child := range plan.children {
		e.renderPlanWindow(child, originY, start, end, targetY)
	}
}

func (e *PDFTextEngine) renderPlanWindowDecoration(plan *textPlan, windowTop, windowBottom, shiftY float64) {
	if plan.backgroundColor != "" && plan.backgroundHeight > 0 {
		top := max(plan.box.Y, windowTop)
		bottom := min(plan.box.Y+plan.backgroundHeight, windowBottom)
		if bottom > top {
			r, g, b := hexToRGB(plan.backgroundColor)
			e.pdf.SetFillColor(r, g, b)
			width := plan.backgroundWidth
			if width <= 0 {
				width = plan.metrics.Width
			}
			e.pdf.Rect(plan.box.X, top+shiftY, width, bottom-top, "F")
		}
	}
	if plan.borderWidth > 0 && plan.borderStyle != "none" && plan.backgroundHeight > 0 {
		top := max(plan.box.Y, windowTop)
		bottom := min(plan.box.Y+plan.backgroundHeight, windowBottom)
		width := plan.backgroundWidth
		if width <= 0 {
			width = plan.metrics.Width
		}
		if bottom > top && width > 0 {
			r, g, b := hexToRGB(plan.borderColor)
			e.pdf.SetDrawColor(r, g, b)
			e.pdf.SetLineWidth(plan.borderWidth)
			e.pdf.Rect(plan.box.X, top+shiftY, width, bottom-top, "D")
		}
	}
}
