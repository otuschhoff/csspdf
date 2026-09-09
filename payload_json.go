package csspdf

import (
	"encoding/json"
	"fmt"
)

func asJSONObject(data any) (map[string]any, error) {
	return asJSONObjectWithLimit(data, 0)
}

func asJSONObjectWithLimit(data any, limit int64) (map[string]any, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data to JSON: %w", err)
	}
	if limit > 0 && int64(len(buf)) > limit {
		return nil, &BudgetError{Stage: "source data bytes", Limit: limit, Actual: int64(len(buf))}
	}
	out := make(map[string]any)
	if err := decodeJSON(buf, &out, false); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data from JSON: %w", err)
	}
	return out, nil
}

func requiredString(root map[string]any, key string) string {
	if value, ok := root[key].(string); ok {
		return value
	}
	return ""
}
