package filemerge

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MergeJSONObjects deep-merges two JSON objects. Overlay keys override base keys.
// Nested objects are merged recursively. The __replace__ sentinel forces atomic replacement.
func MergeJSONObjects(baseJSON []byte, overlayJSON []byte) ([]byte, error) {
	base, err := unmarshalJSONObject(baseJSON)
	if err != nil {
		base = map[string]any{}
	}

	overlay, err := unmarshalJSONObject(overlayJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal overlay json: %w", err)
	}

	merged := mergeObjects(base, overlay)
	encoded, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal merged json: %w", err)
	}

	return append(encoded, '\n'), nil
}

func unmarshalJSONObject(raw []byte) (map[string]any, error) {
	object := map[string]any{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return object, nil
	}

	if err := json.Unmarshal(raw, &object); err == nil {
		return object, nil
	}

	normalized := normalizeJSON(raw)
	if err := json.Unmarshal(normalized, &object); err != nil {
		return nil, err
	}

	return object, nil
}

func normalizeJSON(raw []byte) []byte {
	withoutComments := stripJSONComments(raw)
	return stripTrailingCommas(withoutComments)
}

func stripJSONComments(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out = append(out, ch)
			}
			continue
		}

		if inBlockComment {
			if ch == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == '/' && i+1 < len(raw) {
			next := raw[i+1]
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		out = append(out, ch)
	}

	return out
}

func stripTrailingCommas(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == ',' {
			j := i + 1
			for j < len(raw) {
				next := raw[j]
				if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
					j++
					continue
				}
				if next == '}' || next == ']' {
					ch = 0
				}
				break
			}
		}

		if ch != 0 {
			out = append(out, ch)
		}
	}

	return out
}

const replaceSentinel = "__replace__"

func asSentinel(v any) (any, bool) {
	m, isMap := v.(map[string]any)
	if !isMap {
		return nil, false
	}
	if replacement, hasSentinel := m[replaceSentinel]; hasSentinel && len(m) == 1 {
		return replacement, true
	}
	return nil, false
}

func mergeObjects(base map[string]any, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		result[key] = value
	}

	for key, overlayValue := range overlay {
		if replacement, isSentinel := asSentinel(overlayValue); isSentinel {
			result[key] = replacement
			continue
		}

		baseValue, ok := result[key]
		if !ok {
			if overlayMap, isMap := overlayValue.(map[string]any); isMap {
				result[key] = mergeObjects(map[string]any{}, overlayMap)
			} else {
				result[key] = overlayValue
			}
			continue
		}

		baseMap, baseIsMap := baseValue.(map[string]any)
		overlayMap, overlayIsMap := overlayValue.(map[string]any)
		if baseIsMap && overlayIsMap {
			result[key] = mergeObjects(baseMap, overlayMap)
			continue
		}

		result[key] = overlayValue
	}

	return result
}
