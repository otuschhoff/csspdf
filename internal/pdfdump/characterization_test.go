package pdfdump

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	limitmarker "github.com/otuschhoff/csspdf/internal/limit"
)

func TestDictionaryTokenizationAndFormatting(t *testing.T) {
	dictionary := `<< /Length 42 /Type /Example /Ref 12 0 R /Array [1 (two words) << /Nested true >>] /Text (a \(nested\) value) /Hex <4142> >>`
	entries := dictEntriesMap(dictionary)
	for key, want := range map[string]string{
		"Type": "/Example", "Length": "42", "Ref": "12 0 R", "Array": "[1 (two words) << /Nested true >>]",
		"Text": `(a \(nested\) value)`, "Hex": "<4142>",
	} {
		if entries[key] != want {
			t.Errorf("entry %s = %q, want %q", key, entries[key], want)
		}
	}
	formatted := formatDictionary(dictionary)
	if strings.Index(formatted, "/Type") > strings.Index(formatted, "/Length") || !strings.Contains(formatted, "/Array [1 (two words)") {
		t.Fatalf("formatted dictionary = %q", formatted)
	}
	if got := formatDictionary("not a dictionary"); got != "not a dictionary" {
		t.Fatalf("non-dictionary changed to %q", got)
	}
	if got := formatDictionary("<<>>"); got != "<<>>" {
		t.Fatalf("empty dictionary changed to %q", got)
	}
	if !isOptionalObject("<< /Linearized 1 >>") || !isOptionalObject("<< /S 1 /I 2 /Length 3 >>") || isOptionalObject("<< /S 1 /I 2 /Length 3 /Type /X >>") {
		t.Fatal("optional-object classification failed")
	}
}

func TestDictionaryTokenEdgeCases(t *testing.T) {
	for token, want := range map[string]bool{"0": true, "-12": true, "+7": true, "": false, "+": false, "1.2": false} {
		if got := isIntegerToken(token); got != want {
			t.Errorf("isIntegerToken(%q) = %v, want %v", token, got, want)
		}
	}
	for input, want := range map[string]string{
		"/Name": "/Name", "[1 2]": "[1 2]", "<< /A 1 >>": "<< /A 1 >>", "(a(b)c)": "(a(b)c)", "<4142>": "<4142>", "4 0 R": "4 0 R",
	} {
		got, end := parsePDFObjectToken(input, 0)
		if got != want || end != len(input) {
			t.Errorf("parsePDFObjectToken(%q) = %q, %d", input, got, end)
		}
	}
	if got, end := parsePDFObjectToken("", 0); got != "" || end != 0 {
		t.Fatalf("empty token = %q, %d", got, end)
	}
	if got, end := parseNameToken("value", 0); got != "" || end != 0 {
		t.Fatalf("invalid name = %q, %d", got, end)
	}
	if got, end := parseLiteralStringToken(`(unterminated`, 0); got != `(unterminated` || end != len(got) {
		t.Fatalf("unterminated literal = %q, %d", got, end)
	}
}

func TestPNGPredictors(t *testing.T) {
	encoded := []byte{
		0, 10, 20, 30,
		1, 1, 1, 1,
		2, 1, 2, 3,
		3, 1, 2, 3,
		4, 1, 2, 3,
	}
	decoded, err := decodePNGPredictor(encoded, 3)
	if err != nil {
		t.Fatalf("decode predictor: %v", err)
	}
	want := []byte{10, 20, 30, 1, 2, 3, 2, 4, 6, 2, 5, 8, 3, 7, 11}
	if !bytes.Equal(decoded, want) {
		t.Fatalf("decoded predictor = %v, want %v", decoded, want)
	}
	for _, test := range []struct {
		data    []byte
		columns int
		message string
	}{{nil, 0, "invalid columns"}, {[]byte{0, 1}, 2, "row size mismatch"}, {[]byte{5, 1}, 1, "unsupported"}} {
		if _, err := decodePNGPredictor(test.data, test.columns); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("decodePNGPredictor(%v, %d) error = %v", test.data, test.columns, err)
		}
	}
	if paethPredictor(10, 2, 1) != 10 || paethPredictor(2, 10, 1) != 10 || paethPredictor(10, 10, 9) != 10 {
		t.Fatal("Paeth selection branches failed")
	}
}

