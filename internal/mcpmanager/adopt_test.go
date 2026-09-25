package mcpmanager

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func adoptWriteConfig(t *testing.T, home string, entry map[string]any) string {
	t.Helper()
	path := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	doc := map[string]any{"mcp": map[string]any{"servers": map[string]any{"cortex": entry}}}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func adoptReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func cortexEntry(t *testing.T) map[string]any {
	t.Helper()
	preset, ok := Lookup("cortex")
	if !ok {
		t.Fatal("cortex preset is missing from the catalog")
	}
	return preset.Entry
}

func TestAdoptableEntryClassification(t *testing.T) {
	cases := []struct {
		name   string
		status EntryStatus
		want   bool
	}{
		{"cortex", StatusUnmanagedEquivalent, true},
		{"cortex", StatusManaged, false},
		{"cortex", StatusConflict, false},
		{"cortex", StatusAbsent, false},
		{"forgespec", StatusUnmanagedEquivalent, false},
		{"not-a-preset", StatusUnmanagedEquivalent, false},
	}
	for _, tc := range cases {
		if got := AdoptableEntry(tc.name, tc.status); got != tc.want {
			t.Errorf("AdoptableEntry(%q, %q) = %v, want %v", tc.name, tc.status, got, tc.want)
		}
	}
}

func TestAdoptFailClosedMatrix(t *testing.T) {
	t.Run("UnknownName", func(t *testing.T) {
		manager := New(t.TempDir())
		_, err := manager.Adopt("not-a-preset", nil)
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Kind != ConflictUnmanaged {
			t.Fatalf("want ConflictUnmanaged, got %v", err)
		}
	})

	t.Run("AbsentEntry", func(t *testing.T) {
		manager := New(t.TempDir())
		_, err := manager.Adopt("cortex", nil)
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Kind != ConflictAbsent {
			t.Fatalf("want ConflictAbsent, got %v", err)
		}
	})

	t.Run("DifferentEntry", func(t *testing.T) {
		home := t.TempDir()
		path := adoptWriteConfig(t, home, map[string]any{
			"type":    "local",
			"command": []any{"different-binary", "serve"},
		})
		before := adoptReadBytes(t, path)
		manager := New(home)
		_, err := manager.Adopt("cortex", nil)
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Kind != ConflictModified {
			t.Fatalf("want ConflictModified, got %v", err)
		}
		if string(adoptReadBytes(t, path)) != string(before) {
			t.Fatal("a failed adopt mutated the config")
		}
	})
}

func TestAdoptAccreditsEquivalentEntry(t *testing.T) {
	home := t.TempDir()
	path := adoptWriteConfig(t, home, cortexEntry(t))
	before := adoptReadBytes(t, path)
	manager := New(home)

	result, err := manager.Adopt("cortex", nil)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if result.Action != "adopted" || !result.Configured || result.Changed {
		t.Fatalf("unexpected adopt result: %+v", result)
	}
	if result.Ownership == nil {
		t.Fatal("adopt did not return an ownership record")
	}
	wantDigest, err := SemanticDigest("cortex", cortexEntry(t))
	if err != nil {
		t.Fatalf("SemanticDigest: %v", err)
	}
	if result.Ownership.Digest != wantDigest || result.Ownership.ConfigPath != path {
		t.Fatalf("unexpected ownership record: %+v", result.Ownership)
	}
	if string(adoptReadBytes(t, path)) != string(before) {
		t.Fatal("adopt rewrote the config file")
	}

	again, err := manager.Adopt("cortex", []OwnershipRecord{*result.Ownership})
	if err != nil {
		t.Fatalf("re-adopt: %v", err)
	}
	if again.Action != "already-present" || !again.Configured {
		t.Fatalf("re-adopt was not an idempotent no-op: %+v", again)
	}
}

func TestAdoptAcceptsV1EquivalentField(t *testing.T) {
	home := t.TempDir()
	adoptWriteConfig(t, home, map[string]any{
		"type":    "local",
		"command": []any{"cortex", "mcp", "--tools=agent"},
		"enabled": true,
	})
	manager := New(home)
	result, err := manager.Adopt("cortex", nil)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if result.Action != "adopted" {
		t.Fatalf("want adopted, got %+v", result)
	}
}

func TestAdoptReportsObservedIdentityForNameDivergentEntry(t *testing.T) {
	home := t.TempDir()
	entry := cortexEntry(t)
	entry["env"] = map[string]any{"CORTEX_USER_TOKEN": "placeholder"}
	adoptWriteConfig(t, home, entry)
	manager := New(home)

	result, err := manager.Adopt("cortex", nil)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if result.Action != "adopted" || result.ObservedIdentity == nil {
		t.Fatalf("unexpected adopt result: %+v", result)
	}
	observedDigest, err := SemanticDigest("cortex", entry)
	if err != nil {
		t.Fatalf("SemanticDigest(observed): %v", err)
	}
	presetDigest, err := SemanticDigest("cortex", cortexEntry(t))
	if err != nil {
		t.Fatalf("SemanticDigest(preset): %v", err)
	}
	if observedDigest == presetDigest {
		t.Fatal("the env-name-divergent entry must digest differently from the preset")
	}
	if result.Ownership.Digest != observedDigest {
		t.Fatalf("ownership digest %q is not the observed digest %q", result.Ownership.Digest, observedDigest)
	}
	if !reflect.DeepEqual(result.ObservedIdentity.EnvNames, []string{"CORTEX_USER_TOKEN"}) {
		t.Fatalf("observed identity env names = %v", result.ObservedIdentity.EnvNames)
	}
}
