package docflowpdf

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
type AssetInput struct {
	HTML       TextSource
	CSS        TextSource
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
	merged := AssetInput{
		HTML:       TextSource{FilePath: filepath.Join(baseDir, "doc.html")},
		CSS:        TextSource{FilePath: filepath.Join(baseDir, "doc.css")},
		Flow:       JSONSource{FilePath: filepath.Join(baseDir, "flow.json")},
		SourceData: JSONSource{FilePath: filepath.Join(baseDir, "data.json")},
	}

	if in.HTML.IsSet() {
		merged.HTML = in.HTML
	}
	if in.CSS.IsSet() {
		merged.CSS = in.CSS
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
		return Assets{}, err
	}

	var flow Flow
	if err := in.Flow.DecodeInto(&flow, "flow"); err != nil {
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
		Flow:       flow,
		SourceData: sourceData,
	}
	if err := assets.Validate(); err != nil {
		return Assets{}, err
	}
	return assets, nil
}
