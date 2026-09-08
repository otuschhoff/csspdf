package pdfrender

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otuschhoff/gofpdf"
)

// FontRegistration declares one font family/style with ordered candidate file
// paths. The first existing file path will be loaded.
type FontRegistration struct {
	Family  string
	Style   string
	Sources []string
	Data    []byte
	Name    string
}

type ImageResource struct {
	Name string
	Type string
	Data []byte
}

type ImageLoader func(name string) (ImageResource, error)

// LayoutOptions controls optional renderer initialization behavior.
type LayoutOptions struct {
	FontRegistrations  []FontRegistration
	ImageSearchDirs    []string
	ImageLoader        ImageLoader
	StrictRenderErrors bool
	Context            context.Context
	MaxPages           int
}

func loadDefaultFontSet(pdf *gofpdf.Fpdf, fonts []FontRegistration) error {
	for _, font := range fonts {
		family := strings.TrimSpace(font.Family)
		if family == "" {
			return fmt.Errorf("font registration has empty family")
		}
		style := strings.TrimSpace(font.Style)
		if len(font.Data) > 0 {
			pdf.AddUTF8FontFromBytes(family, style, font.Data)
			if pdf.Err() {
				return fmt.Errorf("failed to load %s (style=%q) from %s: %w", family, style, font.Name, pdf.Error())
			}
			continue
		}
		if len(font.Sources) == 0 {
			return fmt.Errorf("font registration %q has no sources", family)
		}
		if err := loadFontFromSources(pdf, family, style, font.Sources); err != nil {
			return err
		}
	}
	return nil
}

func loadFontFromSources(pdf *gofpdf.Fpdf, family, style string, sources []string) error {
	for _, source := range sources {
		fontPath := strings.TrimSpace(source)
		if fontPath == "" {
			continue
		}
		if _, err := os.Stat(fontPath); err != nil {
			continue
		}
		pdf.AddUTF8Font(family, style, fontPath)
		if pdf.Err() {
			return fmt.Errorf("failed to load %s (style=%q) from %s: %w", family, style, fontPath, pdf.Error())
		}
		return nil
	}
	return fmt.Errorf("font %s (style=%q) not found in configured sources", family, style)
}
