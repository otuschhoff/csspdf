package pdfrender

import (
	"fmt"
	"sort"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/gofpdf"
)

type Translator = pdfdom.Translator
type PDFTextStyle = pdfdom.PDFTextStyle
type PDFTextBox = pdfdom.PDFTextBox
type TextAlign = pdfdom.TextAlign

const (
	TextAlignLeft   = pdfdom.TextAlignLeft
	TextAlignCenter = pdfdom.TextAlignCenter
	TextAlignRight  = pdfdom.TextAlignRight
	TextFitClip     = pdfdom.TextFitClip
	TextFitWrap     = pdfdom.TextFitWrap
)

type PDFNode = pdfdom.PDFNode
type PDFDocumentNode = pdfdom.PDFDocumentNode
type PDFElementNode = pdfdom.PDFElementNode
type PDFTextNode = pdfdom.PDFTextNode
type ElemBr = pdfdom.ElemBr
type ElemImg = pdfdom.ElemImg

// PDFTextMetrics contains calculated dimensions and rendering metadata.
type PDFTextMetrics struct {
	Width      float64
	Height     float64
	LineCount  int
	WasClipped bool
}

// FontGlyphRegistry tracks used glyphs per font for later subsetting.
type FontGlyphRegistry struct {
	used map[string]map[rune]struct{}
}

func NewFontGlyphRegistry() *FontGlyphRegistry {
	return &FontGlyphRegistry{used: make(map[string]map[rune]struct{})}
}

func (g *FontGlyphRegistry) Record(fontFace, text string) {
	if g == nil || fontFace == "" || text == "" {
		return
	}
	if _, ok := g.used[fontFace]; !ok {
		g.used[fontFace] = make(map[rune]struct{})
	}
	for _, r := range text {
		g.used[fontFace][r] = struct{}{}
	}
}

