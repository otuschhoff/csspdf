package docflowpdf

import (
	"errors"
	"strings"
	"testing"
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
