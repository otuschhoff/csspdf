package pdfdom

// TextAlign controls horizontal text alignment inside a bounding box.
type TextAlign string

const (
	TextAlignLeft   TextAlign = "left"
	TextAlignCenter TextAlign = "center"
	TextAlignRight  TextAlign = "right"
)

// TextFitMode controls how text behaves inside a constrained width/height.
type TextFitMode string

const (
	TextFitWrap TextFitMode = "wrap"
	TextFitClip TextFitMode = "clip"
)

// PDFTextStyle defines text styling and supports inheritance.
type PDFTextStyle struct {
	FontFace        string
	FontStyle       string
	FontStyleSet    bool
	FontSize        float64
	FontColor       string
	Align           TextAlign
	LineHeight      float64
	BackgroundColor string
	BorderColor     string
	BorderStyle     string
	BorderWidth     float64
}

// Merge returns a style where missing values from override are inherited from base.
func (base PDFTextStyle) Merge(override *PDFTextStyle) PDFTextStyle {
	res := base
	if override == nil {
		return res.withDefaults()
	}
	if override.FontFace != "" {
		res.FontFace = override.FontFace
	}
	if override.FontStyleSet {
		res.FontStyle = override.FontStyle
	}
	if override.FontSize > 0 {
		res.FontSize = override.FontSize
	}
	if override.FontColor != "" {
		res.FontColor = override.FontColor
	}
	if override.Align != "" {
		res.Align = override.Align
	}
	if override.LineHeight > 0 {
		res.LineHeight = override.LineHeight
	}
	if override.BackgroundColor != "" {
		res.BackgroundColor = override.BackgroundColor
	}
	if override.BorderColor != "" {
		res.BorderColor = override.BorderColor
	}
	if override.BorderStyle != "" {
		res.BorderStyle = override.BorderStyle
	}
	if override.BorderWidth > 0 {
		res.BorderWidth = override.BorderWidth
	}
	return res.withDefaults()
}

func (s PDFTextStyle) withDefaults() PDFTextStyle {
	if s.FontFace == "" {
		s.FontFace = "Helvetica"
	}
	if s.FontSize <= 0 {
		s.FontSize = 10
	}
	if s.FontColor == "" {
		s.FontColor = "#000"
	}
	if s.Align == "" {
		s.Align = TextAlignLeft
	}
	if s.LineHeight <= 0 {
		s.LineHeight = 1.2
	}
	return s
}

// PDFTextBox defines the desired bounding box and fit strategy.
type PDFTextBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Fit    TextFitMode
}

func (b PDFTextBox) withDefaults() PDFTextBox {
	if b.Fit == "" {
		b.Fit = TextFitWrap
	}
	return b
}

// StyleVariant holds the font rendering attributes for a single named style
// variant (Normal, Small, Title, etc.).  It is defined here so that pdflayout
// can use it without importing the invoice package.
type StyleVariant struct {
	FontFace  string
	FontColor string
	FontSize  int
}
