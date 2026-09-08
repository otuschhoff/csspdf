package pdfdom

import "testing"

func FuzzParseHTMLDocFlow(f *testing.F) {
	f.Add(`<div>Hello <span>world</span></div>`, `div { font-size: 10pt; }`)
	f.Add(`<table width="100"><tr><td>x</td></tr></table>`, `table { border: 1pt solid #000; }`)
	f.Add(`<div><div><div><div><div><span>x`, `div div div div span { width: calc(1/0); }`)
	f.Add(`<table><tr><td rowspan="999999999999999999999">x</table>`, `* { unknown: url(../../secret); }`)
	f.Add(`<img src="../../etc/passwd"><script>{{.}}</script>`, `img:not(:not(:not(.x))) { color: #ggg; }`)
	f.Add("\x00\xff<div>&#999999999999;</div>", "@page { size: -1pt -1pt; margin: 1e999pt; }")
	f.Fuzz(func(t *testing.T, html, css string) {
		if len(html)+len(css) > 64<<10 {
			t.Skip()
		}
		_, _ = ParseHTMLDocFlow(html, css)
	})
}
