package filemerge

import (
	"encoding/json"
	"testing"
)

func TestMergeJSONObjects_Simple(t *testing.T) {
	base := []byte(`{"a": 1, "b": 2}`)
	overlay := []byte(`{"b": 3, "c": 4}`)

	result, err := MergeJSONObjects(base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if m["a"] != float64(1) {
		t.Errorf("a = %v, want 1", m["a"])
	}
	if m["b"] != float64(3) {
		t.Errorf("b = %v, want 3 (overlay)", m["b"])
	}
	if m["c"] != float64(4) {
		t.Errorf("c = %v, want 4", m["c"])
	}
}

func TestMergeJSONObjects_DeepMerge(t *testing.T) {
	base := []byte(`{"mcp": {"cortex": {"enabled": true}, "other": {"key": "val"}}}`)
	overlay := []byte(`{"mcp": {"cortex": {"args": ["mcp"]}}}`)

	result, err := MergeJSONObjects(base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	mcp := m["mcp"].(map[string]any)
	cortex := mcp["cortex"].(map[string]any)
	if cortex["enabled"] != true {
		t.Error("expected cortex.enabled to be preserved")
	}
	if cortex["args"] == nil {
		t.Error("expected cortex.args to be added")
	}
	if mcp["other"] == nil {
		t.Error("expected mcp.other to be preserved")
	}
}

func TestMergeJSONObjects_ReplaceSentinel(t *testing.T) {
	base := []byte(`{"mcp": {"cortex": {"old": true, "stale": true}}}`)
	overlay := []byte(`{"mcp": {"cortex": {"__replace__": {"command": "cortex", "args": ["mcp"]}}}}`)

	result, err := MergeJSONObjects(base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	cortex := m["mcp"].(map[string]any)["cortex"].(map[string]any)
	if cortex["old"] != nil {
		t.Error("expected old keys to be replaced")
	}
	if cortex["command"] != "cortex" {
		t.Errorf("command = %v, want cortex", cortex["command"])
	}
}

func TestMergeJSONObjects_EmptyBase(t *testing.T) {
	overlay := []byte(`{"key": "value"}`)

	result, err := MergeJSONObjects(nil, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}
}

func TestMergeJSONObjects_WithComments(t *testing.T) {
	base := []byte(`{
		// this is a comment
		"a": 1,
		"b": 2, // trailing
	}`)
	overlay := []byte(`{"c": 3}`)

	result, err := MergeJSONObjects(base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if m["a"] != float64(1) {
		t.Errorf("a = %v, want 1", m["a"])
	}
	if m["c"] != float64(3) {
		t.Errorf("c = %v, want 3", m["c"])
	}
}
