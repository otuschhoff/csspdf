package csspdf

import (
	"path/filepath"
	"strings"
)

func resolveImageSearchDirs(input RenderInput) []string {
	baseDir := strings.TrimSpace(input.AssetBaseDir)
	if baseDir == "" {
		return nil
	}
	return []string{
		filepath.Join(baseDir, "images"),
		filepath.Join(baseDir, "..", "images"),
	}
}

func templateSourcesInRenderOrder(assets Assets) []string {
	sources := make([]string, 0, len(assets.HTMLLayers)+1)
	for _, layer := range assets.HTMLLayers {
		if strings.TrimSpace(layer.HTML) == "" {
			continue
		}
		sources = append(sources, layer.HTML)
	}
	if strings.TrimSpace(assets.HTML) != "" {
		sources = append(sources, assets.HTML)
	}
	return sources
}
