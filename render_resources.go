package csspdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

func resolveRenderAssetsContext(ctx context.Context, input RenderInput, limits RenderLimits) (Assets, error) {
	resolver := input.ResourceResolver
	if resolver == nil {
		resolver = TrustedFileResolver{}
	}
	if baseDir := strings.TrimSpace(input.AssetBaseDir); baseDir != "" {
		overrides := AssetInput{}
		if input.AssetInput != nil {
			overrides = *input.AssetInput
		}
		return resolveAssetInputWithBaseContext(ctx, overrides, baseDir, resolver, limits.SourceBytes)
	}
	if input.AssetInput != nil {
		return input.AssetInput.resolveAssetsContext(ctx, resolver, limits.SourceBytes)
	}
	assets := input.Assets
	assets.Flow = cloneFlow(input.Assets.Flow)
	if _, err := effectiveTemplateHTML(assets); err != nil {
		return Assets{}, err
	}
	if err := applyFlowDefaults(&assets.Flow, templateSourcesInRenderOrder(assets)); err != nil {
		return Assets{}, err
	}
	if err := assets.Validate(); err != nil {
		return Assets{}, err
	}
	return assets, nil
}

func resolveAssetInputWithBaseContext(ctx context.Context, input AssetInput, baseDir string, resolver ResourceResolver, maxBytes int64) (Assets, error) {
	defaultSource := func(name string) TextSource { return TextSource{FilePath: filepath.Join(baseDir, name)} }
	merged := AssetInput{HTML: defaultSource("doc.html"), CSS: defaultSource("doc.css"), SourceData: JSONSource{FilePath: filepath.Join(baseDir, "data.json")}}
	flowName := filepath.Join(baseDir, "flow.json")
	if data, err := resolver.ReadFile(ctx, flowName, maxBytes); err == nil {
		merged.Flow = JSONSource{Raw: data}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Assets{}, fmt.Errorf("failed to inspect optional flow resource %q: %w", flowName, err)
	}
	applyAssetOverrides(&merged, input)
	if len(input.CSSLayers) > 0 && !input.CSS.IsSet() {
		cssName := filepath.Join(baseDir, "doc.css")
		if _, err := resolver.ReadFile(ctx, cssName, maxBytes); err != nil && errors.Is(err, os.ErrNotExist) {
			merged.CSS = TextSource{}
		}
	}
	return merged.resolveAssetsContext(ctx, resolver, maxBytes)
}

func applyAssetOverrides(merged *AssetInput, input AssetInput) {
	if input.HTML.IsSet() {
		merged.HTML = input.HTML
	}
	if len(input.HTMLLayers) > 0 {
		merged.HTMLLayers = append([]HTMLLayerInput(nil), input.HTMLLayers...)
	}
	if input.CSS.IsSet() {
		merged.CSS = input.CSS
	}
	if len(input.CSSLayers) > 0 {
		merged.CSSLayers = append([]CSSLayerInput(nil), input.CSSLayers...)
	}
	if input.Flow.IsSet() {
		merged.Flow = input.Flow
	}
	if input.SourceData.IsSet() {
		merged.SourceData = input.SourceData
	}
}

func resolveI18nInput(locale string, input RenderInput) (*i18n.I18n, error) {
	limits, err := normalizeRenderLimits(input.Limits)
	if err != nil {
		return nil, err
	}
	return resolveI18nInputContext(context.Background(), locale, input, limits)
}

func resolveI18nInputContext(ctx context.Context, locale string, input RenderInput, limits RenderLimits) (*i18n.I18n, error) {
	i18nSource := input.I18nSource
	resolver := input.ResourceResolver
	if resolver == nil {
		resolver = TrustedFileResolver{}
	}
	if !i18nSource.IsSet() {
		baseDir := strings.TrimSpace(input.AssetBaseDir)
		if baseDir != "" {
			candidate := filepath.Join(baseDir, "i18n.json")
			if data, readErr := resolver.ReadFile(ctx, candidate, limits.SourceBytes); readErr == nil {
				i18nSource = JSONSource{Raw: data}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				return nil, readErr
			}
		}
	}
	if !i18nSource.IsSet() {
		return i18n.New(locale)
	}
	var source map[string]any
	if err := i18nSource.decodeIntoContext(ctx, resolver, limits.SourceBytes, &source, "i18n", false); err != nil {
		return nil, err
	}
	return i18n.NewFromSource(locale, source)
}

