package templating

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
)

type PreparedTemplates struct {
	template *template.Template
}

type ExecuteOptions struct {
	Context        context.Context
	MaxOutputBytes int64
}

type OutputLimitError struct {
	Limit int64
}

func (e *OutputLimitError) Error() string {
	return fmt.Sprintf("template output exceeds byte limit %d", e.Limit)
}

func ExecuteNamedWithFuncs(templateSource, templateName string, data any, funcs template.FuncMap) (string, error) {
	return ExecuteNamedFromSourcesWithFuncs([]string{templateSource}, templateName, data, funcs)
}

func ExecuteNamedFromSourcesWithFuncs(templateSources []string, templateName string, data any, funcs template.FuncMap) (string, error) {
	return ExecuteNamedFromSourcesWithOptions(templateSources, templateName, data, funcs, ExecuteOptions{})
}

func ExecuteNamedFromSourcesWithOptions(templateSources []string, templateName string, data any, funcs template.FuncMap, options ExecuteOptions) (string, error) {
	prepared, err := PrepareTemplates(templateSources, funcs)
	if err != nil {
		return "", err
	}
	return prepared.Execute(templateName, data, funcs, options)
}

func PrepareTemplates(templateSources []string, funcs template.FuncMap) (*PreparedTemplates, error) {
	if len(templateSources) == 0 {
		return nil, fmt.Errorf("failed to parse template source: no template sources provided")
	}

	tmpl := template.New("doc").Option("missingkey=error")
	if len(funcs) > 0 {
		tmpl = tmpl.Funcs(funcs)
	}

	parsed := tmpl
	for idx, source := range templateSources {
		var err error
		parsed, err = parsed.Parse(source)
		if err != nil {
			if len(templateSources) == 1 {
				return nil, fmt.Errorf("failed to parse template source: %w", err)
			}
			return nil, fmt.Errorf("failed to parse template source[%d]: %w", idx, err)
		}
	}
	return &PreparedTemplates{template: parsed}, nil
}

func (prepared *PreparedTemplates) Execute(templateName string, data any, funcs template.FuncMap, options ExecuteOptions) (string, error) {
	if prepared == nil || prepared.template == nil {
		return "", fmt.Errorf("prepared template is nil")
	}
	parsed, err := prepared.template.Clone()
	if err != nil {
		return "", fmt.Errorf("failed to clone prepared template: %w", err)
	}
	if len(funcs) > 0 {
		parsed = parsed.Funcs(funcs)
	}

	var buf bytes.Buffer
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	writer := &executionWriter{ctx: ctx, buffer: &buf, remaining: options.MaxOutputBytes, limit: options.MaxOutputBytes}
	if err := parsed.ExecuteTemplate(writer, templateName, data); err != nil {
		return "", fmt.Errorf("failed to execute %s template flow: %w", templateName, err)
	}

	return buf.String(), nil
}

type executionWriter struct {
	ctx       context.Context
	buffer    *bytes.Buffer
	remaining int64
	limit     int64
}

func (w *executionWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if w.limit > 0 && int64(len(data)) > w.remaining {
		return 0, &OutputLimitError{Limit: w.limit}
	}
	written, err := w.buffer.Write(data)
	if w.limit > 0 {
		w.remaining -= int64(written)
	}
	return written, err
}
