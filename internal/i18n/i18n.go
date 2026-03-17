// Package i18n provides locale-aware translation lookups.
// It has no dependencies on PDF rendering, layout, or domain models.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// I18n handles internationalization and translation lookups.
type I18n struct {
	translations map[string]string
	locale       string
}

// New creates a new I18n instance and loads translations for the given locale.
func New(locale string) (*I18n, error) {
	i := &I18n{
		locale:       locale,
		translations: make(map[string]string),
	}

	if err := i.loadTranslations(locale); err != nil {
		return nil, err
	}

	return i, nil
}

func (i *I18n) loadTranslations(locale string) error {
	paths := []string{
		filepath.Join("resources", "locales", locale+".json"),
		filepath.Join("locales", locale+".json"),
		filepath.Join("..", "..", "resources", "locales", locale+".json"),
	}

	var lastErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}

		if err := json.Unmarshal(data, &i.translations); err != nil {
			return fmt.Errorf("failed to parse translations from %s: %w", path, err)
		}

		return nil
	}

	return fmt.Errorf("failed to load translations for locale %s: %w", locale, lastErr)
}

// T translates a key to the current locale.
// If the key is not found, returns the key itself.
func (i *I18n) T(key string) string {
	if val, ok := i.translations[key]; ok {
		return val
	}
	return key
}

// TWithVars translates a key and replaces {{varName}} placeholders.
func (i *I18n) TWithVars(key string, vars map[string]string) string {
	text := i.T(key)

	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		text = strings.ReplaceAll(text, placeholder, v)
	}

	return text
}

// FloatSeparator returns the decimal separator for the current locale.
func (i *I18n) FloatSeparator() string {
	return i.T("_floatSeparator")
}

// KiloSeparator returns the thousands separator for the current locale.
func (i *I18n) KiloSeparator() string {
	return i.T("_kiloSeparator")
}

// Locale returns the current locale.
func (i *I18n) Locale() string {
	return i.locale
}
