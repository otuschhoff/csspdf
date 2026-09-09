package csspdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssetInput resolves all render assets from flexible sources.
type CSSLayerInput struct {
	Name     string
	Source   TextSource
	Optional bool
}

type HTMLLayerInput struct {
	Name     string
	Source   TextSource
	Optional bool
}

// Resolve resolves the layer source into a concrete HTMLLayer.
func (l HTMLLayerInput) Resolve(index int) (HTMLLayer, bool, error) {
	return l.resolve(context.Background(), TrustedFileResolver{}, 0, index)
}

func (l HTMLLayerInput) resolve(ctx context.Context, resolver ResourceResolver, maxBytes int64, index int) (HTMLLayer, bool, error) {
	name := strings.TrimSpace(l.Name)
	if name == "" {
		name = fmt.Sprintf("layer-%d", index+1)
	}

	if !l.Source.IsSet() {
		if l.Optional {
			return HTMLLayer{}, true, nil
		}
		return HTMLLayer{}, false, fmt.Errorf("missing template HTML layer source for %q", name)
	}

	html, err := l.Source.resolve(ctx, resolver, maxBytes, fmt.Sprintf("template HTML layer %q", name))
	if err != nil {
		if l.Optional && errors.Is(err, os.ErrNotExist) {
			return HTMLLayer{}, true, nil
		}
		return HTMLLayer{}, false, err
	}

	return HTMLLayer{Name: name, HTML: html, Optional: l.Optional}, false, nil
}

// Resolve resolves the layer source into a concrete CSSLayer.
func (l CSSLayerInput) Resolve(index int) (CSSLayer, bool, error) {
	return l.resolve(context.Background(), TrustedFileResolver{}, 0, index)
}

func (l CSSLayerInput) resolve(ctx context.Context, resolver ResourceResolver, maxBytes int64, index int) (CSSLayer, bool, error) {
	name := strings.TrimSpace(l.Name)
	if name == "" {
		name = fmt.Sprintf("layer-%d", index+1)
	}

	if !l.Source.IsSet() {
		if l.Optional {
			return CSSLayer{}, true, nil
		}
		return CSSLayer{}, false, fmt.Errorf("missing template CSS layer source for %q", name)
	}

	cssText, err := l.Source.resolve(ctx, resolver, maxBytes, fmt.Sprintf("template CSS layer %q", name))
	if err != nil {
		if l.Optional && errors.Is(err, os.ErrNotExist) {
			return CSSLayer{}, true, nil
		}
		return CSSLayer{}, false, err
	}

	return CSSLayer{Name: name, CSS: cssText, Optional: l.Optional}, false, nil
}

type AssetInput struct {
	HTML       TextSource
	HTMLLayers []HTMLLayerInput
	CSS        TextSource
	CSSLayers  []CSSLayerInput
	Flow       JSONSource
	SourceData JSONSource
}

// ResolveWithBaseDir resolves assets from baseDir defaults and allows
// per-asset overrides from the receiver.
//
// Defaults relative to baseDir:
//   - doc.html
//   - doc.css
//   - flow.json
//   - data.json
func (in AssetInput) ResolveWithBaseDir(baseDir string) (Assets, error) {
	baseDir = filepath.Clean(baseDir)
	flowPath := filepath.Join(baseDir, "flow.json")
	flowSource := JSONSource{}
	if stat, err := os.Stat(flowPath); err == nil && !stat.IsDir() {
		flowSource = JSONSource{FilePath: flowPath}
	}
	merged := AssetInput{
		HTML:       TextSource{FilePath: filepath.Join(baseDir, "doc.html")},
		HTMLLayers: nil,
		CSS:        TextSource{FilePath: filepath.Join(baseDir, "doc.css")},
		CSSLayers:  nil,
		Flow:       flowSource,
		SourceData: JSONSource{FilePath: filepath.Join(baseDir, "data.json")},
	}

	if in.HTML.IsSet() {
		merged.HTML = in.HTML
	}
	if len(in.HTMLLayers) > 0 {
		merged.HTMLLayers = append([]HTMLLayerInput(nil), in.HTMLLayers...)
	}
	if in.CSS.IsSet() {
		merged.CSS = in.CSS
	}
	if len(in.CSSLayers) > 0 {
		merged.CSSLayers = append([]CSSLayerInput(nil), in.CSSLayers...)
		if !in.CSS.IsSet() {
			defaultCSSPath := filepath.Join(baseDir, "doc.css")
			if stat, err := os.Stat(defaultCSSPath); err == nil && !stat.IsDir() {
				merged.CSS = TextSource{FilePath: defaultCSSPath}
			} else {
				merged.CSS = TextSource{}
			}
		}
	}
	if in.Flow.IsSet() {
		merged.Flow = in.Flow
	}
	if in.SourceData.IsSet() {
		merged.SourceData = in.SourceData
	}

	return merged.ResolveAssets()
}

func (in AssetInput) ResolveAssets() (Assets, error) {
	return in.resolveAssetsContext(context.Background(), TrustedFileResolver{}, 0)
}

func (in AssetInput) resolveAssetsContext(ctx context.Context, resolver ResourceResolver, maxBytes int64) (Assets, error) {
	return (&assetInputResolver{ctx: ctx, resolver: resolver, maxBytes: maxBytes}).resolve(in)
}

func resolveLegacyCSSContext(ctx context.Context, resolver ResourceResolver, maxBytes int64, source TextSource, hasLayers bool) (string, error) {
	if source.IsSet() {
		return source.resolve(ctx, resolver, maxBytes, "template CSS")
	}
	if hasLayers {
		return "", nil
	}
	return source.resolve(ctx, resolver, maxBytes, "template CSS")
}
