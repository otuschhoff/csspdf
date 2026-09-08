package docflowpdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResourceResolver controls filesystem access for render resources.
type ResourceResolver interface {
	ReadFile(ctx context.Context, name string, maxBytes int64) ([]byte, error)
	ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error)
}

// ConfinedFileResolver resolves only regular files beneath Root. os.Root keeps
// symlink resolution and filesystem operations inside the configured root.
type ConfinedFileResolver struct {
	Root string
}

func (r ConfinedFileResolver) ReadFile(ctx context.Context, name string, maxBytes int64) ([]byte, error) {
	if err := validateResourceName(name); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return nil, fmt.Errorf("open confined resource root %q: %w", r.Root, err)
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open confined resource %q: %w", name, err)
	}
	defer file.Close()
	return readRegularFile(ctx, file, name, maxBytes)
}

func (r ConfinedFileResolver) ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	if err := validateResourceName(name); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return nil, fmt.Errorf("open confined resource root %q: %w", r.Root, err)
	}
	defer root.Close()
	dir, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open confined resource directory %q: %w", name, err)
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read confined resource directory %q: %w", name, err)
	}
	return entries, nil
}

// TrustedFileResolver preserves unrestricted host-file access for trusted
// local authoring. It must not be used as a sandbox for tenant-controlled paths.
type TrustedFileResolver struct{}

func (TrustedFileResolver) ReadFile(ctx context.Context, name string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readRegularFile(ctx, file, name, maxBytes)
}

func (TrustedFileResolver) ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadDir(name)
}

func validateResourceName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || !filepath.IsLocal(trimmed) {
		return fmt.Errorf("resource path %q must be a local path beneath the configured root", name)
	}
	return nil
}

func readRegularFile(ctx context.Context, file *os.File, name string, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect resource %q: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("resource %q is not a regular file", name)
	}
	reader := io.Reader(file)
	if maxBytes > 0 {
		reader = io.LimitReader(file, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read resource %q: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, &LimitError{Resource: name, Limit: maxBytes, Actual: int64(len(data))}
	}
	return data, nil
}

// LimitError reports a configured resource budget violation.
type LimitError struct {
	Resource string
	Limit    int64
	Actual   int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("resource %q exceeds byte limit %d", e.Resource, e.Limit)
}

func (e *LimitError) Is(target error) bool {
	if target == ErrLimitExceeded {
		return true
	}
	var other *LimitError
	return errors.As(target, &other)
}
