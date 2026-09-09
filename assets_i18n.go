package csspdf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/otuschhoff/csspdf/internal/i18n"
)

func resolveI18nInput(locale string, input RenderInput) (*i18n.I18n, error) {
	limits, err := normalizeRenderLimits(input.Limits)
	if err != nil {
		return nil, err
	}
	return resolveI18nInputContext(context.Background(), locale, input, limits)
}

func resolveI18nInputContext(ctx context.Context, locale string, input RenderInput, limits RenderLimits) (*i18n.I18n, error) {
	i18nSource := input.I18nSource
	resolver := input.ResourceResolver
	if resolver == nil {
		resolver = TrustedFileResolver{}
	}
	if !i18nSource.IsSet() {
		baseDir := strings.TrimSpace(input.AssetBaseDir)
		if baseDir != "" {
			candidate := filepath.Join(baseDir, "i18n.json")
			if data, readErr := resolver.ReadFile(ctx, candidate, limits.SourceBytes); readErr == nil {
				i18nSource = JSONSource{Raw: data}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				return nil, readErr
			}
		}
	}
	if !i18nSource.IsSet() {
		return i18n.New(locale)
	}
	var source map[string]any
	if err := i18nSource.decodeIntoContext(ctx, resolver, limits.SourceBytes, &source, "i18n", false); err != nil {
		return nil, err
	}
	return i18n.NewFromSource(locale, source)
}
