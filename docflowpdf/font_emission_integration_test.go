package docflowpdf

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"unicode/utf16"

	"golang.org/x/image/font/gofont/goregular"
)

var toUnicodeMappingPattern = regexp.MustCompile(`<([0-9A-Fa-f]{4})>\s+<([0-9A-Fa-f]{4,8})>`)

func testUTF8FontAssets() Assets {
	return Assets{
		HTML: `
{{define "doc"}}<div id="body">ÄÖÜ äöü ß - Leistungsnachweis</div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`,
		CSS: `
@page { size: A4; margin: 30pt; }
#body { font-family: GoSans; font-size: 12; }
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
	fixtureDir := t.TempDir()
	fontPath := filepath.Join(fixtureDir, "Go-Regular.ttf")
	if err := os.WriteFile(fontPath, goregular.TTF, 0600); err != nil {
		t.Fatalf("write font fixture: %v", err)
	}

	pdfBytes, err := RenderToBytes(RenderInput{
		Assets:              testUTF8FontAssets(),
		DefaultLocale:       "de",
		DefaultCurrencyCode: "EUR",
		ResourceResolver:    TrustedFileResolver{},
		FontRegistrations: []FontRegistration{{
			Family:  "GoSans",
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

	usedChars, err := extractToUnicodeRunes(pdfBytes)
	if err != nil {
		t.Fatalf("parse ToUnicode CMap: %v", err)
	}
	if len(usedChars) == 0 {
		t.Fatalf("expected explicit ToUnicode mappings")
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

func extractToUnicodeRunes(pdfBytes []byte) (map[rune]bool, error) {
	streamStart := bytes.Index(pdfBytes, []byte("begincmap"))
	if streamStart < 0 {
		return nil, fmt.Errorf("begincmap marker not found")
	}
	streamEnd := bytes.Index(pdfBytes[streamStart:], []byte("endcmap"))
	if streamEnd < 0 {
		return nil, fmt.Errorf("endcmap marker not found")
	}

	matches := toUnicodeMappingPattern.FindAllSubmatch(pdfBytes[streamStart:streamStart+streamEnd], -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no bfchar mappings found")
	}

	runes := make(map[rune]bool, len(matches))
	for _, match := range matches {
		encoded, err := hex.DecodeString(string(match[2]))
		if err != nil || len(encoded)%2 != 0 {
			return nil, fmt.Errorf("invalid UTF-16 mapping %q", match[2])
		}
		units := make([]uint16, len(encoded)/2)
		for index := range units {
			units[index] = binary.BigEndian.Uint16(encoded[index*2 : index*2+2])
		}
		decoded := utf16.Decode(units)
		if len(decoded) != 1 {
			return nil, fmt.Errorf("mapping %q decoded to %d runes", match[2], len(decoded))
		}
		runes[decoded[0]] = true
	}
	return runes, nil
}
