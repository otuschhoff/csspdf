package templating

import (
	"bytes"
	"fmt"
	"html/template"
)

func ExecuteNamed(templateSource, templateName string, data any) (string, error) {
	return ExecuteNamedWithFuncs(templateSource, templateName, data, nil)
}

func ExecuteNamedWithFuncs(templateSource, templateName string, data any, funcs template.FuncMap) (string, error) {
	tmpl := template.New("doc")
	if len(funcs) > 0 {
		tmpl = tmpl.Funcs(funcs)
	}
	parsed, err := tmpl.Parse(templateSource)
	if err != nil {
		return "", fmt.Errorf("failed to parse template source: %w", err)
	}

	var buf bytes.Buffer
	if err := parsed.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("failed to execute %s template flow: %w", templateName, err)
	}

	return buf.String(), nil
}
