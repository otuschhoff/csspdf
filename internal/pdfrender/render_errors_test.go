package pdfrender

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

var errInjectedRender = errors.New("injected render failure")

type recoverableCase struct {
	name        string
	wantMessage string
	failAt      int
	elements    func(testing.TB) []pdfdom.PDFElementNode
}

func TestRecoverableRenderSitesHonorStrictAndLegacyModes(t *testing.T) {
	cases := []recoverableCase{
		{name: "block initial measure", wantMessage: "failed to measure doc flow element", failAt: 1, elements: blockWithImage(false)},
		{name: "block final measure", wantMessage: "failed to measure doc flow element", failAt: 1, elements: blockWithImage(true)},
		{name: "block render", wantMessage: "failed to render doc flow element", failAt: 3, elements: blockWithImage(false)},
		{name: "image initial measure", wantMessage: "failed to measure doc image", failAt: 1, elements: imageElements(false)},
		{name: "image final measure", wantMessage: "failed to measure doc image", failAt: 1, elements: imageElements(true)},
		{name: "image render", wantMessage: "failed to render doc image", failAt: 3, elements: imageElements(false)},
		{name: "running footer registration", wantMessage: "failed to register running footer", elements: runningFooterWithMissingTemplate},
		{name: "use-template", wantMessage: "failed to render use-template element", elements: missingUseTemplate},
		{name: "create-template", wantMessage: "failed to create template", elements: invalidCreateTemplate},
		{name: "create-template child", wantMessage: "failed to create template", failAt: 1, elements: createTemplateWithImage},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, strict := range []bool{true, false} {
				mode := "legacy"
				if strict {
					mode = "strict"
				}
				t.Run(mode, func(t *testing.T) {
					layout, warnings := recoverableTestLayout(t, strict, testCase.failAt)
					err := RenderDocTemplateFlow(layout, testCase.elements(t))
					if strict {
						if err == nil || !strings.Contains(err.Error(), testCase.wantMessage) {
							t.Fatalf("expected error containing %q, got %v", testCase.wantMessage, err)
						}
						if testCase.failAt > 0 && !errors.Is(err, errInjectedRender) {
							t.Fatalf("strict error did not preserve injected cause: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatalf("legacy render returned error: %v", err)
					}
					if !containsWarning(warnings, testCase.wantMessage) {
						t.Fatalf("warnings %q do not contain %q", *warnings, testCase.wantMessage)
					}
				})
			}
		})
	}
}

func recoverableTestLayout(t testing.TB, strict bool, failAt int) (*LayoutPDF, *[]string) {
	t.Helper()
	settings := templateload.PageSettings{Width: 300, Height: 400, Margins: templateload.PageMargins{Top: 20, Right: 20, Bottom: 20, Left: 20}}
	imageData := onePixelPNG(t)
	loads := 0
	layout, err := NewLayoutPDFWithOptions(settings, settings, nil, nil, LayoutOptions{
		StrictRenderErrors: strict,
		ImageLoader: func(name string) (ImageResource, error) {
			loads++
			if failAt > 0 && loads == failAt {
				return ImageResource{}, errInjectedRender
			}
			return ImageResource{Name: name, Type: "PNG", Data: imageData}, nil
		},
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}
	layout.StartFlow()
	warnings := []string{}
	layout.SetWarningFunc(func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	})
	return layout, &warnings
}

func onePixelPNG(t testing.TB) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return data.Bytes()
}

func blockWithImage(absolute bool) func(testing.TB) []pdfdom.PDFElementNode {
	return func(t testing.TB) []pdfdom.PDFElementNode {
		block := pdfdom.NewElemDiv()
		if absolute {
			block.SetAttribute("position", "absolute")
		}
		mustAddNode(t, block, testImage())
		return []pdfdom.PDFElementNode{block}
	}
}

func imageElements(absolute bool) func(testing.TB) []pdfdom.PDFElementNode {
	return func(testing.TB) []pdfdom.PDFElementNode {
		image := testImage()
		if absolute {
			image.SetAttribute("position", "absolute")
		}
		return []pdfdom.PDFElementNode{image}
	}
}

func testImage() *pdfdom.ElemImg {
	image := pdfdom.NewElemImg()
	image.SetAttribute("src", "test.png")
	image.SetAttribute("width", "10")
	image.SetAttribute("height", "10")
	return image
}

func runningFooterWithMissingTemplate(t testing.TB) []pdfdom.PDFElementNode {
	footer := pdfdom.NewElemDiv()
	footer.SetAttribute("position", "running(site-footer)")
	footer.SetAttribute("height", "20")
	useTemplate := pdfdom.NewElemUseTemplate()
	useTemplate.SetAttribute("name", "missing")
	mustAddNode(t, footer, useTemplate)
	return []pdfdom.PDFElementNode{footer}
}

func missingUseTemplate(testing.TB) []pdfdom.PDFElementNode {
	element := pdfdom.NewElemUseTemplate()
	element.SetAttribute("name", "missing")
	return []pdfdom.PDFElementNode{element}
}

func invalidCreateTemplate(testing.TB) []pdfdom.PDFElementNode {
	return []pdfdom.PDFElementNode{pdfdom.NewElemCreateTemplate()}
}

func createTemplateWithImage(t testing.TB) []pdfdom.PDFElementNode {
	element := pdfdom.NewElemCreateTemplate()
	element.SetAttribute("name", "broken")
	element.SetAttribute("width", "100")
	element.SetAttribute("height", "100")
	mustAddNode(t, element, testImage())
	return []pdfdom.PDFElementNode{element}
}

func containsWarning(warnings *[]string, substring string) bool {
	for _, warning := range *warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}
