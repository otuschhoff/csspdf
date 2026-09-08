package pdfrender

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

	marginTop := 0.0
	marginBottom := 0.0
	paddingTop := 0.0
	paddingRight := 0.0
	paddingBottom := 0.0
	paddingLeft := 0.0
	switch node.ElementType() {
	case "div":
		marginTop = htmlLengthToFloat(node, "marginTop", "margin-top")
		marginBottom = htmlLengthToFloat(node, "marginBottom", "margin-bottom")

		if pad := htmlLengthToFloat(node, "padding"); pad > 0 {
			paddingTop, paddingRight, paddingBottom, paddingLeft = pad, pad, pad, pad
		}
		if pad := htmlLengthToFloat(node, "paddingTop", "padding-top"); pad > 0 {
			paddingTop = pad
		}
		if pad := htmlLengthToFloat(node, "paddingRight", "padding-right"); pad > 0 {
			paddingRight = pad
		}
		if pad := htmlLengthToFloat(node, "paddingBottom", "padding-bottom"); pad > 0 {
			paddingBottom = pad
		}
		if pad := htmlLengthToFloat(node, "paddingLeft", "padding-left"); pad > 0 {
			paddingLeft = pad
		}
	case "h1":
		if _, ok := node.Attribute("marginTop"); ok {
			marginTop = htmlLengthToFloat(node, "marginTop", "margin-top")
		} else if _, ok := node.Attribute("margin-top"); ok {
			marginTop = htmlLengthToFloat(node, "marginTop", "margin-top")
		} else {
			marginTop = style.FontSize * 0.67
		}
		if _, ok := node.Attribute("marginBottom"); ok {
			marginBottom = htmlLengthToFloat(node, "marginBottom", "margin-bottom")
		} else if _, ok := node.Attribute("margin-bottom"); ok {
			marginBottom = htmlLengthToFloat(node, "marginBottom", "margin-bottom")
		} else {
			marginBottom = style.FontSize * 0.67
		}
	}
	outerBox := box
	outerBox.Y += marginTop
	if outerBox.Height > 0 {
		outerBox.Height = pdfMax(0, outerBox.Height-marginTop-marginBottom)
	}

	innerBox := outerBox
	innerBox.X += paddingLeft
	innerBox.Y += paddingTop
	if innerBox.Width > 0 {
		innerBox.Width = pdfMax(0, innerBox.Width-paddingLeft-paddingRight)
	}
	if innerBox.Height > 0 {
		innerBox.Height = pdfMax(0, innerBox.Height-paddingTop-paddingBottom)
	}

	plan, contentMetrics, err := e.layoutContainerNode(node.ElementStyle(), node.ElementChildren(), node.ElementChildLineBreaks(), parentStyle, innerBox)
	if err != nil {
		return nil, PDFTextMetrics{}, err
	}

	if bg, ok := node.Attribute("backgroundColor"); ok {
		plan.backgroundColor = strings.TrimSpace(bg)
	} else if bg, ok := node.Attribute("background-color"); ok {
		plan.backgroundColor = strings.TrimSpace(bg)
	}
	plan.box = outerBox
	plan.backgroundWidth = outerBox.Width
	if plan.backgroundWidth <= 0 {
		plan.backgroundWidth = contentMetrics.Width + paddingLeft + paddingRight
	}
	plan.backgroundHeight = contentMetrics.Height + paddingTop + paddingBottom

	borderWidth := htmlLengthToFloat(node, "borderWidth", "border-width")
	borderStyle := ""
	if v, ok := node.Attribute("borderStyle"); ok {
		borderStyle = strings.ToLower(strings.TrimSpace(v))
	} else if v, ok := node.Attribute("border-style"); ok {
		borderStyle = strings.ToLower(strings.TrimSpace(v))
	}
	borderColor := ""
	if v, ok := node.Attribute("borderColor"); ok {
		borderColor = strings.TrimSpace(v)
	} else if v, ok := node.Attribute("border-color"); ok {
		borderColor = strings.TrimSpace(v)
	}
	if raw, ok := node.Attribute("border"); ok {
		bw, bs, bc := htmlParseBorderShorthand(raw)
		if borderWidth <= 0 {
			borderWidth = bw
		}
		if borderStyle == "" {
			borderStyle = bs
		}
		if borderColor == "" {
			borderColor = bc
		}
	}
	if borderStyle != "" && borderStyle != "none" {
		if borderWidth <= 0 {
			borderWidth = 1
		}
		plan.borderWidth = borderWidth
		plan.borderStyle = borderStyle
		if borderColor != "" {
			plan.borderColor = borderColor
		} else {
			plan.borderColor = "#000"
		}
	}

	totalMetrics := contentMetrics
	totalMetrics.Width += paddingLeft + paddingRight
	totalMetrics.Height += marginTop + marginBottom + paddingTop + paddingBottom
	return plan, totalMetrics, nil
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
	childY := box.Y + baseHeight
	inlineY := box.Y
	if baseLineCount > 0 {
		inlineY = box.Y + float64(baseLineCount-1)*lineHeight
	}
	inlineX := box.X + lastLineWidth
	maxWidth := baseWidth
	totalHeight := baseHeight
	clippedAny := baseClipped
	lineCount := baseLineCount

	for childIdx, child := range children {
		if child == nil {
			continue
		}

		if _, ok := child.(*ElemBr); ok {
			childY = pdfMax(childY, inlineY+lineHeight)
			if childY-box.Y > totalHeight {
				totalHeight = childY - box.Y
			}
			inlineY = childY
			inlineX = box.X
			continue
		}

		lineBreak := false
		if childIdx < len(childLineBreaks) {
			lineBreak = childLineBreaks[childIdx]
		}

		childInherited := PDFTextBox{Fit: box.Fit}
		if lineBreak {
			childInherited.X = box.X
			childInherited.Y = childY
			if box.Width > 0 {
				childInherited.Width = box.Width
			}
			if box.Height > 0 {
				childInherited.Height = pdfMax(0, box.Height-totalHeight)
			}
		} else {
			childInherited.X = inlineX
			childInherited.Y = inlineY
			if box.Width > 0 {
				used := inlineX - box.X
				childInherited.Width = pdfMax(0, box.Width-used)
			}
			if box.Height > 0 {
				used := inlineY - box.Y
				childInherited.Height = pdfMax(0, box.Height-used)
			}
		}

		childPlan, childMetrics, err := e.layoutNode(child, style, childInherited)
		if err != nil {
			return PDFTextMetrics{}, err
		}
		plan.children = append(plan.children, childPlan)

		if lineBreak {
			childY += childMetrics.Height
			totalHeight += childMetrics.Height
			inlineY = childY
			inlineX = box.X
		} else {
			inlineX += childMetrics.Width
			if childInherited.Y+childMetrics.Height > box.Y+totalHeight {
				totalHeight = (childInherited.Y + childMetrics.Height) - box.Y
				childY = box.Y + totalHeight
			}
		}

		lineCount += childMetrics.LineCount

		childRight := (childInherited.X - box.X) + childMetrics.Width
		if childRight > maxWidth {
			maxWidth = childRight
		}
		clippedAny = clippedAny || childMetrics.WasClipped
	}

	return PDFTextMetrics{
		Width:      maxWidth,
		Height:     totalHeight,
		LineCount:  lineCount,
		WasClipped: clippedAny,
	}, nil
}

