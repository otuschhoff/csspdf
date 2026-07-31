package docflowpdf

import "testing"

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
