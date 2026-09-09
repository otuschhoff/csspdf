package csspdf

import (
	"context"
	"fmt"
	"strings"
)

type assetInputResolver struct {
	ctx      context.Context
	resolver ResourceResolver
	maxBytes int64
}

func (r *assetInputResolver) resolve(input AssetInput) (Assets, error) {
	html, htmlLayers, err := r.resolveHTML(input)
	if err != nil {
		return Assets{}, err
	}
	css, cssLayers, err := r.resolveCSS(input)
	if err != nil {
		return Assets{}, err
	}
	flow, err := r.resolveFlow(input.Flow, html, htmlLayers)
	if err != nil {
		return Assets{}, err
	}
	sourceData, err := r.resolveSourceData(input.SourceData)
	if err != nil {
		return Assets{}, err
	}
	assets := Assets{HTML: html, HTMLLayers: htmlLayers, CSS: css, CSSLayers: cssLayers, Flow: flow, SourceData: sourceData}
	if err := assets.Validate(); err != nil {
		return Assets{}, err
	}
	return assets, nil
}

func (r *assetInputResolver) resolveHTML(input AssetInput) (string, []HTMLLayer, error) {
	html := ""
	if input.HTML.IsSet() {
		resolved, err := input.HTML.resolve(r.ctx, r.resolver, r.maxBytes, "template HTML")
		if err != nil {
			return "", nil, err
		}
		html = resolved
	} else if len(input.HTMLLayers) == 0 {
		if _, err := input.HTML.resolve(r.ctx, r.resolver, r.maxBytes, "template HTML"); err != nil {
			return "", nil, err
		}
	}
	layers, err := r.resolveHTMLLayers(input.HTMLLayers)
	if err != nil {
		return "", nil, err
	}
	if _, err := composeTemplateHTML(layers, html); err != nil {
		return "", nil, err
	}
	return html, layers, nil
}

func (r *assetInputResolver) resolveHTMLLayers(inputs []HTMLLayerInput) ([]HTMLLayer, error) {
	layers := make([]HTMLLayer, 0, len(inputs))
	for idx, input := range inputs {
		layer, skipped, err := input.resolve(r.ctx, r.resolver, r.maxBytes, idx)
		if err != nil {
			return nil, &DiagnosticError{Code: diagnosticCode(err, DiagnosticAsset), Stage: "html-layer", Layer: layerInputName(input.Name, idx), Err: err}
		}
		if !skipped {
			layers = append(layers, layer)
		}
	}
	return layers, nil
}

func (r *assetInputResolver) resolveCSS(input AssetInput) (string, []CSSLayer, error) {
	css, err := resolveLegacyCSSContext(r.ctx, r.resolver, r.maxBytes, input.CSS, len(input.CSSLayers) > 0)
	if err != nil {
		return "", nil, err
	}
	layers := make([]CSSLayer, 0, len(input.CSSLayers))
	for idx, layerInput := range input.CSSLayers {
		layer, skipped, layerErr := layerInput.resolve(r.ctx, r.resolver, r.maxBytes, idx)
		if layerErr != nil {
			return "", nil, &DiagnosticError{Code: diagnosticCode(layerErr, DiagnosticAsset), Stage: "css-layer", Layer: layerInputName(layerInput.Name, idx), Err: layerErr}
		}
		if !skipped {
			layers = append(layers, layer)
		}
	}
	return css, layers, nil
}

func layerInputName(name string, index int) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("layer-%d", index+1)
}

func (r *assetInputResolver) resolveFlow(source JSONSource, html string, layers []HTMLLayer) (Flow, error) {
	var flow Flow
	if source.IsSet() {
		if err := source.decodeIntoContext(r.ctx, r.resolver, r.maxBytes, &flow, "flow", true); err != nil {
			return Flow{}, err
		}
	}
	assets := Assets{HTML: html, HTMLLayers: layers}
	if err := applyFlowDefaults(&flow, templateSourcesInRenderOrder(assets)); err != nil {
		return Flow{}, err
	}
	return flow, nil
}

func (r *assetInputResolver) resolveSourceData(source JSONSource) (map[string]any, error) {
	if !source.IsSet() {
		return nil, nil
	}
	data := make(map[string]any)
	if err := source.decodeIntoContext(r.ctx, r.resolver, r.maxBytes, &data, "source data", false); err != nil {
		return nil, err
	}
	return data, nil
}