func (e *PDFTextEngine) layoutTextLines(text string, style PDFTextStyle, box PDFTextBox) ([]string, float64, bool) {
	if text == "" {
		return []string{""}, 0, false
	}

	segments := strings.Split(text, "\n")
	lines := make([]string, 0, len(segments))
	wasClipped := false

	for _, seg := range segments {
		if seg == "" {
			lines = append(lines, "")
			continue
		}

		if box.Width > 0 && box.Fit == TextFitWrap {
			if e.pdf.CurrentFontIsUTF8() {
				wrapped := e.pdf.SplitText(seg, box.Width)
				if len(wrapped) == 0 {
					lines = append(lines, "")
					continue
				}
				lines = append(lines, wrapped...)
				continue
			}

			wrapped := e.pdf.SplitLines([]byte(seg), box.Width)
			if len(wrapped) == 0 {
				lines = append(lines, "")
				continue
			}
			for _, wl := range wrapped {
				lines = append(lines, string(wl))
			}
			continue
		}

		if box.Width > 0 && box.Fit == TextFitClip {
			clipped, didClip := e.clipToWidth(seg, box.Width)
			wasClipped = wasClipped || didClip
			lines = append(lines, clipped)
			continue
		}

		lines = append(lines, seg)
	}

	lineHeight := style.FontSize * style.LineHeight
	if box.Height > 0 && lineHeight > 0 {
		maxLines := int(box.Height / lineHeight)
		if maxLines < len(lines) {
			lines = lines[:pdfMaxInt(0, maxLines)]
			wasClipped = true
		}
	}

	maxWidth := 0.0
	for _, line := range lines {
		w := e.pdf.GetStringWidth(line)
		if w > maxWidth {
			maxWidth = w
		}
	}

	return lines, maxWidth, wasClipped
}

