package templating

import (
	"html/template"
	"strings"
	"testing"
)

func TestExecuteNamedWithFuncs_ParseErrorReturnsError(t *testing.T) {
	_, err := ExecuteNamedWithFuncs("{{define \"doc\"}}", "doc", nil, nil)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "failed to parse template source") {
		t.Fatalf("expected parse error wrapper, got %v", err)
	}
}

func TestPreparedTemplatesBindCurrentFunctions(t *testing.T) {
	prepared, err := PrepareTemplates([]string{`{{define "doc"}}{{value}}{{end}}`}, template.FuncMap{"value": func() string { return "initial" }})
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepared.Execute("doc", nil, template.FuncMap{"value": func() string { return "current" }}, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "current" {
		t.Fatalf("expected current function implementation, got %q", result)
	}
}
