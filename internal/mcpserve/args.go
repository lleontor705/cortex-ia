package mcpserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

func optString(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func reqString(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

// presentString reports a string argument that was supplied, even when empty.
func presentString(args map[string]any, key string) (string, bool) {
	value, ok := args[key].(string)
	return value, ok
}

func optStringList(args map[string]any, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of strings", key)
		}
		out = append(out, text)
	}
	return out, nil
}

// asInt64 accepts integral JSON numbers only.
func asInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		n, err := typed.Int64()
		return n, err == nil
	case float64:
		if typed != math.Trunc(typed) || math.Abs(typed) > 1<<53 {
			return 0, false
		}
		return int64(typed), true
	case int64:
		return typed, true
	}
	return 0, false
}

// reqRevision is the compare-and-set revision every transition and approval
// must carry.
func reqRevision(args map[string]any) (int64, error) {
	raw, ok := args["revision"]
	if !ok || raw == nil {
		return 0, errors.New("revision is required (the current task revision from the claim receipt or cortex_ia_work_status)")
	}
	revision, ok := asInt64(raw)
	if !ok || revision <= 0 {
		return 0, errors.New("revision must be a positive integer")
	}
	return revision, nil
}

func optInt(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", nil
	}
	n, ok := asInt64(raw)
	if !ok {
		return "", fmt.Errorf("%s must be an integer", key)
	}
	return strconv.FormatInt(n, 10), nil
}

func schemaString(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func schemaNumber(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

func schemaEnum(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func schemaStrings(description string) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": map[string]any{"type": "string"}}
}

func schemaArray(description string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": items}
}

// schemaObject builds an object schema; required lists the mandatory keys.
func schemaObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func mergeProps(parts ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, part := range parts {
		for key, value := range part {
			out[key] = value
		}
	}
	return out
}

func props(pairs ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i].(string)] = pairs[i+1]
	}
	return out
}
