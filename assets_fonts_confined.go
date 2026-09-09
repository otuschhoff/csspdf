package csspdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

func resolveLayoutFontRegistrations(ctx context.Context, input RenderInput, limits RenderLimits) ([]pdfrender.FontRegistration, error) {
	if input.ResourceResolver == nil {
		registrations, err := resolveFontRegistrations(input)
		if err != nil {
			return nil, err
		}
		return toLayoutFontRegistrations(registrations), nil
	}
	registrations, err := confinedFontRegistrations(ctx, input)
	if err != nil {
		return nil, err
	}
	resolved := make([]pdfrender.FontRegistration, 0, len(registrations))
	for _, registration := range dedupeFontRegistrations(registrations) {
		font, err := resolveConfinedFont(ctx, input.ResourceResolver, normalizeFontRegistration(registration), limits.SourceBytes)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, font)
	}
	return resolved, nil
}

func confinedFontRegistrations(ctx context.Context, input RenderInput) ([]FontRegistration, error) {
	registrations := append([]FontRegistration(nil), input.FontRegistrations...)
	if len(registrations) > 0 {
		return registrations, nil
	}
	fontDir := filepath.Join(strings.TrimSpace(input.AssetBaseDir), "fonts")
	entries, err := input.ResourceResolver.ReadDir(ctx, fontDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan confined font directory %q: %w", fontDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && isFontExtension(entry.Name()) {
			family := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			registrations = append(registrations, FontRegistration{Family: family, Sources: []string{filepath.Join(fontDir, entry.Name())}})
		}
	}
	return registrations, nil
}

func resolveConfinedFont(ctx context.Context, resolver ResourceResolver, registration FontRegistration, maxBytes int64) (pdfrender.FontRegistration, error) {
	for _, candidate := range registration.Sources {
		data, err := resolver.ReadFile(ctx, candidate, maxBytes)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return pdfrender.FontRegistration{}, fmt.Errorf("failed to read font %q: %w", candidate, err)
		}
		if !isSupportedFont(data) {
			return pdfrender.FontRegistration{}, fmt.Errorf("font resource %q is not a supported TTF, OTF, or TTC file", candidate)
		}
		return pdfrender.FontRegistration{Family: registration.Family, Style: registration.Style, Data: data, Name: candidate}, nil
	}
	return pdfrender.FontRegistration{}, fmt.Errorf("font %s (style=%q) not found in configured sources", registration.Family, registration.Style)
}

func isFontExtension(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc":
		return true
	default:
		return false
	}
}

func isSupportedFont(data []byte) bool {
	return len(data) >= 4 && (bytes.Equal(data[:4], []byte{0, 1, 0, 0}) || bytes.Equal(data[:4], []byte("OTTO")) || bytes.Equal(data[:4], []byte("ttcf")))
}
