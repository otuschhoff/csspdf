package csspdf

import (
	"fmt"
	"strings"
)

func effectiveTemplateHTML(assets Assets) (string, error) {
	return composeTemplateHTML(assets.HTMLLayers, assets.HTML)
}

func composeTemplateHTML(layers []HTMLLayer, legacyHTML string) (string, error) {
	var b strings.Builder

	appendPart := func(html string) {
		trimmed := strings.TrimSpace(html)
		if trimmed == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(trimmed)
		if !strings.HasSuffix(trimmed, "\n") {
			b.WriteString("\n")
		}
	}

	for _, layer := range layers {
		appendPart(layer.HTML)
	}
	appendPart(legacyHTML)

	composed := strings.TrimSpace(b.String())
	if composed == "" {
		return "", fmt.Errorf("template HTML must not be empty")
	}
	return composed, nil
}

func effectiveTemplateCSS(assets Assets) (string, error) {
	return composeTemplateCSS(assets.CSSLayers, assets.CSS)
}

func composeTemplateCSS(layers []CSSLayer, legacyCSS string) (string, error) {
	var b strings.Builder

	appendPart := func(css string) {
		trimmed := strings.TrimSpace(css)
		if trimmed == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(trimmed)
		if !strings.HasSuffix(trimmed, "\n") {
			b.WriteString("\n")
		}
	}

	for _, layer := range layers {
		appendPart(layer.CSS)
	}
	appendPart(legacyCSS)

	composed := strings.TrimSpace(b.String())
	if composed == "" {
		return "", fmt.Errorf("template CSS must not be empty")
	}
	return composed, nil
}
