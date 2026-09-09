package csspdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
