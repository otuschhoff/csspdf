package docflowpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveFontRegistrations(input RenderInput) ([]FontRegistration, error) {
	if len(input.FontRegistrations) > 0 {
		result := make([]FontRegistration, 0, len(input.FontRegistrations))
		for _, registration := range input.FontRegistrations {
			result = append(result, normalizeFontRegistration(registration))
		}
		return dedupeFontRegistrations(result), nil
	}
	baseDir := strings.TrimSpace(input.AssetBaseDir)
	if baseDir == "" {
		return nil, nil
	}
	fontDirs := []string{filepath.Join(baseDir, "fonts"), filepath.Join(baseDir, "..", "fonts"), filepath.Join(baseDir, "..", "..", "fonts")}
	var entries []os.DirEntry
	selectedFontDir := ""
	for _, candidate := range fontDirs {
		candidate = filepath.Clean(candidate)
		scanned, err := os.ReadDir(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to scan font directory %q: %w", candidate, err)
		}
		entries, selectedFontDir = scanned, candidate
		break
	}
	if len(entries) == 0 {
		return nil, nil
	}
	registrations := make([]FontRegistration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		extension := strings.ToLower(filepath.Ext(name))
		if extension != ".ttf" && extension != ".otf" && extension != ".ttc" {
			continue
		}
		family := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
		if family == "" {
			continue
		}
		registrations = append(registrations, normalizeFontRegistration(FontRegistration{Family: family, Sources: []string{filepath.Join(selectedFontDir, name)}}))
	}
	return dedupeFontRegistrations(registrations), nil
}

func normalizeFontRegistration(registration FontRegistration) FontRegistration {
	family := strings.TrimSpace(registration.Family)
	style := normalizeFontStyleCode(registration.Style)
	if style == "" {
		family, style = splitFamilyAndStyleSuffix(family)
	}
	registration.Family, registration.Style = family, style
	return registration
}

func dedupeFontRegistrations(registrations []FontRegistration) []FontRegistration {
	if len(registrations) == 0 {
		return nil
	}
	result := make([]FontRegistration, 0, len(registrations))
	seen := make(map[string]struct{}, len(registrations))
	for _, registration := range registrations {
		key := strings.ToLower(strings.TrimSpace(registration.Family)) + "|" + strings.ToUpper(strings.TrimSpace(registration.Style))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, registration)
	}
	return result
}

func splitFamilyAndStyleSuffix(family string) (string, string) {
	family = strings.TrimSpace(family)
	if family == "" {
		return "", ""
	}
	lower := strings.ToLower(family)
	type suffixStyle struct{ suffix, style string }
	replacements := []suffixStyle{
		{"-bold-italic", "BI"}, {"_bold_italic", "BI"}, {" bold italic", "BI"},
		{"-bold-oblique", "BI"}, {"_bold_oblique", "BI"}, {" bold oblique", "BI"},
		{"-bolditalic", "BI"}, {"_bolditalic", "BI"}, {" bolditalic", "BI"},
		{"-boldoblique", "BI"}, {"_boldoblique", "BI"}, {" boldoblique", "BI"},
		{"-italic", "I"}, {"_italic", "I"}, {" italic", "I"},
		{"-oblique", "I"}, {"_oblique", "I"}, {" oblique", "I"},
		{"-bold", "B"}, {"_bold", "B"}, {" bold", "B"},
		{"-regular", ""}, {"_regular", ""}, {" regular", ""},
		{"-normal", ""}, {"_normal", ""}, {" normal", ""},
		{"-roman", ""}, {"_roman", ""}, {" roman", ""},
	}
	for _, replacement := range replacements {
		if strings.HasSuffix(lower, replacement.suffix) {
			base := strings.TrimSpace(family[:len(family)-len(replacement.suffix)])
			if base == "" {
				return family, ""
			}
			return base, replacement.style
		}
	}
	return family, ""
}

func normalizeFontStyleCode(style string) string {
	style = strings.ToUpper(strings.TrimSpace(style))
	style = strings.NewReplacer("-", "", "_", "", " ", "").Replace(style)
	switch style {
	case "IB":
		return "BI"
	case "", "B", "I", "BI":
		return style
	case "BOLD":
		return "B"
	case "ITALIC", "OBLIQUE":
		return "I"
	case "BOLDITALIC", "ITALICBOLD", "BOLDOBLIQUE", "OBLIQUEBOLD":
		return "BI"
	default:
		return style
	}
}
