package csspdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/otuschhoff/csspdf/internal/flowrender"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func emitArtifactContext(ctx context.Context, artifact *renderArtifact, out io.Writer, outputLimit int64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before emission: %w", err)
	}
	limited := &budgetWriter{writer: out, remaining: outputLimit, limit: outputLimit, stage: "PDF output bytes", ctx: ctx}
	if err := artifact.layout.PDF.Output(limited); err != nil {
		return fmt.Errorf("failed to output PDF: %w", err)
	}
	return nil
}

func renderMainFlow(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput) error {
	prepared, err := flowrender.PrepareFlow(templateSourcesInRenderOrder(assets), assets.CSS)
	if err != nil {
		return err
	}
	return renderMainFlowPrepared(renderCtx, limits, complexity, prepared, layout, assets, source, input)
}

func renderMainFlowPrepared(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, prepared *flowrender.PreparedFlow, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput) error {
	ctx := transformContext{
		Layout:  layout,
		Source:  source,
		Input:   input,
		Context: renderCtx,
		Limits:  limits,
	}
	for _, section := range assets.Flow.MainFlow {
		if err := renderCtx.Err(); err != nil {
			return fmt.Errorf("render canceled before section %q: %w", section.Template, err)
		}
		payload, err := transformSectionPayload(section, ctx)
		if err != nil {
			var diagnosticErr *DiagnosticError
			if errors.As(err, &diagnosticErr) {
				return err
			}
			return &DiagnosticError{Code: DiagnosticTemplate, Stage: "payload", Section: section.Template, Err: err}
		}
		jsonData, err := asJSONObject(payload)
		if err != nil {
			return &DiagnosticError{Code: DiagnosticTemplate, Stage: "payload", Section: section.Template, Err: err}
		}
		funcs := buildFuncMap(input, strings.TrimSpace(input.DefaultLocale), requiredString(jsonData, "locale"))

		elements, err := prepared.Build(section.Template, jsonData, funcs, flowrender.BuildOptions{
			Context: renderCtx, MaxTemplateOutputBytes: limits.TemplateOutputBytes, MaxNodes: limits.Nodes, MaxDepth: limits.Depth, MaxRows: limits.Rows, Complexity: complexity,
			AllowInvalidAttributes: input.AllowPartialRender, Warnf: warningFunc(input),
		})
		if err != nil {
			var outputLimit *templateload.OutputLimitError
			if errors.As(err, &outputLimit) {
				return &BudgetError{Stage: "template output bytes", Limit: outputLimit.Limit}
			}
			var complexityLimit *flowrender.ComplexityLimitError
			if errors.As(err, &complexityLimit) {
				return &BudgetError{Stage: complexityLimit.Kind, Limit: int64(complexityLimit.Limit), Actual: int64(complexityLimit.Actual)}
			}
			return &DiagnosticError{Code: diagnosticCode(err, DiagnosticTemplate), Stage: "template", Section: section.Template, Err: err}
		}
		if err := pdfrender.RenderDocTemplateFlow(layout, elements); err != nil {
			var pageLimit *pdfrender.PageLimitError
			if errors.As(err, &pageLimit) {
				return &BudgetError{Stage: "pages", Limit: int64(pageLimit.Limit), Actual: int64(pageLimit.Requested)}
			}
			return &fatalFlowRenderError{err: &DiagnosticError{Code: DiagnosticLayout, Stage: "layout", Section: section.Template, Err: err}}
		}
	}
	return nil
}

func pageNumberTemplateFlowElements(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, page, total int, input RenderInput) ([]pdfdom.PDFElementNode, error) {
	prepared, err := flowrender.PrepareFlow(templateSourcesInRenderOrder(assets), assets.CSS)
	if err != nil {
		return nil, err
	}
	return pageNumberTemplateFlowElementsPrepared(renderCtx, limits, complexity, prepared, layout, assets, source, page, total, input)
}

func pageNumberTemplateFlowElementsPrepared(renderCtx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, prepared *flowrender.PreparedFlow, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, page, total int, input RenderInput) ([]pdfdom.PDFElementNode, error) {
	payload, err := transformSectionPayload(assets.Flow.PageNumber, transformContext{Layout: layout, Source: source, Page: page, Total: total, Input: input, Context: renderCtx, Limits: limits})
	if err != nil {
		return nil, err
	}
	data, err := asJSONObject(payload)
	if err != nil {
		return nil, err
	}
	funcs := buildFuncMap(input, layout.I18n.Locale(), requiredString(data, "locale"))

	elements, err := prepared.Build(assets.Flow.PageNumber.Template, data, funcs, flowrender.BuildOptions{
		Context: renderCtx, MaxTemplateOutputBytes: limits.TemplateOutputBytes, MaxNodes: limits.Nodes, MaxDepth: limits.Depth, MaxRows: limits.Rows, Complexity: complexity,
		AllowInvalidAttributes: input.AllowPartialRender, Warnf: warningFunc(input),
	})
	if err != nil {
		var outputLimit *templateload.OutputLimitError
		if errors.As(err, &outputLimit) {
			return nil, &BudgetError{Stage: "template output bytes", Limit: outputLimit.Limit}
		}
		var complexityLimit *flowrender.ComplexityLimitError
		if errors.As(err, &complexityLimit) {
			return nil, &BudgetError{Stage: complexityLimit.Kind, Limit: int64(complexityLimit.Limit), Actual: int64(complexityLimit.Actual)}
		}
		return nil, err
	}
	return elements, nil
}
