package flowrender

import (
	"context"
	"fmt"

	limitmarker "github.com/otuschhoff/csspdf/internal/limit"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

type BuildOptions struct {
	Context                context.Context
	MaxTemplateOutputBytes int64
	MaxNodes               int
	MaxRows                int
	MaxDepth               int
	Complexity             *ComplexityBudget
	AllowInvalidAttributes bool
	Warnf                  func(string, ...any)
}

type ComplexityBudget struct {
	Nodes int
	Rows  int
}

type ComplexityLimitError struct {
	Kind   string
	Limit  int
	Actual int
}

func (e *ComplexityLimitError) Error() string {
	return fmt.Sprintf("%s limit exceeded: limit=%d actual=%d", e.Kind, e.Limit, e.Actual)
}

func (e *ComplexityLimitError) Is(target error) bool {
	return target == limitmarker.ErrExceeded
}

type complexityValidator struct {
	options BuildOptions
	budget  *ComplexityBudget
}

func validateComplexity(elements []pdfdom.PDFElementNode, options BuildOptions) error {
	budget := options.Complexity
	if budget == nil {
		budget = &ComplexityBudget{}
	}
	validator := complexityValidator{options: options, budget: budget}
	for _, element := range elements {
		if err := validator.visit(element, 1); err != nil {
			return err
		}
	}
	return nil
}

func (v *complexityValidator) visit(node pdfdom.PDFNode, depth int) error {
	if node == nil {
		return nil
	}
	if err := v.countNode(depth); err != nil {
		return err
	}
	if element, ok := node.(pdfdom.PDFElementNode); ok {
		return v.visitElement(element, depth)
	}
	return v.visitContainerChildren(node, depth)
}

func (v *complexityValidator) countNode(depth int) error {
	if v.options.MaxDepth > 0 && depth > v.options.MaxDepth {
		return &ComplexityLimitError{Kind: "PDFDOM depth", Limit: v.options.MaxDepth, Actual: depth}
	}
	v.budget.Nodes++
	if v.options.MaxNodes > 0 && v.budget.Nodes > v.options.MaxNodes {
		return &ComplexityLimitError{Kind: "PDFDOM nodes", Limit: v.options.MaxNodes, Actual: v.budget.Nodes}
	}
	return nil
}

func (v *complexityValidator) visitElement(element pdfdom.PDFElementNode, depth int) error {
	if _, isRow := element.(*pdfdom.ElemTr); isRow {
		v.budget.Rows++
		if v.options.MaxRows > 0 && v.budget.Rows > v.options.MaxRows {
			return &ComplexityLimitError{Kind: "table rows", Limit: v.options.MaxRows, Actual: v.budget.Rows}
		}
	}
	return v.visitChildren(element.ElementChildren(), depth)
}

func (v *complexityValidator) visitContainerChildren(node pdfdom.PDFNode, depth int) error {
	switch typed := node.(type) {
	case *pdfdom.PDFTextNode:
		return v.visitChildren(typed.Children, depth)
	case *pdfdom.PDFDocumentNode:
		return v.visitChildren(typed.Children, depth)
	default:
		return nil
	}
}

func (v *complexityValidator) visitChildren(children []pdfdom.PDFNode, depth int) error {
	for _, child := range children {
		if err := v.visit(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}
