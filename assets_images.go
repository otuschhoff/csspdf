package csspdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

func confinedImageLoader(ctx context.Context, input RenderInput, limits RenderLimits) pdfrender.ImageLoader {
	if input.ResourceResolver == nil {
		return nil
	}
	return func(name string) (pdfrender.ImageResource, error) {
		candidates := []string{name}
		base := strings.TrimSpace(input.AssetBaseDir)
		candidates = append(candidates, filepath.Join(base, "images", name), filepath.Join(base, "images", filepath.Base(name)))
		var lastErr error
		for _, candidate := range candidates {
			data, err := input.ResourceResolver.ReadFile(ctx, candidate, limits.ImageBytes)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return pdfrender.ImageResource{}, fmt.Errorf("load image resource %q: %w", candidate, err)
				}
				lastErr = err
				continue
			}
			imageType, pixels, ok := supportedImageType(data)
			if !ok {
				return pdfrender.ImageResource{}, fmt.Errorf("image resource %q is not a supported PNG, JPEG, or GIF file", candidate)
			}
			if pixels > limits.ImagePixels {
				return pdfrender.ImageResource{}, &BudgetError{Stage: "decoded image pixels", Limit: limits.ImagePixels, Actual: pixels}
			}
			return pdfrender.ImageResource{Name: "confined:" + candidate, Type: imageType, Data: data}, nil
		}
		return pdfrender.ImageResource{}, fmt.Errorf("image resource %q is unavailable: %w", name, lastErr)
	}
}

func supportedImageType(data []byte) (string, int64, bool) {
	var imageType string
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		imageType = "PNG"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		imageType = "JPG"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		imageType = "GIF"
	default:
		return "", 0, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", 0, false
	}
	width, height := int64(config.Width), int64(config.Height)
	if width > math.MaxInt64/height {
		return "", 0, false
	}
	return imageType, width * height, true
}
