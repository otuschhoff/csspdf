package docflowpdf

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// RenderOption customizes the default RenderInput used by Render.
type RenderOption func(*RenderInput) error

// Render builds a RenderInput internally from defaults + options and renders to
// outputPath.
func Render(outputPath string, options ...RenderOption) error {
	input, err := buildRenderInput(outputPath, options...)
	if err != nil {
		return err
	}
	return RenderWithInput(input)
}

// RenderContext builds a RenderInput from options and renders with ctx.
func RenderContext(ctx context.Context, outputPath string, options ...RenderOption) error {
	input, err := buildRenderInput(outputPath, options...)
	if err != nil {
		return err
	}
	return RenderWithInputContext(ctx, input)
}

func buildRenderInput(outputPath string, options ...RenderOption) (RenderInput, error) {
	if strings.TrimSpace(outputPath) == "" {
		return RenderInput{}, fmt.Errorf("output path is required")
	}

	input := RenderInput{
		OutputPath:          outputPath,
		DefaultLocale:       DefaultLocale,
		DefaultCurrencyCode: DefaultCurrencyCode,
	}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(&input); err != nil {
			return RenderInput{}, err
		}
	}

	return input, nil
}

// WithAssetBaseDir enables base-directory-driven defaults for assets, i18n,
// fonts, and images.
func WithAssetBaseDir(path string) RenderOption {
	return func(input *RenderInput) error {
		input.AssetBaseDir = path
		return nil
	}
}

// WithConfinedResourceRoot confines base-directory defaults, images, fonts,
// and explicit relative file sources to root.
func WithConfinedResourceRoot(root string) RenderOption {
	return func(input *RenderInput) error {
		input.AssetBaseDir = "."
		input.ResourceResolver = ConfinedFileResolver{Root: root}
		return nil
	}
}

// WithTrustedFileAccess explicitly enables unrestricted host-file resources.
func WithTrustedFileAccess() RenderOption {
	return func(input *RenderInput) error {
		input.ResourceResolver = TrustedFileResolver{}
		return nil
	}
}

// WithRenderLimits configures per-render operational budgets.
func WithRenderLimits(limits RenderLimits) RenderOption {
	return func(input *RenderInput) error {
		input.Limits = limits
		return nil
	}
}

// WithAssets sets fully resolved assets.
func WithAssets(assets Assets) RenderOption {
	return func(input *RenderInput) error {
		input.Assets = assets
		input.AssetInput = nil
		return nil
	}
}

// WithAssetInput sets flexible asset sources.
func WithAssetInput(assetInput AssetInput) RenderOption {
	return func(input *RenderInput) error {
		copyInput := assetInput
		input.AssetInput = &copyInput
		return nil
	}
}

// WithSourceData overrides source data used for flow payload source context.
func WithSourceData(source any) RenderOption {
	return func(input *RenderInput) error {
		input.SourceData = source
		return nil
	}
}

// WithI18nSource sets explicit i18n source.
func WithI18nSource(source JSONSource) RenderOption {
	return func(input *RenderInput) error {
		input.I18nSource = source
		return nil
	}
}

// WithI18nTemplateMacros enables execution of template macros inside i18n
// translation values.
func WithI18nTemplateMacros(enabled bool) RenderOption {
	return func(input *RenderInput) error {
		input.EnableI18nTemplateMacros = enabled
		return nil
	}
}

// WithLegacyPartialRendering allows recoverable template and element failures
// to be logged while emitting a potentially incomplete PDF. Strict rendering
// is the default.
func WithLegacyPartialRendering(enabled bool) RenderOption {
	return func(input *RenderInput) error {
		input.AllowPartialRender = enabled
		return nil
	}
}

// WithFontRegistrations sets explicit font registrations.
func WithFontRegistrations(registrations ...FontRegistration) RenderOption {
	return func(input *RenderInput) error {
		input.FontRegistrations = append([]FontRegistration(nil), registrations...)
		return nil
	}
}

// WithPageSize overrides both page width and height.
func WithPageSize(width, height float64) RenderOption {
	return func(input *RenderInput) error {
		input.PageWidth = width
		input.PageHeight = height
		return nil
	}
}

// WithPageFormat sets a named page format like A4, A3, A5, letter, or legal.
func WithPageFormat(format string) RenderOption {
	return func(input *RenderInput) error {
		input.PageFormat = format
		return nil
	}
}

// WithPageOrientation sets page orientation (portrait or landscape).
func WithPageOrientation(orientation string) RenderOption {
	return func(input *RenderInput) error {
		input.PageOrientation = orientation
		return nil
	}
}

// WithPageWidth overrides page width.
func WithPageWidth(width float64) RenderOption {
	return func(input *RenderInput) error {
		input.PageWidth = width
		return nil
	}
}

// WithPageHeight overrides page height.
func WithPageHeight(height float64) RenderOption {
	return func(input *RenderInput) error {
		input.PageHeight = height
		return nil
	}
}

// WithDefaultLocale sets fallback locale.
func WithDefaultLocale(locale string) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultLocale = locale
		return nil
	}
}

// WithDefaultCurrencyCode sets fallback currency code.
func WithDefaultCurrencyCode(code string) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultCurrencyCode = code
		return nil
	}
}

// WithDefaultMargins overrides default page margins.
func WithDefaultMargins(margins PageMargins) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultMargins = margins
		return nil
	}
}

// WithPageMarginsLeftRight sets both left and right default page margins.
func WithPageMarginsLeftRight(value float64) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultMargins.Left = value
		input.DefaultMargins.Right = value
		return nil
	}
}

// WithPageMarginsTopBottom sets both top and bottom default page margins.
func WithPageMarginsTopBottom(value float64) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultMargins.Top = value
		input.DefaultMargins.Bottom = value
		return nil
	}
}

// WithNow injects deterministic time source for template functions.
func WithNow(now func() time.Time) RenderOption {
	return func(input *RenderInput) error {
		input.Now = now
		return nil
	}
}

// WithFuncMapFactoryEx sets context-aware template function factory.
func WithFuncMapFactoryEx(factory FuncMapFactoryWithContext) RenderOption {
	return func(input *RenderInput) error {
		input.FuncMapFactoryEx = factory
		return nil
	}
}

// WithFuncMapFactory sets legacy template function factory.
func WithFuncMapFactory(factory FuncMapFactory) RenderOption {
	return func(input *RenderInput) error {
		input.FuncMapFactory = factory
		return nil
	}
}

// WithLogger sets non-fatal warning sink.
func WithLogger(logger Logger) RenderOption {
	return func(input *RenderInput) error {
		input.Logger = logger
		return nil
	}
}

// WithWarningWriter sets deprecated warning writer sink.
func WithWarningWriter(w io.Writer) RenderOption {
	return func(input *RenderInput) error {
		input.WarningWriter = w
		return nil
	}
}