func (e *PDFTextEngine) clipToWidth(text string, width float64) (string, bool) {
	if width <= 0 || text == "" {
		return text, false
	}
	if e.pdf.GetStringWidth(text) <= width {
		return text, false
	}

	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes)
		if e.pdf.GetStringWidth(candidate) <= width {
			return candidate, true
		}
	}
	return "", true
}

func (e *PDFTextEngine) resolveText(node *PDFTextNode) string {
	if node == nil {
		return ""
	}
	if node.I18nKey == "" {
		return node.Text
	}
	if e.i18n == nil {
		if node.Text != "" {
			return node.Text
		}
		return node.I18nKey
	}
	if len(node.I18nVars) > 0 {
		return e.i18n.TWithVars(node.I18nKey, node.I18nVars)
	}
	return e.i18n.T(node.I18nKey)
}

func (e *PDFTextEngine) applyStyle(style PDFTextStyle) {
	e.pdf.SetFont(style.FontFace, style.FontStyle, style.FontSize)
	if style.FontColor != "" {
		r, g, b := hexToRGB(style.FontColor)
		e.pdf.SetTextColor(r, g, b)
	}
}

func alignedX(align TextAlign, x, width, lineWidth float64) float64 {
	if width <= 0 {
		return x
	}
	switch align {
	case TextAlignRight:
		return x + width - lineWidth
	case TextAlignCenter:
		return x + (width-lineWidth)/2
	default:
		return x
	}
}

func resolveBox(box *PDFTextBox) PDFTextBox {
	if box != nil {
		return ensureTextBoxDefaults(*box)
	}
	return ensureTextBoxDefaults(PDFTextBox{X: 0, Y: 0, Width: 0, Height: 0, Fit: TextFitWrap})
}

func ensureTextStyleDefaults(style PDFTextStyle) PDFTextStyle {
	if style.FontFace == "" {
		style.FontFace = "Helvetica"
	}
	if style.FontSize <= 0 {
		style.FontSize = 10
	}
	if style.FontColor == "" {
		style.FontColor = "#000"
	}
	if style.Align == "" {
		style.Align = TextAlignLeft
	}
	if style.LineHeight <= 0 {
		style.LineHeight = 1.2
	}
	return style
}

func ensureTextBoxDefaults(box PDFTextBox) PDFTextBox {
	if box.Fit == "" {
		box.Fit = TextFitWrap
	}
	return box
}

func htmlLengthToFloat(node PDFElementNode, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := node.Attribute(key)
		if !ok {
			continue
		}
		value := strings.TrimSpace(strings.ToLower(raw))
		value = strings.TrimSuffix(value, "px")
		value = strings.TrimSuffix(value, "pt")
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
	}
	return 0
}

func resolveImageElement(node PDFElementNode, fallbackWidth float64, searchDirs []string, deferResolution bool) (path string, width, height, marginTop, marginBottom float64, err error) {
	if node == nil {
		return "", 0, 0, 0, 0, fmt.Errorf("image node cannot be nil")
	}
	src, ok := node.Attribute("src")
	if !ok || strings.TrimSpace(src) == "" {
		return "", 0, 0, 0, 0, fmt.Errorf("img missing src attribute")
	}
	if deferResolution {
		path = strings.TrimSpace(src)
	} else {
		path, ok = resolveImagePath(src, searchDirs)
		if !ok {
			return "", 0, 0, 0, 0, fmt.Errorf("image not found: %s", src)
		}
	}
	width = htmlLengthToFloat(node, "width")
	if width <= 0 {
		width = fallbackWidth
	}
	height = htmlLengthToFloat(node, "height")
	if height <= 0 {
		height = width
	}
	if width <= 0 || height <= 0 {
		return "", 0, 0, 0, 0, fmt.Errorf("img %q requires width/height or a positive fallback width", src)
	}
	marginTop = htmlLengthToFloat(node, "marginTop", "margin-top")
	marginBottom = htmlLengthToFloat(node, "marginBottom", "margin-bottom")
	return path, width, height, marginTop, marginBottom, nil
}