func resolveLayoutFontRegistrations(ctx context.Context, input RenderInput, limits RenderLimits) ([]pdfrender.FontRegistration, error) {
	if input.ResourceResolver == nil {
		registrations, err := resolveFontRegistrations(input)
		if err != nil {
			return nil, err
		}
		return toLayoutFontRegistrations(registrations), nil
	}
	registrations, err := confinedFontRegistrations(ctx, input)
	if err != nil {
		return nil, err
	}
	resolved := make([]pdfrender.FontRegistration, 0, len(registrations))
	for _, registration := range dedupeFontRegistrations(registrations) {
		font, err := resolveConfinedFont(ctx, input.ResourceResolver, normalizeFontRegistration(registration), limits.SourceBytes)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, font)
	}
	return resolved, nil
}

func confinedFontRegistrations(ctx context.Context, input RenderInput) ([]FontRegistration, error) {
	registrations := append([]FontRegistration(nil), input.FontRegistrations...)
	if len(registrations) > 0 {
		return registrations, nil
	}
	fontDir := filepath.Join(strings.TrimSpace(input.AssetBaseDir), "fonts")
	entries, err := input.ResourceResolver.ReadDir(ctx, fontDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan confined font directory %q: %w", fontDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && isFontExtension(entry.Name()) {
			family := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			registrations = append(registrations, FontRegistration{Family: family, Sources: []string{filepath.Join(fontDir, entry.Name())}})
		}
	}
	return registrations, nil
}

func resolveConfinedFont(ctx context.Context, resolver ResourceResolver, registration FontRegistration, maxBytes int64) (pdfrender.FontRegistration, error) {
	for _, candidate := range registration.Sources {
		data, err := resolver.ReadFile(ctx, candidate, maxBytes)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return pdfrender.FontRegistration{}, fmt.Errorf("failed to read font %q: %w", candidate, err)
		}
		if !isSupportedFont(data) {
			return pdfrender.FontRegistration{}, fmt.Errorf("font resource %q is not a supported TTF, OTF, or TTC file", candidate)
		}
		return pdfrender.FontRegistration{Family: registration.Family, Style: registration.Style, Data: data, Name: candidate}, nil
	}
	return pdfrender.FontRegistration{}, fmt.Errorf("font %s (style=%q) not found in configured sources", registration.Family, registration.Style)
}

func confinedImageLoader(ctx context.Context, input RenderInput, limits RenderLimits) pdfrender.ImageLoader {
	if input.ResourceResolver == nil {
		return nil
	}
	return func(name string) (pdfrender.ImageResource, error) {
		candidates := []string{name}
		base := strings.TrimSpace(input.AssetBaseDir)
		candidates = append(candidates, filepath.Join(base, "images", name), filepath.Join(base, "images", filepath.Base(name)))
		var lastErr error
		for _, candidate := range candidates {
			data, err := input.ResourceResolver.ReadFile(ctx, candidate, limits.ImageBytes)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return pdfrender.ImageResource{}, fmt.Errorf("load image resource %q: %w", candidate, err)
				}
				lastErr = err
				continue
			}
			imageType, pixels, ok := supportedImageType(data)
			if !ok {
				return pdfrender.ImageResource{}, fmt.Errorf("image resource %q is not a supported PNG, JPEG, or GIF file", candidate)
			}
			if pixels > limits.ImagePixels {
				return pdfrender.ImageResource{}, &BudgetError{Stage: "decoded image pixels", Limit: limits.ImagePixels, Actual: pixels}
			}
			return pdfrender.ImageResource{Name: "confined:" + candidate, Type: imageType, Data: data}, nil
		}
		return pdfrender.ImageResource{}, fmt.Errorf("image resource %q is unavailable: %w", name, lastErr)
	}
}

func isFontExtension(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc":
		return true
	default:
		return false
	}
}

func isSupportedFont(data []byte) bool {
	return len(data) >= 4 && (bytes.Equal(data[:4], []byte{0, 1, 0, 0}) || bytes.Equal(data[:4], []byte("OTTO")) || bytes.Equal(data[:4], []byte("ttcf")))
}

func supportedImageType(data []byte) (string, int64, bool) {
	var imageType string
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		imageType = "PNG"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		imageType = "JPG"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		imageType = "GIF"
	default:
		return "", 0, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", 0, false
	}
	width, height := int64(config.Width), int64(config.Height)
	if width > math.MaxInt64/height {
		return "", 0, false
	}
	return imageType, width * height, true
}
