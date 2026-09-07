// Package i18n provides locale-aware translation lookups.
// It has no dependencies on PDF rendering, layout, or domain models.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// I18n handles internationalization and translation lookups.
type I18n struct {
	translations map[string]string
	locale       string
}

// New creates a self-contained I18n instance with generic number separators.
func New(locale string) (*I18n, error) {
	i := newI18n(locale)
	i.translations["_floatSeparator"] = "."
	i.translations["_kiloSeparator"] = ","
	if locale == "de" {
		i.translations["_floatSeparator"] = ","
		i.translations["_kiloSeparator"] = "."
	}
	return i, nil
}

// NewFromSource creates a new I18n instance from a parsed JSON source object.
// The source should match the same i18n JSON structure used by i18n.json.
func NewFromSource(locale string, source map[string]any) (*I18n, error) {
	i := newI18n(locale)
	translations, err := flattenTranslationsForLocale(source, locale)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s translations from provided source: %w", locale, err)
	}
	i.translations = translations
	return i, nil
}

func newI18n(locale string) *I18n {
	return &I18n{
		locale:       locale,
		translations: make(map[string]string),
	}
}

func flattenTranslationsForLocale(source map[string]any, locale string) (map[string]string, error) {
	out := make(map[string]string)
	if err := flattenTranslationNode(source, "", locale, out); err != nil {
		return nil, err
	}
	return out, nil
}

func flattenTranslationNode(node any, prefix, locale string, out map[string]string) error {
	switch typed := node.(type) {
	case map[string]any:
		if localizedValue, ok := typed[locale]; ok {
			if prefix == "" {
				return fmt.Errorf("locale leaf %q found at root", locale)
			}
			text, ok := localizedValue.(string)
			if !ok {
				return fmt.Errorf("key %q locale %q is not a string", prefix, locale)
			}
			out[prefix] = text
			return nil
		}

		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			nextPrefix := key
			if prefix != "" {
				nextPrefix = prefix + "." + key
			}
			if err := flattenTranslationNode(typed[key], nextPrefix, locale, out); err != nil {
				return err
			}
		}
		return nil
	case string:
		if prefix == "" {
			return fmt.Errorf("string value found at root")
		}
		out[prefix] = typed
		return nil
	default:
		if prefix == "" {
			return fmt.Errorf("unsupported translation node type at root: %T", node)
		}
		return fmt.Errorf("unsupported translation node type at key %q: %T", prefix, node)
	}
}

// T translates a key to the current locale.
// If the key is not found, returns the key itself.
func (i *I18n) T(key string) string {
	return i.rawT(key)
}

// rawT returns the raw translated string without any variable substitution.
func (i *I18n) rawT(key string) string {
	if val, ok := i.translations[key]; ok {
		return val
	}
	return key
}

// TWithVars translates a key and replaces {{varName}} placeholders.
func (i *I18n) TWithVars(key string, vars map[string]string) string {
	return applyVars(i.rawT(key), vars)
}

func applyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}

// TemplateData returns all translations as a nested template object, applying
// placeholder vars to every value.
func (i *I18n) TemplateData(vars map[string]string) map[string]any {
	out := make(map[string]any)
	for key := range i.translations {
		setNestedValue(out, strings.Split(key, "."), i.TWithVars(key, vars))
	}
	return out
}

func setNestedValue(target map[string]any, path []string, value string) {
	if len(path) == 1 {
		target[path[0]] = value
		return
	}

	next, ok := target[path[0]].(map[string]any)
	if !ok {
		next = make(map[string]any)
		target[path[0]] = next
	}

	setNestedValue(next, path[1:], value)
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
