package docflowpdf

import (
	"fmt"
	"io"
	"strings"
	"time"

	templateload "github.com/otuschhoff/go-dom2pdf/internal/templating"
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

func buildRenderInput(outputPath string, options ...RenderOption) (RenderInput, error) {
	if strings.TrimSpace(outputPath) == "" {
		return RenderInput{}, fmt.Errorf("output path is required")
	}

	input := RenderInput{
		OutputPath:          outputPath,
		PageCount:           1,
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

// WithPageCount sets the number of pages for flow rendering.
func WithPageCount(pageCount int) RenderOption {
	return func(input *RenderInput) error {
		input.PageCount = pageCount
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
func WithDefaultMargins(margins templateload.PageMargins) RenderOption {
	return func(input *RenderInput) error {
		input.DefaultMargins = margins
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