func resolveImagePath(src string, searchDirs []string) (string, bool) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", false
	}

	candidates := []string{trimmed}
	if !filepath.IsAbs(trimmed) {
		base := filepath.Base(trimmed)
		for _, dir := range searchDirs {
			cleanDir := strings.TrimSpace(dir)
			if cleanDir == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(cleanDir, trimmed),
				filepath.Join(cleanDir, base),
			)
		}
		candidates = append(candidates,
			filepath.Join("examples", "invoice", trimmed),
			filepath.Join("examples", "invoice", base),
			filepath.Join("resources", trimmed),
			filepath.Join("resources", base),
			filepath.Join("..", trimmed),
			filepath.Join("..", "examples", "invoice", trimmed),
			filepath.Join("..", "examples", "invoice", base),
			filepath.Join("..", "resources", trimmed),
			filepath.Join("..", "resources", base),
			filepath.Join("..", "..", trimmed),
			filepath.Join("..", "..", "examples", "invoice", trimmed),
			filepath.Join("..", "..", "examples", "invoice", base),
			filepath.Join("..", "..", "resources", trimmed),
			filepath.Join("..", "..", "resources", base),
		)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func imageTypeFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "JPG"
	case ".gif":
		return "GIF"
	default:
		return "PNG"
	}
}

// encodePDFTextLatin1 converts UTF-8 text to a PDF-friendly Latin-1 / CP-1252 byte string.
func encodePDFTextLatin1(text string) string {
	if text == "" {
		return ""
	}

	cp1252 := map[rune]byte{
		'€':      0x80,
		'‚':      0x82,
		'„':      0x84,
		'…':      0x85,
		'†':      0x86,
		'‡':      0x87,
		'ˆ':      0x88,
		'‰':      0x89,
		'Š':      0x8A,
		'‹':      0x8B,
		'Œ':      0x8C,
		'Ž':      0x8E,
		'\u2018': 0x91,
		'\u2019': 0x92,
		'\u201C': 0x93,
		'\u201D': 0x94,
		'•':      0x95,
		'–':      0x96,
		'—':      0x97,
		'˜':      0x98,
		'™':      0x99,
		'š':      0x9A,
		'›':      0x9B,
		'œ':      0x9C,
		'ž':      0x9E,
		'Ÿ':      0x9F,
	}

	var b strings.Builder
	b.Grow(len(text))

	for _, r := range text {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case r <= 0xFF:
			b.WriteByte(byte(r))
		case cp1252[r] != 0:
			b.WriteByte(cp1252[r])
		default:
			if utf8.ValidRune(r) {
				b.WriteByte('?')
			} else {
				b.WriteByte('?')
			}
		}
	}

	return b.String()
}

// --- Private helper copies (avoid import cycles with profile adapter packages) ---

func hexToRGB(hex string) (int, int, int) {
	hex = strings.ToLower(strings.TrimSpace(hex))
	if hex == "" {
		return 0, 0, 0
	}

	if named, ok := map[string][3]int{
		"black": {0, 0, 0}, "white": {255, 255, 255},
		"gray": {128, 128, 128}, "grey": {128, 128, 128},
		"lightgray": {211, 211, 211}, "lightgrey": {211, 211, 211},
		"darkgray": {169, 169, 169}, "darkgrey": {169, 169, 169},
		"red": {255, 0, 0}, "green": {0, 128, 0}, "blue": {0, 0, 255},
		"yellow": {255, 255, 0}, "orange": {255, 165, 0},
	}[hex]; ok {
		return named[0], named[1], named[2]
	}

	if len(hex) >= 4 && hex[0] == '#' {
		hex = hex[1:]
	}
	var r, g, b int
	if len(hex) == 3 {
		fmt.Sscanf(hex, "%1x%1x%1x", &r, &g, &b)
		r, g, b = r*17, g*17, b*17
	} else if len(hex) == 6 {
		fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	}
	return r, g, b
}

func htmlNormaliseTextAlign(value string) TextAlign {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "left", "l":
		return TextAlignLeft
	case "center", "c":
		return TextAlignCenter
	case "right", "r":
		return TextAlignRight
	default:
		return TextAlign(value)
	}
}

func htmlParseBorderShorthand(value string) (width float64, style, color string) {
	for _, part := range strings.Fields(strings.ToLower(strings.TrimSpace(value))) {
		switch part {
		case "none", "solid", "dashed", "dotted", "double":
			style = part
		default:
			v := strings.TrimSuffix(strings.TrimSuffix(part, "px"), "pt")
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				width = f
				continue
			}
			color = part
		}
	}
	return width, style, color
}

func pdfMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func pdfMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
