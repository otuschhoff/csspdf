package docflowpdf

import (
	"fmt"
	"regexp"
	"strings"
)

var templateDefinePattern = regexp.MustCompile(`\{\{\-?\s*define\s+"([^"]+)"\s*\-?\}\}`)

func applyFlowDefaults(flow *Flow, htmlSource string) error {
	if flow == nil {
		return fmt.Errorf("flow is nil")
	}
	names := extractDefinedTemplateNames(htmlSource)
	pageTemplate := strings.TrimSpace(flow.PageNumber.Template)
	if pageTemplate == "" {
		pageTemplate = defaultPageNumberTemplateName(names)
		flow.PageNumber.Template = pageTemplate
	}
	if flow.PageNumber.Transformer == "" {
		flow.PageNumber.Transformer = "generic"
	}

	if len(flow.MainFlow) == 0 {
		for _, name := range names {
			if name == pageTemplate {
				continue
			}
			flow.MainFlow = append(flow.MainFlow, Section{
				Template:    name,
				Transformer: "generic",
				Payload: PayloadConfig{
					IncludeSource: true,
				},
			})
		}
	}

	if len(flow.MainFlow) == 0 {
		return fmt.Errorf("flow must include at least one mainFlow section (or define at least one non-page template in HTML)")
	}

	for i := range flow.MainFlow {
		if strings.TrimSpace(flow.MainFlow[i].Transformer) == "" {
			flow.MainFlow[i].Transformer = "generic"
		}
	}

	return nil
}

func extractDefinedTemplateNames(htmlSource string) []string {
	matches := templateDefinePattern.FindAllStringSubmatch(htmlSource, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) != 2 {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func defaultPageNumberTemplateName(templateNames []string) string {
	for _, name := range templateNames {
		if name == "page-number" {
			return name
		}
	}
	for _, name := range templateNames {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "page") && strings.Contains(lower, "number") {
			return name
		}
	}
	return "page-number"
}
