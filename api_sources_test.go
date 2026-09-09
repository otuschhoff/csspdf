package csspdf

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestJSONSourceDecodeInto_PreservesLargeIntegerAcrossInputModes(t *testing.T) {
	const largeInteger = "9007199254740993"
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.json")
	writeFile(t, filePath, `{"id":`+largeInteger+`}`)
	fsys := fstest.MapFS{
		"data.json": &fstest.MapFile{Data: []byte(`{"id":` + largeInteger + `}`)},
	}

	testCases := []struct {
		name   string
		source JSONSource
	}{
		{name: "object", source: JSONSource{Object: map[string]any{"id": int64(9007199254740993)}}},
		{name: "raw", source: JSONSource{Raw: []byte(`{"id":` + largeInteger + `}`)}},
		{name: "text", source: JSONSource{Text: `{"id":` + largeInteger + `}`}},
		{name: "file", source: JSONSource{FilePath: filePath}},
		{name: "fs", source: JSONSource{FS: fsys, FSPath: "data.json"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var decoded map[string]any
			if err := testCase.source.DecodeInto(&decoded, "source data"); err != nil {
				t.Fatalf("DecodeInto returned error: %v", err)
			}
			number, ok := decoded["id"].(json.Number)
			if !ok {
				t.Fatalf("decoded id type = %T, want json.Number", decoded["id"])
			}
			if number.String() != largeInteger {
				t.Fatalf("decoded id = %q, want %q", number, largeInteger)
			}
		})
	}
}
