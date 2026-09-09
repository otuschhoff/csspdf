package csspdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type atomicOutputFile interface {
	io.Writer
	Chmod(os.FileMode) error
	Close() error
	Name() string
}

type atomicOutputOps struct {
	createTemp func(string, string) (atomicOutputFile, error)
	stat       func(string) (os.FileInfo, error)
	rename     func(string, string) error
	remove     func(string) error
}

// RenderToFile renders and atomically writes a PDF to the given file path.
func RenderToFile(input RenderInput, outputPath string) error {
	return RenderToFileContext(context.Background(), input, outputPath)
}

// RenderToFileContext renders and atomically writes a PDF with cancellation.
func RenderToFileContext(ctx context.Context, input RenderInput, outputPath string) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	pdf, err := RenderToBytesContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to render PDF for %s: %w", outputPath, err)
	}
	return writeFileAtomically(outputPath, pdf, atomicOutputOps{
		createTemp: func(dir, pattern string) (atomicOutputFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		stat:   os.Stat,
		rename: os.Rename,
		remove: os.Remove,
	})
}

func writeFileAtomically(outputPath string, data []byte, ops atomicOutputOps) error {
	dir := filepath.Dir(outputPath)
	temp, err := ops.createTemp(dir, "."+filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary output for %q: %w", outputPath, err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = ops.remove(tempPath)
	}()

	if info, statErr := ops.stat(outputPath); statErr == nil {
		if err := temp.Chmod(info.Mode().Perm()); err != nil {
			return fmt.Errorf("failed to preserve output permissions for %q: %w", outputPath, err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect output file %q: %w", outputPath, statErr)
	}

	if _, err := io.Copy(temp, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to write temporary PDF for %q: %w", outputPath, err)
	}
	if err := temp.Close(); err != nil {
		closed = true
		return fmt.Errorf("failed to close temporary PDF for %q: %w", outputPath, err)
	}
	closed = true
	if err := ops.rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("failed to replace output file %q: %w", outputPath, err)
	}
	return nil
}
