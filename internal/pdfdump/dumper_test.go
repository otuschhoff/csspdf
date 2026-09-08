package pdfdump

import (
	"bytes"
	"compress/zlib"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecompressFlateRejectsExpandedStreamOverLimit(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("A"), 4096)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := decompressFlate(compressed.Bytes(), 128)
	if err == nil || !strings.Contains(err.Error(), "exceeds byte limit 128") {
		t.Fatalf("expected decompression limit error, got %v", err)
	}
	var limitErr *InspectionLimitError
	if !errors.As(err, &limitErr) || limitErr.Stage != "decompressed stream" {
		t.Fatalf("expected typed decompression limit error, got %T: %v", err, err)
	}
}

func TestDumpPDFToRejectsCompressedExpansionOverLimit(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("A"), 4096)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	pdf := append([]byte("%PDF-1.4\n1 0 obj\n<</Filter /FlateDecode>>\nstream\n"), compressed.Bytes()...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n%%EOF")...)
	path := t.TempDir() + "/compressed.pdf"
	if err := os.WriteFile(path, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	err := DumpPDFTo(path, &bytes.Buffer{}, Limits{InputBytes: 1 << 20, DecompressedBytes: 128})
	var limitErr *InspectionLimitError
	if !errors.As(err, &limitErr) || limitErr.Stage != "decompressed stream" {
		t.Fatalf("expected typed decompression limit error, got %T: %v", err, err)
	}
}

func TestDumpPDFToPropagatesWriterError(t *testing.T) {
	path := t.TempDir() + "/minimal.pdf"
	if err := os.WriteFile(path, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("writer failed")
	err := DumpPDFTo(path, failingWriter{err: want}, Limits{InputBytes: 1024, DecompressedBytes: 1024})
	if !errors.Is(err, want) {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestDumpPDFToRejectsOversizedInputWithTypedError(t *testing.T) {
	path := t.TempDir() + "/large.pdf"
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 129), 0o600); err != nil {
		t.Fatal(err)
	}
	err := DumpPDFTo(path, &bytes.Buffer{}, Limits{InputBytes: 128, DecompressedBytes: 128})
	var limitErr *InspectionLimitError
	if !errors.As(err, &limitErr) || limitErr.Stage != "PDF input" {
		t.Fatalf("expected typed input limit error, got %T: %v", err, err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func FuzzDecompressFlateBounded(f *testing.F) {
	f.Add([]byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01})
	f.Add([]byte("not-zlib"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		decoded, _ := decompressFlate(data, 64<<10)
		if len(decoded) > 64<<10 {
			t.Fatalf("decoded %d bytes beyond limit", len(decoded))
		}
	})
}
