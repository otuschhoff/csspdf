package flowrender

import (
	"errors"
	"fmt"
	htmltmpl "html/template"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

// PreparedFlow caches immutable parsing work for one render. Callers should
// create a new instance for each render; no cross-render cache is retained.
type PreparedFlow struct {
	templateSources  []string
	stylesheet       *templateload.PreparedStylesheet
	mu               sync.Mutex
	templates        map[string]*templateload.PreparedTemplates
	templateParses   int
	stylesheetParses int
}

// PreparationStats reports parsing performed during this prepared flow's
// render-scoped lifetime.
type PreparationStats struct {
	TemplateParses   int
	StylesheetParses int
}

// PrepareFlow parses reusable template and stylesheet inputs for one render.
func PrepareFlow(templateSources []string, cssStyle string) (*PreparedFlow, error) {
	stylesheet, err := templateload.PrepareStylesheet(cssStyle)
	if err != nil {
		return nil, err
	}
	return &PreparedFlow{
		templateSources:  append([]string(nil), templateSources...),
		stylesheet:       stylesheet,
		templates:        make(map[string]*templateload.PreparedTemplates),
		stylesheetParses: 1,
	}, nil
}

// Build executes one flow section using the render-scoped prepared inputs.
func (prepared *PreparedFlow) Build(templateName string, data any, funcs htmltmpl.FuncMap, options BuildOptions) ([]pdfdom.PDFElementNode, error) {
	templates, err := prepared.templatesFor(funcs)
	if err != nil {
		return nil, err
	}
	htmlString, err := templates.Execute(templateName, data, funcs, templateload.ExecuteOptions{Context: options.Context, MaxOutputBytes: options.MaxTemplateOutputBytes})
	if err != nil {
		var limitErr *templateload.OutputLimitError
		if errors.As(err, &limitErr) {
			return nil, limitErr
		}
		return nil, err
	}
	elements, err := pdfdom.ParseHTMLDocFlowPreparedWithOptions(htmlString, prepared.stylesheet, pdfdom.ParseOptions{
		AllowInvalidSpanAttributes: options.AllowInvalidAttributes,
		Warnf:                      options.Warnf,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s template flow: %w", templateName, err)
	}
	if err := validateComplexity(elements, options); err != nil {
		return nil, err
	}
	return elements, nil
}

// Stats returns preparation counts. More than one template parse indicates
// that the render used distinct function-map signatures.
func (prepared *PreparedFlow) Stats() PreparationStats {
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	return PreparationStats{TemplateParses: prepared.templateParses, StylesheetParses: prepared.stylesheetParses}
}

func (prepared *PreparedFlow) templatesFor(funcs htmltmpl.FuncMap) (*templateload.PreparedTemplates, error) {
	key := funcMapSignature(funcs)
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	if templates := prepared.templates[key]; templates != nil {
		return templates, nil
	}
	templates, err := templateload.PrepareTemplates(prepared.templateSources, funcs)
	if err != nil {
		return nil, err
	}
	prepared.templates[key] = templates
	prepared.templateParses++
	return templates, nil
}

func funcMapSignature(funcs htmltmpl.FuncMap) string {
	parts := make([]string, 0, len(funcs))
	for name, function := range funcs {
		functionType := reflect.TypeOf(function)
		typeName := "<nil>"
		if functionType != nil {
			typeName = functionType.String()
		}
		parts = append(parts, name+":"+typeName)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}
