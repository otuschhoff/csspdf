package flowrender

import (
	"context"
	"fmt"
	htmltmpl "html/template"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

func TestPreparedFlowBuildAppliesSpanErrorPolicy(t *testing.T) {
	prepared, err := PrepareFlow([]string{`{{define "doc"}}<div><span font-size="invalid">text</span></div>{{end}}`}, "")
	if err != nil {
		t.Fatal(err)
	}
	options := BuildOptions{Context: context.Background(), MaxTemplateOutputBytes: 1024, MaxNodes: 100, MaxDepth: 10}
	if _, err := prepared.Build("doc", nil, nil, options); err == nil || !strings.Contains(err.Error(), "invalid span attribute") {
		t.Fatalf("expected strict span error, got %v", err)
	}

	var warnings []string
	options.AllowInvalidAttributes = true
	options.Warnf = func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	elements, err := prepared.Build("doc", nil, nil, options)
	if err != nil {
		t.Fatalf("legacy build returned error: %v", err)
	}
	if len(elements) != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "invalid span attribute") {
		t.Fatalf("unexpected legacy result: elements=%d warnings=%q", len(elements), warnings)
	}
}

func TestPreparedFlowReusesTemplateAndStylesheetParsing(t *testing.T) {
	prepared, err := PrepareFlow([]string{`{{define "doc"}}<div>{{value}}</div>{{end}}`}, `div { color: #123456; }`)
	if err != nil {
		t.Fatal(err)
	}
	options := BuildOptions{Context: context.Background(), MaxTemplateOutputBytes: 1024, MaxNodes: 100, MaxDepth: 10}
	for _, value := range []string{"first", "second"} {
		elements, err := prepared.Build("doc", nil, htmltmpl.FuncMap{"value": func() string { return value }}, options)
		if err != nil {
			t.Fatal(err)
		}
		color, hasColor := elements[0].Attribute("font-color")
		if len(elements) != 1 || !hasColor || color != "#123456" {
			t.Fatalf("expected prepared CSS on one element, got %#v", elements)
		}
		children := elements[0].ElementChildren()
		if len(children) != 1 || children[0].(*pdfdom.PDFTextNode).Text != value {
			t.Fatalf("expected current template function value %q, got %#v", value, children)
		}
	}
	stats := prepared.Stats()
	if stats.TemplateParses != 1 || stats.StylesheetParses != 1 {
		t.Fatalf("expected one template and stylesheet parse, got %+v", stats)
	}
}

func TestPreparedFlowConcurrentBuildDoesNotCorruptCache(t *testing.T) {
	prepared, err := PrepareFlow([]string{`{{define "doc"}}<div>{{value}}</div>{{end}}`}, `div { color: #123456; }`)
	if err != nil {
		t.Fatal(err)
	}
	options := BuildOptions{Context: context.Background(), MaxTemplateOutputBytes: 1024, MaxNodes: 100, MaxDepth: 10}
	errors := make(chan error, 8)
	for index := 0; index < 8; index++ {
		index := index
		go func() {
			var funcs htmltmpl.FuncMap
			if index%2 == 0 {
				funcs = htmltmpl.FuncMap{"value": func() string { return fmt.Sprintf("value-%d", index) }}
			} else {
				funcs = htmltmpl.FuncMap{"value": func() (string, error) { return fmt.Sprintf("value-%d", index), nil }}
			}
			elements, buildErr := prepared.Build("doc", nil, funcs, options)
			if buildErr != nil {
				errors <- buildErr
				return
			}
			children := elements[0].ElementChildren()
			if len(children) != 1 || children[0].(*pdfdom.PDFTextNode).Text != fmt.Sprintf("value-%d", index) {
				errors <- fmt.Errorf("build %d returned unexpected children: %#v", index, children)
				return
			}
			errors <- nil
		}()
	}
	for index := 0; index < 8; index++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	stats := prepared.Stats()
	if stats.TemplateParses != 2 || stats.StylesheetParses != 1 {
		t.Fatalf("expected two function-map variants and one stylesheet parse, got %+v", stats)
	}
}
