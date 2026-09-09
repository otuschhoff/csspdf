package csspdf

import (
	"context"
	"io"
)

type budgetWriter struct {
	writer    io.Writer
	remaining int64
	limit     int64
	stage     string
	ctx       context.Context
}

func (w *budgetWriter) Write(data []byte) (int, error) {
	if w.ctx != nil {
		if err := w.ctx.Err(); err != nil {
			return 0, err
		}
	}
	if int64(len(data)) > w.remaining {
		return 0, &BudgetError{Stage: w.stage, Limit: w.limit, Actual: w.limit + int64(len(data)) - w.remaining}
	}
	written, err := w.writer.Write(data)
	w.remaining -= int64(written)
	if err == nil && w.ctx != nil {
		err = w.ctx.Err()
	}
	return written, err
}
