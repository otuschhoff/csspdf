package templating

import (
	"bytes"
	"fmt"
	"html/template"
)

func ExecuteNamed(templateSource, templateName string, data any) (string, error) {
	return ExecuteNamedWithFuncs(templateSource, templateName, data, nil)
}

func ExecuteNamedFromSources(templateSources []string, templateName string, data any) (string, error) {
	return ExecuteNamedFromSourcesWithFuncs(templateSources, templateName, data, nil)
}

func ExecuteNamedWithFuncs(templateSource, templateName string, data any, funcs template.FuncMap) (string, error) {
	return ExecuteNamedFromSourcesWithFuncs([]string{templateSource}, templateName, data, funcs)
}

func ExecuteNamedFromSourcesWithFuncs(templateSources []string, templateName string, data any, funcs template.FuncMap) (string, error) {
	if len(templateSources) == 0 {
		return "", fmt.Errorf("failed to parse template source: no template sources provided")
	}

	tmpl := template.New("doc")
	if len(funcs) > 0 {
		tmpl = tmpl.Funcs(funcs)
	}

	parsed := tmpl
	for idx, source := range templateSources {
		var err error
		parsed, err = parsed.Parse(source)
		if err != nil {
			if len(templateSources) == 1 {
				return "", fmt.Errorf("failed to parse template source: %w", err)
			}
			return "", fmt.Errorf("failed to parse template source[%d]: %w", idx, err)
		}
	}

	var buf bytes.Buffer
	if err := parsed.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("failed to execute %s template flow: %w", templateName, err)
	}

	return buf.String(), nil
}
