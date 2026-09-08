package docflowpdf

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDiagnosticErrorPreservesCauseAndProvenance(t *testing.T) {
	cause := errors.New("invalid declaration")
	err := &DiagnosticError{Code: DiagnosticTemplate, Stage: "css", Section: "doc", Layer: "customer", Page: 2, Err: cause}
	if !errors.Is(err, cause) {
		t.Fatal("diagnostic did not preserve its cause")
	}
	for _, expected := range []string{"DF003", "css", "section=doc", "layer=customer", "page=2"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("diagnostic %q does not contain %q", err, expected)
		}
	}
}

func TestMissingCSSLayerHasStructuredProvenance(t *testing.T) {
	_, err := (AssetInput{
		HTML:       TextSource{Text: `{{define "doc"}}<div>x</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`},
		CSSLayers:  []CSSLayerInput{{Name: "customer"}},
		SourceData: JSONSource{Text: `{}`},
	}).ResolveAssets()
	var diagnosticErr *DiagnosticError
	if !errors.As(err, &diagnosticErr) {
		t.Fatalf("expected structured layer diagnostic, got %T: %v", err, err)
	}
	if diagnosticErr.Code != DiagnosticAsset || diagnosticErr.Stage != "css-layer" || diagnosticErr.Layer != "customer" {
		t.Fatalf("unexpected layer diagnostic: %+v", diagnosticErr)
	}
}

func TestRenderPreservesMissingFileCauseThroughFacade(t *testing.T) {
	_, err := RenderToBytes(RenderInput{AssetInput: &AssetInput{
		HTML:       TextSource{FS: fstest.MapFS{}, FSPath: "missing.html"},
		CSS:        TextSource{Text: `@page { size: A4; }`},
		Flow:       JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}]}`},
		SourceData: JSONSource{Text: `{}`},
	}})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist through facade, got %T: %v", err, err)
	}
	var diagnosticErr *DiagnosticError
	if !errors.As(err, &diagnosticErr) || diagnosticErr.Code != DiagnosticInvalidInput || diagnosticErr.Stage != "preparation" {
		t.Fatalf("unexpected diagnostic: %+v", diagnosticErr)
	}
}
