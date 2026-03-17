package template

import (
	"bytes"
	"fmt"
	"html/template"
)

func ExecuteNamed(templateSource, templateName string, data any) (string, error) {
	tmpl := template.Must(template.New("doc").Parse(templateSource))

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("failed to execute %s template flow: %w", templateName, err)
	}

	return buf.String(), nil
}
