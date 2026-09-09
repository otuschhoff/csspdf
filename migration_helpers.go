package csspdf

import "strings"

// LegacyCSSSourceAsLayer converts a legacy CSS source into a single CSS layer
// input suitable for AssetInput.CSSLayers migration.
func LegacyCSSSourceAsLayer(source TextSource, layerName string) CSSLayerInput {
	name := strings.TrimSpace(layerName)
	if name == "" {
		name = "legacy-css"
	}
	return CSSLayerInput{
		Name:   name,
		Source: source,
	}
}

// MigrateAssetInputLegacyCSSToSingleLayer converts AssetInput.CSS into one
// CSSLayers entry and clears AssetInput.CSS. If CSS is not set, input is
// returned unchanged.
func MigrateAssetInputLegacyCSSToSingleLayer(in AssetInput, layerName string) AssetInput {
	if !in.CSS.IsSet() {
		return in
	}

	out := in
	legacyLayer := LegacyCSSSourceAsLayer(in.CSS, layerName)
	out.CSSLayers = append([]CSSLayerInput{legacyLayer}, out.CSSLayers...)
	out.CSS = TextSource{}
	return out
}
