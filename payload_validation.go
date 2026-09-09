package csspdf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var payloadPathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

var reservedPayloadRoots = map[string]struct{}{
	"Payload": {},
	"Source":  {},
	"i18n":    {},
	"locale":  {},
	"page":    {},
}

func (f Flow) Validate() error {
	if len(f.MainFlow) == 0 {
		return fmt.Errorf("flow must include at least one mainFlow section")
	}
	if f.PageNumber.Template == "" || f.PageNumber.Transformer == "" {
		return fmt.Errorf("flow must define pageNumber template and transformer")
	}
	for i, section := range f.MainFlow {
		if section.Template == "" || section.Transformer == "" {
			return fmt.Errorf("every mainFlow section must define template and transformer")
		}
		if err := section.Validate(); err != nil {
			return fmt.Errorf("mainFlow[%d] (%s): %w", i, section.Template, err)
		}
	}
	if err := f.PageNumber.Validate(); err != nil {
		return fmt.Errorf("pageNumber (%s): %w", f.PageNumber.Template, err)
	}
	return nil
}

func (s Section) Validate() error {
	if s.Template == "" {
		return fmt.Errorf("template must not be empty")
	}
	if s.Transformer != "generic" {
		return fmt.Errorf("unsupported transformer %q", s.Transformer)
	}
	if err := validatePayloadPathMap(s.Payload.Runtime, "runtime"); err != nil {
		return err
	}
	if err := validatePayloadPathMap(s.Payload.Static, "static"); err != nil {
		return err
	}
	if err := validatePayloadTargetPaths(s.Payload.Runtime, s.Payload.Static); err != nil {
		return err
	}
	for name, path := range s.Payload.I18nVars {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid i18n var path %q for key %q", path, name)
		}
	}
	if s.Payload.LocalePath != "" && !payloadPathPattern.MatchString(s.Payload.LocalePath) {
		return fmt.Errorf("invalid localePath %q", s.Payload.LocalePath)
	}
	for _, expr := range s.Payload.Runtime {
		if err := validateRuntimeExpression(expr); err != nil {
			return err
		}
	}
	return nil
}

func validatePayloadPathMap[T any](values map[string]T, kind string) error {
	for path := range values {
		if !payloadPathPattern.MatchString(path) {
			return fmt.Errorf("invalid %s target path %q", kind, path)
		}
	}
	return nil
}

func validatePayloadTargetPaths(runtime map[string]string, static map[string]any) error {
	paths := make([]string, 0, len(runtime)+len(static))
	owners := make(map[string]string, len(runtime)+len(static))
	for path := range runtime {
		paths = append(paths, path)
		owners[path] = "runtime"
	}
	for path := range static {
		if owner, exists := owners[path]; exists {
			return fmt.Errorf("payload target path %q is defined by both %s and static values", path, owner)
		}
		paths = append(paths, path)
		owners[path] = "static"
	}
	sort.Strings(paths)
	for index, path := range paths {
		root := strings.SplitN(path, ".", 2)[0]
		if _, reserved := reservedPayloadRoots[root]; reserved {
			return fmt.Errorf("payload target path %q uses reserved root %q", path, root)
		}
		if index > 0 && strings.HasPrefix(path, paths[index-1]+".") {
			return fmt.Errorf("payload target paths %q and %q conflict", paths[index-1], path)
		}
	}
	return nil
}

func validateRuntimeExpression(expr string) error {
	switch {
	case expr == "flow.tableWidth":
		return nil
	case strings.HasPrefix(expr, "flow.remainingWidth:"):
		spec := strings.TrimPrefix(expr, "flow.remainingWidth:")
		if strings.TrimSpace(spec) == "" {
			return fmt.Errorf("invalid runtime expression %q: missing width offsets", expr)
		}
		for _, part := range strings.Split(spec, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				return fmt.Errorf("invalid runtime expression %q: empty width offset", expr)
			}
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return fmt.Errorf("invalid runtime expression %q: %w", expr, err)
			}
		}
		return nil
	case expr == "page.number":
		return nil
	case expr == "page.total":
		return nil
	default:
		return fmt.Errorf("unsupported runtime expression %q", expr)
	}
}
