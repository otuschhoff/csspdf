package docflowpdf

import (
	"fmt"
	"regexp"
	"strings"
	"text/template/parse"
)

var templateDefinePattern = regexp.MustCompile(`\{\{\-?\s*define\s+"([^"]+)"\s*\-?\}\}`)

func applyFlowDefaults(flow *Flow, htmlSources []string) error {
	if flow == nil {
		return fmt.Errorf("flow is nil")
	}
	names, err := extractDefinedTemplateNames(htmlSources)
	if err != nil {
		return fmt.Errorf("failed to inspect HTML templates for flow defaults: %w", err)
	}
	pageTemplate := strings.TrimSpace(flow.PageNumber.Template)
	if pageTemplate == "" {
		pageTemplate = defaultPageNumberTemplateName(names)
		flow.PageNumber.Template = pageTemplate
	}
	if flow.PageNumber.Transformer == "" {
		flow.PageNumber.Transformer = "generic"
	}

	if len(flow.MainFlow) == 0 {
		flow.MainFlow = inferDefaultMainFlow(names, pageTemplate)
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

func inferDefaultMainFlow(templateNames []string, pageTemplate string) []Section {
	if hasTemplateName(templateNames, "doc") {
		return []Section{defaultMainFlowSection("doc")}
	}

	sections := make([]Section, 0, len(templateNames))
	for _, name := range templateNames {
		if name == pageTemplate {
			continue
		}
		sections = append(sections, defaultMainFlowSection(name))
	}
	return sections
}

func defaultMainFlowSection(templateName string) Section {
	return Section{
		Template:    templateName,
		Transformer: "generic",
		Payload: PayloadConfig{
			IncludeSource: true,
		},
	}
}

func hasTemplateName(templateNames []string, want string) bool {
	for _, name := range templateNames {
		if name == want {
			return true
		}
	}
	return false
}

func extractDefinedTemplateNames(htmlSources []string) ([]string, error) {
	parsedNames := make(map[string]struct{})
	orderedCandidates := make([]string, 0)
	for index, htmlSource := range htmlSources {
		tree := parse.New(fmt.Sprintf("source-%d", index))
		tree.Mode = parse.SkipFuncCheck
		parsed := make(map[string]*parse.Tree)
		if _, err := tree.Parse(htmlSource, "{{", "}}", parsed); err != nil {
			return nil, fmt.Errorf("failed to parse template source[%d]: %w", index, err)
		}
		for name := range parsed {
			parsedNames[name] = struct{}{}
		}
		for _, match := range templateDefinePattern.FindAllStringSubmatch(htmlSource, -1) {
			if len(match) == 2 {
				orderedCandidates = append(orderedCandidates, match[1])
			}
		}
	}

	seen := make(map[string]struct{}, len(orderedCandidates))
	out := make([]string, 0, len(orderedCandidates))
	for _, candidate := range orderedCandidates {
		name := strings.TrimSpace(candidate)
		if name == "" {
			continue
		}
		if _, ok := parsedNames[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
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
