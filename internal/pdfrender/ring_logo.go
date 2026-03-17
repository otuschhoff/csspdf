package pdfrender

import (
	"fmt"
	"math"

	"github.com/otuschhoff/gofpdf"
)

const (
	LogoBaseRadius = 16.5
	LogoTplExtent  = 3.4 * LogoBaseRadius
	LogoTplCenter  = LogoTplExtent / 2

	linGradAngle       = 53.0
	widthFillRatio     = 4.4 / 16.5
	widthStrokeRatio   = 1.1 / 16.5
	shadowOffsetXRatio = 0.1 / 16.5
	shadowOffsetYRatio = 2.5 / 16.5
	shadowWidthRatio   = 1.0
)

var (
	shadowGradientStops = []gofpdf.ColorStop{
		{Position: 0.0, R: 0xFF, G: 0xFF, B: 0xFF},
		{Position: 0.333, R: 0xFF, G: 0xFF, B: 0xFF},
		{Position: 0.666, R: 0x6D, G: 0x6D, B: 0x6D},
		{Position: 0.933, R: 0xFF, G: 0xFF, B: 0xFF},
		{Position: 1.0, R: 0xFF, G: 0xFF, B: 0xFF},
	}

	mainGradientStops = []gofpdf.ColorStop{
		{Position: 0.0, R: 0xF4, G: 0xDC, B: 0xDB},
		{Position: 0.4, R: 0xE2, G: 0x76, B: 0x72},
		{Position: 1.0, R: 0xAA, G: 0x39, B: 0x2D},
	}
)

type logoDrawer interface {
	ClipCircle(x, y, r float64, outline bool)
	LinearGradient(x, y, w, h float64, r1, g1, b1, r2, g2, b2 int, x1, y1, x2, y2 float64)
	RadialGradientMultiStop(x, y, w, h float64, colorStops []gofpdf.ColorStop, x1, y1, x2, y2, r float64)
	LinearGradientMultiStop(x, y, w, h float64, colorStops []gofpdf.ColorStop, x1, y1, x2, y2 float64)
	TransformBegin()
	TransformEnd()
	MoveTo(x, y float64)
	CurveBezierCubicTo(cx0, cy0, cx1, cy1, x, y float64)
	ClosePath()
	DrawPath(styleStr string)
	ClipEnd()
	SetFillColor(r, g, b int)
	Circle(x, y, r float64, style string)
	SetDrawColor(r, g, b int)
	SetLineWidth(width float64)
}

type logoGeometry struct {
	widthFill         float64
	widthStroke       float64
	shadowOffsetX     float64
	shadowOffsetY     float64
	shadowWidth       float64
	linGradOffsetX    float64
	linGradOffsetY    float64
	outerShadowRadius float64
	innerShadowRadius float64
	shadowX           float64
	shadowY           float64
	outerMainRadius   float64
	innerMainRadius   float64
}

func CreateRingLogoTemplate(pdf *gofpdf.Fpdf, r, cx, cy float64) gofpdf.Template {
	return CreateRingLogoTemplateWithLayers(pdf, r, cx, cy, true, true)
}

func CreateRingLogoTemplateWithLayers(pdf *gofpdf.Fpdf, r, cx, cy float64, renderBg, renderFg bool) gofpdf.Template {
	return pdf.CreateTemplateCustom(
		gofpdf.PointType{X: 0, Y: 0},
		gofpdf.SizeType{Wd: 3.4 * r, Ht: 3.4 * r},
		func(t *gofpdf.Tpl) {
			drawRingLogoCore(t, r, cx, cy, renderBg, renderFg)
		},
	)
}

func calculateLogoGeometry(r, cx, cy float64) logoGeometry {
	g := logoGeometry{
		widthFill:     widthFillRatio * r,
		widthStroke:   widthStrokeRatio * r,
		shadowOffsetX: shadowOffsetXRatio * r,
		shadowOffsetY: shadowOffsetYRatio * r,
		shadowWidth:   shadowWidthRatio * r,
	}

	g.linGradOffsetX = r * math.Sin(linGradAngle*math.Pi/180.0)
	g.linGradOffsetY = math.Sqrt(r*r + g.linGradOffsetX*g.linGradOffsetX)

	g.outerShadowRadius = r + g.shadowWidth/2
	g.innerShadowRadius = r - g.shadowWidth/2
	g.shadowX = cx + g.shadowOffsetX
	g.shadowY = cy + g.shadowOffsetY

	g.outerMainRadius = r + g.widthFill/2
	g.innerMainRadius = r - g.widthFill/2

	return g
}

