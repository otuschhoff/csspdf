package csspdf

import (
	"io"
	"time"
)

// RenderInput contains all data and options needed to render a flow-driven PDF.
type RenderInput struct {
	OutputPath string
	// AssetBaseDir is the profile directory. Automatic font discovery checks
	// fonts below this directory, then its parent and grandparent, in that order.
	AssetBaseDir             string
	EnableI18nTemplateMacros bool
	Assets                   Assets
	AssetInput               *AssetInput
	SourceData               any
	I18nSource               JSONSource
	FontRegistrations        []FontRegistration
	PageWidth                float64
	PageHeight               float64
	PageFormat               string
	PageOrientation          string
	DefaultLocale            string
	DefaultCurrencyCode      string
	DefaultMargins           PageMargins
	// Now supplies template time and, when non-nil, fixes PDF creation and
	// modification metadata for byte-reproducible rendering.
	Now              func() time.Time
	FuncMapFactoryEx FuncMapFactoryWithContext
	FuncMapFactory   FuncMapFactory
	Logger           Logger
	ResourceResolver ResourceResolver
	Limits           RenderLimits
	// AllowPartialRender preserves the legacy behavior of logging recoverable
	// template and element errors while emitting a potentially incomplete PDF.
	// Deprecated: use strict rendering, the default. Scheduled for removal in
	// v0.3.0.
	AllowPartialRender bool
	// Deprecated: use Logger. Scheduled for removal in v0.3.0.
	WarningWriter io.Writer
}

// FontRegistration declares one font family/style with ordered candidate file
// paths. The first existing path will be registered.
type FontRegistration struct {
	Family  string
	Style   string
	Sources []string
}
