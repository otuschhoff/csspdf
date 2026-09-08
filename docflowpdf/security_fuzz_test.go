package docflowpdf

import "testing"

func FuzzDecodeJSONAndFlowValidation(f *testing.F) {
	f.Add([]byte(`{"mainFlow":[{"template":"doc","transformer":"generic","payload":{}}],"pageNumber":{"template":"page-number","transformer":"generic","payload":{}}}`))
	f.Add([]byte(`{"unknown":true}`))
	f.Add([]byte(`{"mainFlow":"wrong-type","pageNumber":null}`))
	f.Add([]byte(`{"mainFlow":[],"mainFlow":[{"template":"","transformer":"../../generic"}]}`))
	f.Add([]byte(`{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{}}}}}}}}}`))
	f.Add([]byte(`{"mainFlow":[{"template":"doc","transformer":"generic","payload":{"static":{"value":"\ud800"}}}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		var flow Flow
		if err := decodeJSON(data, &flow, true); err == nil {
			_ = flow.Validate()
		}
	})
}
