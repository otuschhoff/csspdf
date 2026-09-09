package csspdf

import (
	"encoding/json"
	htmltmpl "html/template"
)

func aggregateTemplateFuncs() htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		"sumNumbers": func(rows any, key string) float64 {
			items, ok := rows.([]any)
			if !ok {
				return 0
			}
			total := 0.0
			for _, item := range items {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				total += asFloat64(obj[key])
			}
			return total
		},
	}
}

func asFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case json.Number:
		value, err := typed.Float64()
		if err == nil {
			return value
		}
		return 0
	default:
		return 0
	}
}
