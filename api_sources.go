package csspdf

import (
	"context"
	"io/fs"
)

// TextSource resolves textual content from in-memory bytes/text, a file path,
// or an io/fs path.
type TextSource struct {
	Raw      []byte
	Text     string
	FilePath string
	FS       fs.FS
	FSPath   string
}

func (s TextSource) IsSet() bool {
	return s.Raw != nil || s.Text != "" || s.FilePath != "" || (s.FS != nil && s.FSPath != "")
}

func (s TextSource) Resolve(label string) (string, error) {
	return s.resolve(context.Background(), TrustedFileResolver{}, 0, label)
}

// JSONSource resolves JSON payloads from a Go object, raw JSON bytes/string,
// a file path, or an io/fs path.
type JSONSource struct {
	Object   any
	Raw      []byte
	Text     string
	FilePath string
	FS       fs.FS
	FSPath   string
}

func (s JSONSource) IsSet() bool {
	return s.Object != nil || s.Raw != nil || s.Text != "" || s.FilePath != "" || (s.FS != nil && s.FSPath != "")
}

func (s JSONSource) DecodeInto(target any, label string) error {
	return s.decodeInto(target, label, false)
}
