package docflowpdf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	return s.resolve(context.Background(), TrustedFileResolver{}, 0, label)
}

func (s TextSource) resolve(ctx context.Context, resolver ResourceResolver, maxBytes int64, label string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.Raw != nil {
		if maxBytes > 0 && int64(len(s.Raw)) > maxBytes {
			return "", &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(s.Raw))}
		}
		return string(s.Raw), nil
	}
	if s.Text != "" {
		if maxBytes > 0 && int64(len(s.Text)) > maxBytes {
			return "", &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(s.Text))}
		}
		return s.Text, nil
	}
	if s.FilePath != "" {
		buf, err := resolver.ReadFile(ctx, s.FilePath, maxBytes)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		return string(buf), nil
	}
	if s.FS != nil && s.FSPath != "" {
		buf, err := readFSFile(ctx, s.FS, s.FSPath, maxBytes)
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
	return s.decodeInto(target, label, false)
}

func (s JSONSource) decodeInto(target any, label string, disallowUnknownFields bool) error {
	return s.decodeIntoContext(context.Background(), TrustedFileResolver{}, 0, target, label, disallowUnknownFields)
}

func (s JSONSource) decodeIntoContext(ctx context.Context, resolver ResourceResolver, maxBytes int64, target any, label string, disallowUnknownFields bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Object != nil {
		return decodeJSONObject(s.Object, target, label, maxBytes, disallowUnknownFields)
	}
	buf, err := s.resolveJSONBytes(ctx, resolver, maxBytes, label)
	if err != nil {
		return err
	}
	if maxBytes > 0 && int64(len(buf)) > maxBytes {
		return &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(buf))}
	}

	if err := decodeJSON(buf, target, disallowUnknownFields); err != nil {
		return fmt.Errorf("failed to parse %s JSON: %w", label, err)
	}
	return nil
}

func decodeJSONObject(object, target any, label string, maxBytes int64, disallowUnknownFields bool) error {
	buf, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("failed to marshal %s object: %w", label, err)
	}
	if maxBytes > 0 && int64(len(buf)) > maxBytes {
		return &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(buf))}
	}
	if err := decodeJSON(buf, target, disallowUnknownFields); err != nil {
		return fmt.Errorf("failed to decode %s object: %w", label, err)
	}
	return nil
}

func (s JSONSource) resolveJSONBytes(ctx context.Context, resolver ResourceResolver, maxBytes int64, label string) ([]byte, error) {
	switch {
	case s.Raw != nil:
		return s.Raw, nil
	case s.Text != "":
		return []byte(s.Text), nil
	case s.FilePath != "":
		data, err := resolver.ReadFile(ctx, s.FilePath, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		return data, nil
	case s.FS != nil && s.FSPath != "":
		data, err := readFSFile(ctx, s.FS, s.FSPath, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s from fs path %q: %w", label, s.FSPath, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("missing %s source", label)
	}
}

func readFSFile(ctx context.Context, sourceFS fs.FS, name string, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := sourceFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := io.Reader(file)
	if maxBytes > 0 {
		reader = io.LimitReader(file, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, &LimitError{Resource: name, Limit: maxBytes, Actual: int64(len(data))}
	}
	return data, ctx.Err()
}

func decodeJSON(data []byte, target any, disallowUnknownFields bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

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
