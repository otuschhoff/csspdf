package docflowpdf

import (
	"errors"
	"fmt"

	limitmarker "github.com/otuschhoff/csspdf/internal/limit"
)

// ErrLimitExceeded identifies failures caused by a configured resource or
// rendering limit.
var ErrLimitExceeded = limitmarker.ErrExceeded

// RenderLimits bounds resource use during one render. Zero values select the
// documented defaults; negative values are invalid.
type RenderLimits struct {
	SourceBytes         int64
	TemplateOutputBytes int64
	ImageBytes          int64
	ImagePixels         int64
	OutputBytes         int64
	Nodes               int
	Depth               int
	Rows                int
	Pages               int
}

const (
	defaultSourceBytes         = 16 << 20
	defaultTemplateOutputBytes = 32 << 20
	defaultImageBytes          = 32 << 20
	defaultImagePixels         = 40_000_000
	defaultOutputBytes         = 128 << 20
	defaultNodes               = 100_000
	defaultDepth               = 256
	defaultRows                = 50_000
	defaultPages               = 1_000
)

// DefaultRenderLimits returns an independent copy of the default render
// budgets.
func DefaultRenderLimits() RenderLimits {
	return RenderLimits{
		SourceBytes:         defaultSourceBytes,
		TemplateOutputBytes: defaultTemplateOutputBytes,
		ImageBytes:          defaultImageBytes,
		ImagePixels:         defaultImagePixels,
		OutputBytes:         defaultOutputBytes,
		Nodes:               defaultNodes,
		Depth:               defaultDepth,
		Rows:                defaultRows,
		Pages:               defaultPages,
	}
}

func normalizeRenderLimits(limits RenderLimits) (RenderLimits, error) {
	defaults := DefaultRenderLimits()
	byteLimits := []struct {
		value        *int64
		defaultValue int64
	}{
		{&limits.SourceBytes, defaults.SourceBytes},
		{&limits.TemplateOutputBytes, defaults.TemplateOutputBytes},
		{&limits.ImageBytes, defaults.ImageBytes},
		{&limits.ImagePixels, defaults.ImagePixels},
		{&limits.OutputBytes, defaults.OutputBytes},
	}
	countLimits := []struct {
		value        *int
		defaultValue int
	}{
		{&limits.Nodes, defaults.Nodes},
		{&limits.Depth, defaults.Depth},
		{&limits.Rows, defaults.Rows},
		{&limits.Pages, defaults.Pages},
	}
	for _, setting := range byteLimits {
		if *setting.value < 0 {
			return RenderLimits{}, fmt.Errorf("render limits must not be negative")
		}
		if *setting.value == 0 {
			*setting.value = setting.defaultValue
		}
	}
	for _, setting := range countLimits {
		if *setting.value < 0 {
			return RenderLimits{}, fmt.Errorf("render limits must not be negative")
		}
		if *setting.value == 0 {
			*setting.value = setting.defaultValue
		}
	}
	return limits, nil
}

// BudgetError reports an operational render limit violation.
type BudgetError struct {
	Stage  string
	Limit  int64
	Actual int64
}

func (e *BudgetError) Error() string {
	if e.Actual > 0 {
		return fmt.Sprintf("%s budget exceeded: limit=%d actual=%d", e.Stage, e.Limit, e.Actual)
	}
	return fmt.Sprintf("%s budget exceeded: limit=%d", e.Stage, e.Limit)
}

func (e *BudgetError) Is(target error) bool {
	if target == ErrLimitExceeded {
		return true
	}
	var other *BudgetError
	return errors.As(target, &other)
}