func TestXRefParsingAndFormatting(t *testing.T) {
	for input, valid := range map[string]bool{"[]": true, "[1 2 -3]": true, "1 2": false, "[1 x]": false} {
		_, got := parseIntArray(input)
		if got != valid {
			t.Errorf("parseIntArray(%q) valid = %v", input, got)
		}
	}
	position := 0
	if value, ok := readBigEndianField([]byte{1, 2, 3}, &position, 2); !ok || value != 258 || position != 2 {
		t.Fatalf("big-endian field = %d, %v, position %d", value, ok, position)
	}
	if _, ok := readBigEndianField([]byte{1}, &position, 2); ok {
		t.Fatal("truncated big-endian field accepted")
	}
	if value, ok := readBigEndianField(nil, &position, 0); !ok || value != 0 {
		t.Fatal("zero-width field rejected")
	}
	decoded := []byte{0, 0, 0, 1, 10, 0, 2, 7, 3, 9, 4, 5}
	formatted := formatXRefStream(decoded, "<< /Type /XRef /W [1 1 1] /Index [0 4] >>")
	for _, fragment := range []string{"obj 0: free", "obj 1: in-use", "obj 2: compressed", "obj 3: type=9"} {
		if !strings.Contains(formatted, fragment) {
			t.Errorf("xref output missing %q: %s", fragment, formatted)
		}
	}
	for dictionary, fragment := range map[string]string{
		"<< /Size 1 >>":                  "missing or invalid /W",
		"<< /W [0 0 0] /Size 1 >>":       "invalid entry width",
		"<< /W [1 1 1] /Index [0] >>":    "invalid /Index",
		"<< /W [1 1 1] /Size invalid >>": "invalid /Size",
		"<< /W [1 1 1] /Size 2 >>":       "stream too short",
	} {
		if got := formatXRefStream(nil, dictionary); !strings.Contains(got, fragment) {
			t.Errorf("formatXRefStream(%q) = %q", dictionary, got)
		}
	}
	if got := formatXRefStream([]byte{0}, "<< /W [1 1 1] /Size 1 /DecodeParms << /Predictor 12 /Columns 2 >> >>"); !strings.Contains(got, "predictor decode failed") {
		t.Errorf("predictor failure output = %q", got)
	}
}

func TestStreamTextAndBinaryFormatting(t *testing.T) {
	if got := extractTJText(`[(Hello ) -10 <776f726c64> (\!)] TJ`); got != `Hello world\!` {
		t.Fatalf("TJ text = %q", got)
	}
	if extractTJText("not an array") != "" || decodeHexString("123") != "" || decodeHexString("41xx42") != "AB" {
		t.Fatal("invalid text-array handling failed")
	}
	if !isReadable("readable") || isReadable("") || isReadable("\x00\x01\x02a") {
		t.Fatal("readability classification failed")
	}
	commentary := commandCommentary("[(Hello)] TJ")
	if !strings.Contains(commentary, "Hello") || commandCommentary("unknown") != "" || commandCommentary("") != "" {
		t.Fatalf("command commentary = %q", commentary)
	}
	hex := hexDump([]byte("abc\x00def"), 4, 5)
	if !strings.Contains(hex, "0000:") || !strings.Contains(hex, "abc.") || !strings.Contains(hex, "bytes omitted") || hexDump(nil, 0, 0) != "(empty)" {
		t.Fatalf("hex dump = %q", hex)
	}
	if !isBinaryContent([]byte{0, 1, 2, 'a'}) || isBinaryContent([]byte("plain text")) || isBinaryContent(nil) {
		t.Fatal("binary classification failed")
	}
	if got := formatStreamContent([]byte{0, 1, 2, 3}); !strings.Contains(got, "binary data") {
		t.Fatalf("binary stream = %q", got)
	}
	formatted := formatStreamContent([]byte("BT [(Hello)] TJ ET"))
	if !strings.Contains(formatted, "begin text object") || !strings.Contains(formatted, "Hello") || !strings.Contains(formatted, "end text object") {
		t.Fatalf("formatted commands = %q", formatted)
	}
	if got := indentLines("one\n\ntwo\n", "  "); got != "  one\n\n  two\n" {
		t.Fatalf("indented lines = %q", got)
	}
}