func drawShadowRing(d logoDrawer, g logoGeometry) {
	d.ClipCircle(g.shadowX, g.shadowY, g.outerShadowRadius, false)
	d.RadialGradientMultiStop(
		g.shadowX-g.outerShadowRadius,
		g.shadowY-g.outerShadowRadius,
		2*g.outerShadowRadius,
		2*g.outerShadowRadius,
		shadowGradientStops,
		0.5, 0.5,
		0.5, 0.5,
		0.5,
	)
	d.ClipEnd()
	if g.innerShadowRadius > 0 {
		d.SetFillColor(0xFF, 0xFF, 0xFF)
		d.Circle(g.shadowX, g.shadowY, g.innerShadowRadius, "F")
	}
}

func drawMainRing(d logoDrawer, cx, cy float64, g logoGeometry) {
	d.TransformBegin()
	clipDonut(d, cx, cy, g.outerMainRadius, g.innerMainRadius)
	d.LinearGradientMultiStop(
		cx-g.outerMainRadius,
		cy-g.outerMainRadius,
		2*g.outerMainRadius,
		2*g.outerMainRadius,
		mainGradientStops,
		0.5-g.linGradOffsetX/(2*g.outerMainRadius),
		0.5+g.linGradOffsetY/(2*g.outerMainRadius),
		0.5+g.linGradOffsetX/(2*g.outerMainRadius),
		0.5-g.linGradOffsetY/(2*g.outerMainRadius),
	)
	d.TransformEnd()

	d.SetLineWidth(g.widthStroke)
	d.SetDrawColor(0, 0, 0)
	d.Circle(cx, cy, g.outerMainRadius, "D")
	if g.innerMainRadius > 0 {
		d.Circle(cx, cy, g.innerMainRadius, "D")
	}
}

func clipDonut(d logoDrawer, cx, cy, rOuter, rInner float64) {
	const kappa = 0.552284749831

	ko := rOuter * kappa
	d.MoveTo(cx, cy-rOuter)
	d.CurveBezierCubicTo(cx+ko, cy-rOuter, cx+rOuter, cy-ko, cx+rOuter, cy)
	d.CurveBezierCubicTo(cx+rOuter, cy+ko, cx+ko, cy+rOuter, cx, cy+rOuter)
	d.CurveBezierCubicTo(cx-ko, cy+rOuter, cx-rOuter, cy+ko, cx-rOuter, cy)
	d.CurveBezierCubicTo(cx-rOuter, cy-ko, cx-ko, cy-rOuter, cx, cy-rOuter)
	d.ClosePath()

	if rInner > 0 {
		ki := rInner * kappa
		d.MoveTo(cx, cy-rInner)
		d.CurveBezierCubicTo(cx+ki, cy-rInner, cx+rInner, cy-ki, cx+rInner, cy)
		d.CurveBezierCubicTo(cx+rInner, cy+ki, cx+ki, cy+rInner, cx, cy+rInner)
		d.CurveBezierCubicTo(cx-ki, cy+rInner, cx-rInner, cy+ki, cx-rInner, cy)
		d.CurveBezierCubicTo(cx-rInner, cy-ki, cx-ki, cy-rInner, cx, cy-rInner)
		d.ClosePath()
	}

	d.DrawPath("W* n")
}

func drawRingLogoCore(d logoDrawer, r, cx, cy float64, renderBg, renderFg bool) {
	g := calculateLogoGeometry(r, cx, cy)
	if renderBg {
		drawShadowRing(d, g)
	}
	if renderFg {
		drawMainRing(d, cx, cy, g)
	}
}

func RenderLogoPDF(outputPath string, pageWidth, pageHeight, x, y, r float64, renderBg, renderFg bool) error {
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: pageWidth, Ht: pageHeight})

	drawRingLogoCore(pdf, r, x, y, renderBg, renderFg)

	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("failed to write logo PDF to %s: %w", outputPath, err)
	}

	return nil
}
