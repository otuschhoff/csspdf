package docflowpdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TextSource resolves textual content from in-memory bytes/text, a file path,
// or an io/fs path.
type TextSource struct {
	Raw      []byte
	Text     string
	FilePath string
	FS       fs.FS
	FSPath   string
}

func (s TextSource) IsSet() bool {
	return s.Raw != nil || s.Text != "" || s.FilePath != "" || (s.FS != nil && s.FSPath != "")
}

func (s TextSource) Resolve(label string) (string, error) {
	if s.Raw != nil {
		return string(s.Raw), nil
	}
	if s.Text != "" {
		return s.Text, nil
	}
	if s.FilePath != "" {
		buf, err := os.ReadFile(s.FilePath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		return string(buf), nil
	}
	if s.FS != nil && s.FSPath != "" {
		buf, err := fs.ReadFile(s.FS, s.FSPath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from fs path %q: %w", label, s.FSPath, err)
		}
		return string(buf), nil
	}
	return "", fmt.Errorf("missing %s source", label)
}

// JSONSource resolves JSON payloads from a Go object, raw JSON bytes/string,
// a file path, or an io/fs path.
type JSONSource struct {
	Object   any
	Raw      []byte
	Text     string
	FilePath string
	FS       fs.FS
	FSPath   string
}

func (s JSONSource) IsSet() bool {
	return s.Object != nil || s.Raw != nil || s.Text != "" || s.FilePath != "" || (s.FS != nil && s.FSPath != "")
}

func (s JSONSource) DecodeInto(target any, label string) error {
	if s.Object != nil {
		buf, err := json.Marshal(s.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal %s object: %w", label, err)
		}
		if err := json.Unmarshal(buf, target); err != nil {
			return fmt.Errorf("failed to decode %s object: %w", label, err)
		}
		return nil
	}

	var buf []byte
	switch {
	case s.Raw != nil:
		buf = s.Raw
	case s.Text != "":
		buf = []byte(s.Text)
	case s.FilePath != "":
		read, err := os.ReadFile(s.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		buf = read
	case s.FS != nil && s.FSPath != "":
		read, err := fs.ReadFile(s.FS, s.FSPath)
		if err != nil {
			return fmt.Errorf("failed to read %s from fs path %q: %w", label, s.FSPath, err)
		}
		buf = read
	default:
		return fmt.Errorf("missing %s source", label)
	}

	if err := json.Unmarshal(buf, target); err != nil {
		return fmt.Errorf("failed to parse %s JSON: %w", label, err)
	}
	return nil
}

// AssetInput resolves all render assets from flexible sources.
type CSSLayerInput struct {
	Name     string
	Source   TextSource
	Optional bool
}

// Resolve resolves the layer source into a concrete CSSLayer.
func (l CSSLayerInput) Resolve(index int) (CSSLayer, bool, error) {
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

	cssText, err := l.Source.Resolve(fmt.Sprintf("template CSS layer %q", name))
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
		CSS:        TextSource{FilePath: filepath.Join(baseDir, "doc.css")},
		CSSLayers:  nil,
		Flow:       flowSource,
		SourceData: JSONSource{FilePath: filepath.Join(baseDir, "data.json")},
	}

	if in.HTML.IsSet() {
		merged.HTML = in.HTML
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
	html, err := in.HTML.Resolve("template HTML")
	if err != nil {
		return Assets{}, err
	}
	css, err := in.CSS.Resolve("template CSS")
	if err != nil {
		if len(in.CSSLayers) == 0 {
			return Assets{}, err
		}
		css = ""
	}

	layers := make([]CSSLayer, 0, len(in.CSSLayers))
	for idx, layerInput := range in.CSSLayers {
		layer, skipped, layerErr := layerInput.Resolve(idx)
		if layerErr != nil {
			return Assets{}, layerErr
		}
		if skipped {
			continue
		}
		layers = append(layers, layer)
	}

	var flow Flow
	if in.Flow.IsSet() {
		if err := in.Flow.DecodeInto(&flow, "flow"); err != nil {
			return Assets{}, err
		}
	}
	if err := applyFlowDefaults(&flow, html); err != nil {
		return Assets{}, err
	}

	var sourceData map[string]any
	if in.SourceData.IsSet() {
		sourceData = make(map[string]any)
		if err := in.SourceData.DecodeInto(&sourceData, "source data"); err != nil {
			return Assets{}, err
		}
	}

	assets := Assets{
		HTML:       html,
		CSS:        css,
		CSSLayers:  layers,
		Flow:       flow,
		SourceData: sourceData,
	}
	if err := assets.Validate(); err != nil {
		return Assets{}, err
	}
	return assets, nil
}
