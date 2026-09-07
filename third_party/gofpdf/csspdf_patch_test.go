package gofpdf

import (
	"fmt"
	"strings"
	"testing"
)

func TestCurrentFontIsUTF8(t *testing.T) {
	pdf := New("P", "pt", "A4", "")
	if pdf.CurrentFontIsUTF8() {
		t.Fatal("core font unexpectedly reported as UTF-8")
	}
	pdf.isCurrentUTF8 = true
	if !pdf.CurrentFontIsUTF8() {
		t.Fatal("UTF-8 font state was not exposed")
	}
}

func TestGenerateToUnicodeCMapUsesExplicitMappings(t *testing.T) {
	usedRunes := map[int]int{'A': 'A', 'Ä': 'Ä', '😀': 42}
	cmap := generateToUnicodeCMap(usedRunes)

	for _, mapping := range []string{"<0041> <0041>", "<00C4> <00C4>", "<002A> <002A>"} {
		if !strings.Contains(cmap, mapping) {
			t.Fatalf("missing mapping %s in CMap:\n%s", mapping, cmap)
		}
	}
	if strings.Contains(cmap, "<0000> <FFFF> <0000>") {
		t.Fatal("CMap contains the unsupported generic identity range")
	}
}

func TestGenerateToUnicodeCMapChunksMappings(t *testing.T) {
	usedRunes := make(map[int]int, toUnicodeChunkSize+1)
	for value := 1; value <= toUnicodeChunkSize+1; value++ {
		usedRunes[value] = value
	}
	cmap := generateToUnicodeCMap(usedRunes)
	if !strings.Contains(cmap, fmt.Sprintf("%d beginbfchar", toUnicodeChunkSize)) {
		t.Fatalf("missing full mapping chunk in CMap")
	}
	if !strings.Contains(cmap, "1 beginbfchar") {
		t.Fatalf("missing final mapping chunk in CMap")
	}
}

func TestFontUsageMapsAreCopied(t *testing.T) {
	usedRunes := map[int]int{0: 0, 'A': 'A'}
	codeSymbols := map[int]int{0: 0, 'A': 1}

	usedRunesCopy := cloneRuneUsage(usedRunes)
	codeSymbolsCopy := cloneCodeSymbolDictionary(codeSymbols)
	delete(usedRunesCopy, 0)
	delete(codeSymbolsCopy, 0)

	if _, exists := usedRunes[0]; !exists {
		t.Fatal("copy mutation changed source rune usage")
	}
	if _, exists := codeSymbols[0]; !exists {
		t.Fatal("copy mutation changed source code-symbol dictionary")
	}
}
