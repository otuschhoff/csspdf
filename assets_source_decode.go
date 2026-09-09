package csspdf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

func (s TextSource) resolve(ctx context.Context, resolver ResourceResolver, maxBytes int64, label string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if value, resolved, err := s.resolveInline(maxBytes, label); resolved {
		return value, err
	}
	if s.FilePath != "" {
		buf, err := resolver.ReadFile(ctx, s.FilePath, maxBytes)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		return string(buf), nil
	}
	if s.FS != nil && s.FSPath != "" {
		buf, err := readFSFile(ctx, s.FS, s.FSPath, maxBytes)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from fs path %q: %w", label, s.FSPath, err)
		}
		return string(buf), nil
	}
	return "", fmt.Errorf("missing %s source", label)
}

func (s TextSource) resolveInline(maxBytes int64, label string) (string, bool, error) {
	if s.Raw != nil {
		return boundedSourceText(string(s.Raw), maxBytes, label)
	}
	if s.Text != "" {
		return boundedSourceText(s.Text, maxBytes, label)
	}
	return "", false, nil
}

func boundedSourceText(value string, maxBytes int64, label string) (string, bool, error) {
	if maxBytes > 0 && int64(len(value)) > maxBytes {
		return "", true, &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(value))}
	}
	return value, true, nil
}

func (s JSONSource) decodeInto(target any, label string, disallowUnknownFields bool) error {
	return s.decodeIntoContext(context.Background(), TrustedFileResolver{}, 0, target, label, disallowUnknownFields)
}

func (s JSONSource) decodeIntoContext(ctx context.Context, resolver ResourceResolver, maxBytes int64, target any, label string, disallowUnknownFields bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Object != nil {
		return decodeJSONObject(s.Object, target, label, maxBytes, disallowUnknownFields)
	}
	buf, err := s.resolveJSONBytes(ctx, resolver, maxBytes, label)
	if err != nil {
		return err
	}
	if maxBytes > 0 && int64(len(buf)) > maxBytes {
		return &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(buf))}
	}

	if err := decodeJSON(buf, target, disallowUnknownFields); err != nil {
		return fmt.Errorf("failed to parse %s JSON: %w", label, err)
	}
	return nil
}

func decodeJSONObject(object, target any, label string, maxBytes int64, disallowUnknownFields bool) error {
	buf, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("failed to marshal %s object: %w", label, err)
	}
	if maxBytes > 0 && int64(len(buf)) > maxBytes {
		return &BudgetError{Stage: label + " source bytes", Limit: maxBytes, Actual: int64(len(buf))}
	}
	if err := decodeJSON(buf, target, disallowUnknownFields); err != nil {
		return fmt.Errorf("failed to decode %s object: %w", label, err)
	}
	return nil
}

func (s JSONSource) resolveJSONBytes(ctx context.Context, resolver ResourceResolver, maxBytes int64, label string) ([]byte, error) {
	switch {
	case s.Raw != nil:
		return s.Raw, nil
	case s.Text != "":
		return []byte(s.Text), nil
	case s.FilePath != "":
		data, err := resolver.ReadFile(ctx, s.FilePath, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s from file %q: %w", label, s.FilePath, err)
		}
		return data, nil
	case s.FS != nil && s.FSPath != "":
		data, err := readFSFile(ctx, s.FS, s.FSPath, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s from fs path %q: %w", label, s.FSPath, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("missing %s source", label)
	}
}

func readFSFile(ctx context.Context, sourceFS fs.FS, name string, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := sourceFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := io.Reader(file)
	if maxBytes > 0 {
		reader = io.LimitReader(file, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, &LimitError{Resource: name, Limit: maxBytes, Actual: int64(len(data))}
	}
	return data, ctx.Err()
}

func decodeJSON(data []byte, target any, disallowUnknownFields bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
