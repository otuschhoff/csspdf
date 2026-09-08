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

func FuzzRenderHTMLDoesNotPanic(f *testing.F) {
	f.Add(`<div>Hello <span font-size="12pt">world</span></div>`, false)
	f.Add(`<div><span font-size="invalid">text</span></div>`, false)
	f.Add(`<div><span border-width="NaN">text</span></div>`, true)
	f.Add(`<create-template name="x"><div><img src="missing.png"></div></create-template>`, true)
	f.Fuzz(func(t *testing.T, body string, allowPartial bool) {
		if len(body) > 16<<10 {
			t.Skip()
		}
		assets := minimalAssets()
		assets.HTML = `{{define "doc"}}` + body + `{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
		_, _ = RenderToBytes(RenderInput{
			Assets:             assets,
			AllowPartialRender: allowPartial,
			Limits: RenderLimits{
				TemplateOutputBytes: 64 << 10,
				OutputBytes:         1 << 20,
				Nodes:               2_000,
				Depth:               64,
				Rows:                500,
				Pages:               20,
			},
		})
	})
}
