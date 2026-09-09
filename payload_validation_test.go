package csspdf

import (
	"strings"
	"testing"
)

func TestFlowValidate_RejectsUnsupportedTransformer(t *testing.T) {
	flow := Flow{
		MainFlow:   []Section{{Template: "doc", Transformer: "invoice-doc"}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for unsupported transformer")
	}
}

func TestFlowValidate_RejectsInvalidRuntimeExpression(t *testing.T) {
	flow := Flow{
		MainFlow: []Section{{
			Template:    "doc",
			Transformer: "generic",
			Payload: PayloadConfig{
				Runtime: map[string]string{"Layout.Width": "flow.remainingWidth:20,,40"},
			},
		}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid runtime expression")
	}
}

func TestFlowValidate_RejectsInvalidPayloadPath(t *testing.T) {
	flow := Flow{
		MainFlow: []Section{{
			Template:    "doc",
			Transformer: "generic",
			Payload: PayloadConfig{
				Runtime: map[string]string{"bad-path": "flow.tableWidth"},
			},
		}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid payload path")
	}
}

func TestFlowValidate_RejectsReservedAndConflictingPayloadPaths(t *testing.T) {
	testCases := []struct {
		name    string
		payload PayloadConfig
		want    string
	}{
		{name: "reserved source", payload: PayloadConfig{Static: map[string]any{"Source.customer": "x"}}, want: "reserved root"},
		{name: "reserved page", payload: PayloadConfig{Runtime: map[string]string{"page.width": "flow.tableWidth"}}, want: "reserved root"},
		{name: "same target", payload: PayloadConfig{Runtime: map[string]string{"totals.net": "flow.tableWidth"}, Static: map[string]any{"totals.net": 1}}, want: "defined by both"},
		{name: "path prefix", payload: PayloadConfig{Static: map[string]any{"totals": 1, "totals.net": 2}}, want: "conflict"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			section := Section{Template: "doc", Transformer: "generic", Payload: testCase.payload}
			err := section.Validate()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, testCase.want)
			}
		})
	}
}
