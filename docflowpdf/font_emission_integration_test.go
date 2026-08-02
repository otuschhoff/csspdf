package docflowpdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otuschhoff/pdfa3-go/pkg/pdfa3"
)

func testUTF8FontAssets() Assets {
	return Assets{
		HTML: `
{{define "doc"}}<div id="body">ÄÖÜ äöü ß - Leistungsnachweis</div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`,
		CSS: `
@page { size: A4; margin: 30pt; }
#body { font-family: Futura-Medium; font-size: 12; }
`,
		Flow: Flow{
			MainFlow: []Section{{
				Template:    "doc",
				Transformer: "generic",
				Payload: PayloadConfig{
					IncludeSource: true,
				},
			}},
			PageNumber: Section{
				Template:    "page-number",
				Transformer: "generic",
				Payload:     PayloadConfig{Runtime: map[string]string{"Page": "page.number", "Total": "page.total"}},
			},
		},
		SourceData: map[string]any{"locale": "de"},
	}
}

func TestUTF8Type0Font_ToUnicodeIsExplicitAndParsable(t *testing.T) {
	fontPath := filepath.Clean(filepath.Join("..", "examples", "invoice", "fonts", "Futura-Medium.ttf"))
	if _, err := os.Stat(fontPath); err != nil {
		t.Fatalf("required font fixture missing: %v", err)
	}

	pdfBytes, err := RenderToBytes(RenderInput{
		Assets:              testUTF8FontAssets(),
		DefaultLocale:       "de",
		DefaultCurrencyCode: "EUR",
		FontRegistrations: []FontRegistration{{
			Family:  "Futura-Medium",
			Style:   "",
			Sources: []string{fontPath},
		}},
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}

	if !bytes.Contains(pdfBytes, []byte("/Subtype /Type0")) {
		t.Fatalf("expected Type0 font in output PDF")
	}
	if !bytes.Contains(pdfBytes, []byte("/ToUnicode")) {
		t.Fatalf("expected ToUnicode mapping in output PDF")
	}
	if bytes.Contains(pdfBytes, []byte("<0000> <FFFF> <0000>")) {
		t.Fatalf("unexpected generic identity ToUnicode range; expected explicit mappings")
	}
	if !bytes.Contains(pdfBytes, []byte("beginbfchar")) {
		t.Fatalf("expected explicit bfchar mappings in ToUnicode CMap")
	}

	tmpDir := t.TempDir()
	inputPDF := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(inputPDF, pdfBytes, 0644); err != nil {
		t.Fatalf("failed to write temp PDF: %v", err)
	}

	converter := pdfa3.NewConverter()
	fonts, err := converter.ExtractFontsFromPDF(inputPDF)
	if err != nil {
		t.Fatalf("ExtractFontsFromPDF returned error: %v", err)
	}

	var type0FontName string
	for _, font := range fonts {
		if font == nil {
			continue
		}
		if strings.EqualFold(font.SubType, "Type0") && strings.Contains(strings.ToLower(font.Name), "utf8futura-medium") {
			type0FontName = font.Name
			if font.CMaps == 0 {
				t.Fatalf("expected ToUnicode mappings for %s, got 0", font.Name)
			}
			break
		}
	}
	if type0FontName == "" {
		t.Fatalf("expected utf8futura-medium Type0 font in extracted font list")
	}

	usedChars, err := converter.ExtractUnicodesForFont(inputPDF, type0FontName)
	if err != nil {
		t.Fatalf("ExtractUnicodesForFont returned error: %v", err)
	}
	if len(usedChars) == 0 {
		t.Fatalf("expected downstream extractor to recover used chars for %s", type0FontName)
	}
	var extracted []rune
	for r := range usedChars {
		extracted = append(extracted, r)
	}

	var missing []rune
	for _, want := range []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'} {
		if !usedChars[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("expected extracted chars to include umlauts, missing: %q", string(missing))
	}
}
