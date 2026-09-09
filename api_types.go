package csspdf

// PageMargins defines default page margins in PDF points.
type PageMargins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

type Section struct {
	Template    string        `json:"template"`
	Transformer string        `json:"transformer"`
	Payload     PayloadConfig `json:"payload"`
}

type PayloadConfig struct {
	IncludeSource bool              `json:"includeSource"`
	Runtime       map[string]string `json:"runtime"`
	I18nVars      map[string]string `json:"i18nVars"`
	Static        map[string]any    `json:"static"`
	LocalePath    string            `json:"localePath"`
}

type Flow struct {
	MainFlow   []Section `json:"mainFlow"`
	PageNumber Section   `json:"pageNumber"`
}

// CSSLayer is a resolved stylesheet layer.
// Layers are applied in order and then legacy Assets.CSS is applied last.
type CSSLayer struct {
	Name     string
	CSS      string
	Optional bool
}

// HTMLLayer is a resolved template layer.
// Layers are parsed in order and then legacy Assets.HTML is parsed last,
// allowing legacy HTML to override shared block defaults.
type HTMLLayer struct {
	Name     string
	HTML     string
	Optional bool
}

type Assets struct {
	HTML       string
	HTMLLayers []HTMLLayer
	CSS        string
	CSSLayers  []CSSLayer
	Flow       Flow
	SourceData map[string]any
}