func (g *FontGlyphRegistry) UsedGlyphs(fontFace string) []rune {
	if g == nil || g.used[fontFace] == nil {
		return nil
	}
	out := make([]rune, 0, len(g.used[fontFace]))
	for r := range g.used[fontFace] {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (g *FontGlyphRegistry) Snapshot() map[string][]rune {
	out := make(map[string][]rune)
	if g == nil {
		return out
	}
	for font := range g.used {
		out[font] = g.UsedGlyphs(font)
	}
	return out
}

// PDFTextEngine hides low-level PDF calls and renders declarative PDFNode trees.
type PDFTextEngine struct {
	pdf             *gofpdf.Fpdf
	i18n            Translator
	formatter       pdfdom.ValueFormatter
	defaultStyle    PDFTextStyle
	glyphs          *FontGlyphRegistry
	imageSearchDirs []string
	imageLoader     ImageLoader
}

func NewPDFTextEngine(pdf *gofpdf.Fpdf, i18n Translator) *PDFTextEngine {
	return &PDFTextEngine{
		pdf:  pdf,
		i18n: i18n,
		defaultStyle: PDFTextStyle{
			FontFace:   "Helvetica",
			FontSize:   10,
			FontColor:  "#000",
			Align:      TextAlignLeft,
			LineHeight: 1.2,
		},
		glyphs: NewFontGlyphRegistry(),
	}
}

func (e *PDFTextEngine) SetImageSearchDirs(paths []string) {
	if e == nil {
		return
	}
	e.imageSearchDirs = append([]string(nil), paths...)
}

func (e *PDFTextEngine) SetImageLoader(loader ImageLoader) {
	e.imageLoader = loader
}

func (e *PDFTextEngine) SetDefaultStyle(style PDFTextStyle) {
	e.defaultStyle = ensureTextStyleDefaults(style)
}

func (e *PDFTextEngine) SetValueFormatter(formatter pdfdom.ValueFormatter) {
	e.formatter = formatter
}

func (e *PDFTextEngine) GlyphRegistry() *FontGlyphRegistry {
	return e.glyphs
}

func (e *PDFTextEngine) Measure(node PDFNode) (PDFTextMetrics, error) {
	return e.MeasureInBox(node, nil)
}

func (e *PDFTextEngine) MeasureInBox(node PDFNode, box *PDFTextBox) (PDFTextMetrics, error) {
	if e == nil || e.pdf == nil {
		return PDFTextMetrics{}, fmt.Errorf("pdf text engine is not initialized")
	}
	if node == nil {
		return PDFTextMetrics{}, fmt.Errorf("node cannot be nil")
	}
	_, metrics, err := e.layoutNode(node, e.defaultStyle, resolveBox(box))
	return metrics, err
}

func (e *PDFTextEngine) Render(node PDFNode) (PDFTextMetrics, error) {
	return e.RenderInBox(node, nil)
}

func (e *PDFTextEngine) RenderInBox(node PDFNode, box *PDFTextBox) (PDFTextMetrics, error) {
	if e == nil || e.pdf == nil {
		return PDFTextMetrics{}, fmt.Errorf("pdf text engine is not initialized")
	}
	if node == nil {
		return PDFTextMetrics{}, fmt.Errorf("node cannot be nil")
	}
	planned, metrics, err := e.layoutNode(node, e.defaultStyle, resolveBox(box))
	if err != nil {
		return PDFTextMetrics{}, err
	}
	e.renderPlan(planned)
	return metrics, nil
}

type textPlan struct {
	style            PDFTextStyle
	box              PDFTextBox
	lines            []string
	lineHeight       float64
	metrics          PDFTextMetrics
	children         []*textPlan
	imagePath        string
	imageData        []byte
	imageType        string
	imageWidth       float64
	imageHeight      float64
	backgroundColor  string
	backgroundHeight float64
	backgroundWidth  float64
	borderColor      string
	borderStyle      string
	borderWidth      float64
}

func (e *PDFTextEngine) layoutNode(node PDFNode, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	switch n := node.(type) {
	case *PDFDocumentNode:
		return e.layoutContainerNode(n.Style, n.Children, n.ChildLineBreaks, parentStyle, box)
	case *ElemBr:
		return &textPlan{
			style:      parentStyle,
			box:        box,
			lines:      nil,
			lineHeight: parentStyle.FontSize * parentStyle.LineHeight,
			metrics:    PDFTextMetrics{},
			children:   []*textPlan{},
		}, PDFTextMetrics{}, nil
	case *ElemImg:
		return e.layoutImageNode(n, parentStyle, box)
	case *pdfdom.ElemCurrencyValue:
		return e.layoutValueNode(n.Format(e.formatter), n.ElementStyle(), parentStyle, box)
	case *pdfdom.ElemDateValue:
		return e.layoutValueNode(n.Format(e.formatter), n.ElementStyle(), parentStyle, box)
	case *pdfdom.ElemDurationValue:
		return e.layoutValueNode(n.Format(e.formatter), n.ElementStyle(), parentStyle, box)
	case *pdfdom.ElemManDaysValue:
		return e.layoutValueNode(n.Format(e.formatter), n.ElementStyle(), parentStyle, box)
	case PDFElementNode:
		if strings.TrimSpace(n.ElementType()) == "" {
			return nil, PDFTextMetrics{}, fmt.Errorf("PDFElementNode type must be set")
		}
		return e.layoutElementNode(n, parentStyle, box)
	case *PDFTextNode:
		return e.layoutTextNode(n, parentStyle, box)
	default:
		return nil, PDFTextMetrics{}, fmt.Errorf("unsupported PDF node type")
	}
}

func (e *PDFTextEngine) layoutValueNode(text string, nodeStyle *PDFTextStyle, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	return e.layoutTextNode(&PDFTextNode{Text: text, Style: nodeStyle}, parentStyle, box)
}

func (e *PDFTextEngine) layoutContainerNode(nodeStyle *PDFTextStyle, children []PDFNode, childLineBreaks []bool, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	style := parentStyle.Merge(nodeStyle)
	lineHeight := style.FontSize * style.LineHeight

	plan := &textPlan{
		style:      style,
		box:        box,
		lines:      nil,
		lineHeight: lineHeight,
		metrics:    PDFTextMetrics{},
		children:   []*textPlan{},
	}

	metrics, err := e.layoutChildren(plan, style, box, children, childLineBreaks, 0, 0, 0, 0, false)
	if err != nil {
		return nil, PDFTextMetrics{}, err
	}
	plan.metrics = metrics
	return plan, metrics, nil
}

func (e *PDFTextEngine) layoutImageNode(node *ElemImg, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	style := parentStyle.Merge(node.ElementStyle())
	path, width, height, marginTop, marginBottom, err := resolveImageElement(node, box.Width, e.imageSearchDirs, e.imageLoader != nil)
	if err != nil {
		return nil, PDFTextMetrics{}, err
	}
	imageType := imageTypeFromPath(path)
	var imageData []byte
	if e.imageLoader != nil {
		resource, loadErr := e.imageLoader(path)
		if loadErr != nil {
			return nil, PDFTextMetrics{}, loadErr
		}
		path = resource.Name
		imageType = resource.Type
		imageData = resource.Data
	}

	align := TextAlignLeft
	if raw, ok := node.Attribute("align"); ok {
		align = htmlNormaliseTextAlign(raw)
	}
	x := alignedX(align, box.X, box.Width, width)
	y := box.Y + marginTop

	plan := &textPlan{
		style:       style,
		box:         PDFTextBox{X: x, Y: y, Width: width, Height: height, Fit: box.Fit},
		lineHeight:  style.FontSize * style.LineHeight,
		metrics:     PDFTextMetrics{Width: width, Height: marginTop + height + marginBottom, LineCount: 1},
		children:    []*textPlan{},
		imagePath:   path,
		imageData:   imageData,
		imageType:   imageType,
		imageWidth:  width,
		imageHeight: height,
	}
	return plan, plan.metrics, nil
}

func (e *PDFTextEngine) layoutElementNode(node PDFElementNode, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	style := parentStyle.Merge(node.ElementStyle())
	spacing := elementSpacingFor(node, style)
	outerBox := box
	outerBox.Y += spacing.marginTop
	if outerBox.Height > 0 {
		outerBox.Height = pdfMax(0, outerBox.Height-spacing.marginTop-spacing.marginBottom)
	}
	innerBox := spacing.inset(outerBox)
	plan, contentMetrics, err := e.layoutContainerNode(node.ElementStyle(), node.ElementChildren(), node.ElementChildLineBreaks(), parentStyle, innerBox)
	if err != nil {
		return nil, PDFTextMetrics{}, err
	}

	applyElementDecoration(plan, node)
	plan.box = outerBox
	plan.backgroundWidth = outerBox.Width
	if plan.backgroundWidth <= 0 {
		plan.backgroundWidth = contentMetrics.Width + spacing.paddingLeft + spacing.paddingRight
	}
	plan.backgroundHeight = contentMetrics.Height + spacing.paddingTop + spacing.paddingBottom
	totalMetrics := contentMetrics
	totalMetrics.Width += spacing.paddingLeft + spacing.paddingRight
	totalMetrics.Height += spacing.marginTop + spacing.marginBottom + spacing.paddingTop + spacing.paddingBottom
	return plan, totalMetrics, nil
}

type elementSpacing struct{ marginTop, marginBottom, paddingTop, paddingRight, paddingBottom, paddingLeft float64 }

func elementSpacingFor(node PDFElementNode, style PDFTextStyle) elementSpacing {
	spacing := elementSpacing{}
	if node.ElementType() == "div" {
		spacing.marginTop = htmlLengthToFloat(node, "marginTop", "margin-top")
		spacing.marginBottom = htmlLengthToFloat(node, "marginBottom", "margin-bottom")
		padding := htmlLengthToFloat(node, "padding")
		spacing.paddingTop, spacing.paddingRight, spacing.paddingBottom, spacing.paddingLeft = padding, padding, padding, padding
		if value := htmlLengthToFloat(node, "paddingTop", "padding-top"); value > 0 {
			spacing.paddingTop = value
		}
		if value := htmlLengthToFloat(node, "paddingRight", "padding-right"); value > 0 {
			spacing.paddingRight = value
		}
		if value := htmlLengthToFloat(node, "paddingBottom", "padding-bottom"); value > 0 {
			spacing.paddingBottom = value
		}
		if value := htmlLengthToFloat(node, "paddingLeft", "padding-left"); value > 0 {
			spacing.paddingLeft = value
		}
	}
	if node.ElementType() == "h1" {
		spacing.marginTop = elementMargin(node, "marginTop", "margin-top", style.FontSize*0.67)
		spacing.marginBottom = elementMargin(node, "marginBottom", "margin-bottom", style.FontSize*0.67)
	}
	return spacing
}

func elementMargin(node PDFElementNode, camel, kebab string, fallback float64) float64 {
	if _, ok := node.Attribute(camel); ok {
		return htmlLengthToFloat(node, camel, kebab)
	}
	if _, ok := node.Attribute(kebab); ok {
		return htmlLengthToFloat(node, camel, kebab)
	}
	return fallback
}

func (spacing elementSpacing) inset(box PDFTextBox) PDFTextBox {
	box.X, box.Y = box.X+spacing.paddingLeft, box.Y+spacing.paddingTop
	if box.Width > 0 {
		box.Width = pdfMax(0, box.Width-spacing.paddingLeft-spacing.paddingRight)
	}
	if box.Height > 0 {
		box.Height = pdfMax(0, box.Height-spacing.paddingTop-spacing.paddingBottom)
	}
	return box
}

func applyElementDecoration(plan *textPlan, node PDFElementNode) {
	plan.backgroundColor = firstElementAttribute(node, "backgroundColor", "background-color")
	width := htmlLengthToFloat(node, "borderWidth", "border-width")
	style := strings.ToLower(firstElementAttribute(node, "borderStyle", "border-style"))
	color := firstElementAttribute(node, "borderColor", "border-color")
	if raw, ok := node.Attribute("border"); ok {
		shorthandWidth, shorthandStyle, shorthandColor := htmlParseBorderShorthand(raw)
		if width <= 0 {
			width = shorthandWidth
		}
		if style == "" {
			style = shorthandStyle
		}
		if color == "" {
			color = shorthandColor
		}
	}
	if style == "" || style == "none" {
		return
	}
	if width <= 0 {
		width = 1
	}
	if color == "" {
		color = "#000"
	}
	plan.borderWidth, plan.borderStyle, plan.borderColor = width, style, color
}

func firstElementAttribute(node PDFElementNode, names ...string) string {
	for _, name := range names {
		if value, ok := node.Attribute(name); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (e *PDFTextEngine) layoutTextNode(node *PDFTextNode, parentStyle PDFTextStyle, box PDFTextBox) (*textPlan, PDFTextMetrics, error) {
	style := parentStyle.Merge(node.Style)
	e.applyStyle(style)

	resolvedText := e.resolveText(node)
	encodedText := resolvedText
	if !e.pdf.CurrentFontIsUTF8() {
		encodedText = encodePDFTextLatin1(resolvedText)
	}
	e.glyphs.Record(style.FontFace, encodedText)

	lines, textWidth, wasClipped := e.layoutTextLines(encodedText, style, box)
	lineHeight := style.FontSize * style.LineHeight
	textHeight := lineHeight * float64(len(lines))
	lastLineWidth := 0.0
	if len(lines) > 0 {
		lastLineWidth = e.pdf.GetStringWidth(lines[len(lines)-1])
	}

	plan := &textPlan{
		style:      style,
		box:        box,
		lines:      lines,
		lineHeight: lineHeight,
		metrics: PDFTextMetrics{
			Width:      textWidth,
			Height:     textHeight,
			LineCount:  len(lines),
			WasClipped: wasClipped,
		},
		children: []*textPlan{},
	}
	if style.BackgroundColor != "" {
		plan.backgroundColor = strings.TrimSpace(style.BackgroundColor)
		plan.backgroundWidth = textWidth
		plan.backgroundHeight = textHeight
	}
	if style.BorderStyle != "" && strings.ToLower(strings.TrimSpace(style.BorderStyle)) != "none" {
		bw := style.BorderWidth
		if bw <= 0 {
			bw = 1
		}
		plan.borderWidth = bw
		plan.borderStyle = style.BorderStyle
		if strings.TrimSpace(style.BorderColor) != "" {
			plan.borderColor = strings.TrimSpace(style.BorderColor)
		} else {
			plan.borderColor = "#000"
		}
	}

	metrics, err := e.layoutChildren(plan, style, box, node.Children, node.ChildLineBreaks, textWidth, textHeight, lastLineWidth, len(lines), wasClipped)
	if err != nil {
		return nil, PDFTextMetrics{}, err
	}
	plan.metrics = metrics
	return plan, metrics, nil
}

func (e *PDFTextEngine) layoutChildren(plan *textPlan, style PDFTextStyle, box PDFTextBox, children []PDFNode, childLineBreaks []bool, baseWidth, baseHeight, lastLineWidth float64, baseLineCount int, baseClipped bool) (PDFTextMetrics, error) {
	lineHeight := style.FontSize * style.LineHeight
	state := childLayoutState{childY: box.Y + baseHeight, inlineY: box.Y, inlineX: box.X + lastLineWidth, maxWidth: baseWidth, totalHeight: baseHeight, lineCount: baseLineCount, clipped: baseClipped}
	if baseLineCount > 0 {
		state.inlineY = box.Y + float64(baseLineCount-1)*lineHeight
	}
	for index, child := range children {
		if child == nil {
			continue
		}
		if _, ok := child.(*ElemBr); ok {
			state.consumeBreak(box, lineHeight)
			continue
		}
		lineBreak := index < len(childLineBreaks) && childLineBreaks[index]
		childBox := state.inheritedBox(box, lineBreak)
		childPlan, childMetrics, err := e.layoutNode(child, style, childBox)
		if err != nil {
			return PDFTextMetrics{}, err
		}
		plan.children = append(plan.children, childPlan)
		state.consumeMetrics(box, childBox, childMetrics, lineBreak)
	}
	return state.metrics(), nil
}

type childLayoutState struct {
	childY, inlineY, inlineX, maxWidth, totalHeight float64
	lineCount                                       int
	clipped                                         bool
}

func (state *childLayoutState) consumeBreak(box PDFTextBox, lineHeight float64) {
	state.childY = pdfMax(state.childY, state.inlineY+lineHeight)
	state.totalHeight = pdfMax(state.totalHeight, state.childY-box.Y)
	state.inlineY, state.inlineX = state.childY, box.X
}

func (state childLayoutState) inheritedBox(parent PDFTextBox, lineBreak bool) PDFTextBox {
	box := PDFTextBox{Fit: parent.Fit}
	if lineBreak {
		box.X, box.Y = parent.X, state.childY
	} else {
		box.X, box.Y = state.inlineX, state.inlineY
	}
	if parent.Width > 0 {
		box.Width = pdfMax(0, parent.Width-(box.X-parent.X))
	}
	if parent.Height > 0 {
		box.Height = pdfMax(0, parent.Height-(box.Y-parent.Y))
	}
	return box
}

func (state *childLayoutState) consumeMetrics(parent, childBox PDFTextBox, metrics PDFTextMetrics, lineBreak bool) {
	if lineBreak {
		state.childY += metrics.Height
		state.totalHeight += metrics.Height
		state.inlineY, state.inlineX = state.childY, parent.X
	} else {
		state.inlineX += metrics.Width
		state.totalHeight = pdfMax(state.totalHeight, childBox.Y+metrics.Height-parent.Y)
		state.childY = parent.Y + state.totalHeight
	}
	state.lineCount += metrics.LineCount
	state.maxWidth = pdfMax(state.maxWidth, childBox.X-parent.X+metrics.Width)
	state.clipped = state.clipped || metrics.WasClipped
}

func (state childLayoutState) metrics() PDFTextMetrics {
	return PDFTextMetrics{Width: state.maxWidth, Height: state.totalHeight, LineCount: state.lineCount, WasClipped: state.clipped}
}

func (e *PDFTextEngine) layoutTextLines(text string, style PDFTextStyle, box PDFTextBox) ([]string, float64, bool) {
	if text == "" {
		return []string{""}, 0, false
	}

	segments := strings.Split(text, "\n")
	lines := make([]string, 0, len(segments))
	wasClipped := false
	for _, segment := range segments {
		segmentLines, clipped := e.layoutTextSegment(segment, box)
		lines = append(lines, segmentLines...)
		wasClipped = wasClipped || clipped
	}

	lineHeight := style.FontSize * style.LineHeight
	if box.Height > 0 && lineHeight > 0 {
		maxLines := int(box.Height / lineHeight)
		if maxLines < len(lines) {
			lines = lines[:pdfMaxInt(0, maxLines)]
			wasClipped = true
		}
	}

	return lines, e.maximumTextWidth(lines), wasClipped
}

func (e *PDFTextEngine) layoutTextSegment(segment string, box PDFTextBox) ([]string, bool) {
	if segment == "" {
		return []string{""}, false
	}
	if box.Width <= 0 {
		return []string{segment}, false
	}
	if box.Fit == TextFitClip {
		clipped, changed := e.clipToWidth(segment, box.Width)
		return []string{clipped}, changed
	}
	if box.Fit != TextFitWrap {
		return []string{segment}, false
	}
	if e.pdf.CurrentFontIsUTF8() {
		lines := e.pdf.SplitText(segment, box.Width)
		if len(lines) == 0 {
			return []string{""}, false
		}
		return lines, false
	}
	byteLines := e.pdf.SplitLines([]byte(segment), box.Width)
	if len(byteLines) == 0 {
		return []string{""}, false
	}
	lines := make([]string, len(byteLines))
	for index, line := range byteLines {
		lines[index] = string(line)
	}
	return lines, false
}
