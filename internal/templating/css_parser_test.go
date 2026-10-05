package templating

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aymerick/douceur/parser"
)

func TestCSSParsingRejectsSemicolonInRulePrelude(t *testing.T) {
	parsers := map[string]func(string) error{
		"stylesheet": func(text string) error {
			_, err := PrepareStylesheet(text)
			return err
		},
		"support": func(text string) error {
			_, err := AnalyzeCSSSupport(text)
			return err
		},
		"page": func(text string) error {
			_, _, err := ParseCSSPageSettings(text, testDefaultPageSettings(), ParseLengthValue)
			return err
		},
	}

	for name, parse := range parsers {
		t.Run(name, func(t *testing.T) {
			for _, text := range []string{
				";",
				"div;",
				"div width: calc(1/0); }",
				"div { color: red; } span;",
				"@import 'theme.css'; div;",
				"div } @import 'theme.css';",
				"@media print { div; }",
				"@supports (display: grid) { @media print { div; } }",
			} {
				t.Run(text, func(t *testing.T) {
					if err := parse(text); err == nil || !strings.Contains(err.Error(), "css parse:") {
						t.Fatalf("expected CSS parse error for %q, got %v", text, err)
					}
				})
			}
		})
	}
}

func TestCSSPreludeValidationPreservesParsing(t *testing.T) {
	for _, text := range []string{
		"",
		"\ufeff /* ; { } */ div { color: red; }",
		`[data-label=";{}"] { color: red; content: ";{}"; }`,
		`[data-label="\;"] { color: red; }`,
		`@import url("theme;{}.css"); div { color: red; }`,
		`@page :first { size: A4; margin: 20pt; } div { color: red; }`,
		`@media print { div { color: red; } span { color: blue; } }`,
		`@supports (display: grid) { @media print { div { color: red; } } }`,
		`@keyframes fade { from { color: red; } to { color: blue; } }`,
		`<!-- div { color: red; } -->`,
		`@import } 'theme.css';`,
	} {
		t.Run(text, func(t *testing.T) {
			want, err := parser.Parse(text)
			if err != nil {
				t.Fatalf("baseline parser returned error: %v", err)
			}
			got, err := parseCSSStylesheet(text)
			if err != nil {
				t.Fatalf("validated parser returned error: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("validated parser changed stylesheet: got %#v, want %#v", got, want)
			}
		})
	}
}

func TestCSSPreludeValidationRejectsTokenizerErrors(t *testing.T) {
	if _, err := parseCSSStylesheet("div { color: 'unterminated"); err == nil || !strings.Contains(err.Error(), "tokenizer error") {
		t.Fatalf("expected tokenizer error, got %v", err)
	}
}
