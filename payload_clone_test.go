package csspdf

import (
	"reflect"
	"testing"
)

func TestCloneFlow_DeepCopiesMutablePayloadConfiguration(t *testing.T) {
	original := Flow{MainFlow: []Section{{
		Template:    "doc",
		Transformer: "generic",
		Payload: PayloadConfig{
			Runtime:  map[string]string{"layout.width": "flow.tableWidth"},
			I18nVars: map[string]string{"name": "customer.name"},
			Static: map[string]any{
				"metadata": map[string]any{"labels": []any{"original"}},
			},
		},
	}}}
	cloned := cloneFlow(original)
	cloned.MainFlow[0].Payload.Runtime["layout.width"] = "page.number"
	cloned.MainFlow[0].Payload.I18nVars["name"] = "other.name"
	cloned.MainFlow[0].Payload.Static["metadata"].(map[string]any)["labels"].([]any)[0] = "changed"

	if reflect.DeepEqual(original, cloned) {
		t.Fatalf("expected modified clone to differ")
	}
	if got := original.MainFlow[0].Payload.Runtime["layout.width"]; got != "flow.tableWidth" {
		t.Fatalf("original runtime map changed: %q", got)
	}
	if got := original.MainFlow[0].Payload.Static["metadata"].(map[string]any)["labels"].([]any)[0]; got != "original" {
		t.Fatalf("original nested static value changed: %v", got)
	}
}
