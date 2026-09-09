package csspdf

import (
	"errors"
	"fmt"
)

var errI18nMacroExpansion = errors.New("i18n macro expansion failed")

type i18nTemplateValueError struct {
	Path string
	Err  error
}

func (e *i18nTemplateValueError) Error() string {
	return fmt.Sprintf("path %s: %v", e.Path, e.Err)
}

func (e *i18nTemplateValueError) Unwrap() error {
	return e.Err
}

type fatalFlowRenderError struct {
	err error
}

func (e *fatalFlowRenderError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *fatalFlowRenderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}