func TestObjectAndHintStreamFormatting(t *testing.T) {
	objectStream := formatObjectStreamContent([]byte("5 0 6 10 << /Type /One >> << /Length 2 >>"))
	for _, fragment := range []string{"obj 5 @ 0", "obj 6 @ 10", "Object: 5", "Object: 6"} {
		if !strings.Contains(objectStream, fragment) {
			t.Errorf("object stream missing %q: %s", fragment, objectStream)
		}
	}
	if got := formatObjectStreamContent([]byte("plain text")); !strings.Contains(got, "plain text") {
		t.Fatalf("fallback object stream = %q", got)
	}
	hint := formatHintStream([]byte{1, 2, 3, 4}, "<< /S 1 /I 3 >>")
	if !strings.Contains(hint, "A [0:1]") || !strings.Contains(hint, "B [1:3]") || !strings.Contains(hint, "C [3:4]") {
		t.Fatalf("hint stream = %q", hint)
	}
	clamped := formatHintStream([]byte{1, 2}, "<< /S -1 /I 20 >>")
	if !strings.Contains(clamped, "/S boundary: 0") || !strings.Contains(clamped, "/I boundary: 2") {
		t.Fatalf("clamped hint = %q", clamped)
	}
	if got := formatHintStream([]byte("text"), "<<>>"); !strings.Contains(got, "text") {
		t.Fatalf("hint fallback = %q", got)
	}
}

func TestSyntheticPDFDump(t *testing.T) {
	compressed := compressFixture(t, []byte("BT [(Compressed text)] TJ ET"))
	pdf := append([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n2 0 obj\n<< /Filter /FlateDecode /Length 1 >>\nstream\n"), compressed...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n3 0 obj\n<< /S 1 /I 2 /Length 3 >>\nendobj\n%%EOF")...)
	filename := filepath.Join(t.TempDir(), "synthetic.pdf")
	if err := os.WriteFile(filename, pdf, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var output bytes.Buffer
	if err := DumpPDFTo(filename, &output, Limits{InputBytes: 1 << 20, DecompressedBytes: 1 << 20}); err != nil {
		t.Fatalf("DumpPDFTo returned error: %v", err)
	}
	for _, fragment := range []string{"OBJECT: 1", "Kind: no stream", "OBJECT: 2", "decompressed", "Compressed text", "OBJECT: 3 (optional)", "Total objects processed: 3"} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("dump missing %q:\n%s", fragment, output.String())
		}
	}
	if err := DumpPDFTo(filename, nil, Limits{InputBytes: 1, DecompressedBytes: 1}); err == nil {
		t.Fatal("nil output accepted")
	}
	if err := DumpPDFTo(filename, io.Discard, Limits{}); err == nil {
		t.Fatal("invalid limits accepted")
	}
	if _, err := readBoundedPDF(filepath.Join(t.TempDir(), "missing.pdf"), 1); err == nil {
		t.Fatal("missing PDF was accepted")
	}
}

func TestInspectionLimitErrorContract(t *testing.T) {
	err := &InspectionLimitError{Stage: "input", Limit: 1, Actual: 2}
	if !strings.Contains(err.Error(), "actual=2") || !errors.Is(err, limitmarker.ErrExceeded) {
		t.Fatalf("inspection limit error = %v", err)
	}
}

func compressFixture(t *testing.T, data []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("compress fixture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close fixture compressor: %v", err)
	}
	return compressed.Bytes()
}
