package template

import (
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
