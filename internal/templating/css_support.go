package templating

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aymerick/douceur/parser"
)

const UnsupportedCSSPropertyCode = "CSS001"

type CSSSupportDiagnostic struct {
	Code     string
	Property string
	Selector string
}

func (d CSSSupportDiagnostic) Error() string {
	return fmt.Sprintf("%s unsupported CSS property %q is ignored for selector %q", d.Code, d.Property, d.Selector)
}

func SupportedCSSProperties() []string {
	properties := make([]string, 0, len(cssAttributeMappings)+1)
	for property := range cssAttributeMappings {
		properties = append(properties, property)
	}
	properties = append(properties, "font-weight")
	sort.Strings(properties)
	return properties
}

func AnalyzeCSSSupport(cssText string) ([]CSSSupportDiagnostic, error) {
	if strings.TrimSpace(cssText) == "" {
		return nil, nil
	}
	sheet, err := parser.Parse(cssText)
	if err != nil {
		return nil, fmt.Errorf("css parse: %w", err)
	}
	var diagnostics []CSSSupportDiagnostic
	for _, rule := range sheet.Rules {
		selector := strings.Join(rule.Selectors, ", ")
		for _, declaration := range rule.Declarations {
			property := strings.ToLower(strings.TrimSpace(declaration.Property))
			_, mapped := cssAttributeMappings[property]
			if !mapped && property != "font-weight" {
				diagnostics = append(diagnostics, CSSSupportDiagnostic{Code: UnsupportedCSSPropertyCode, Property: property, Selector: selector})
			}
		}
	}
	return diagnostics, nil
}
