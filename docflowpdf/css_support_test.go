package docflowpdf

import "testing"

func TestCSSSupportContract(t *testing.T) {
	properties := SupportedCSSProperties()
	if len(properties) < 30 {
		t.Fatalf("support matrix unexpectedly small: %v", properties)
	}
	diagnostics, err := AnalyzeCSSSupport(`div { color: #111; display: grid; transform: rotate(1deg); }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 || diagnostics[0].Code != UnsupportedCSSPropertyCode || diagnostics[0].Property != "display" || diagnostics[1].Property != "transform" {
		t.Fatalf("unexpected support diagnostics: %+v", diagnostics)
	}
}
