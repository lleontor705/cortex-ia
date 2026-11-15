package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeStaleProvidersState materializes a v2 state.json that still carries the
// retired "providers" family, emulating a document written before the passive
// retirement. It returns the raw bytes so callers can compare the pre- and
// post-commit documents.
func writeStaleProvidersState(t *testing.T, home string) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(StatePath(home)), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	doc := map[string]any{
		"schema_version": MetadataSchemaV2,
		"opencode_root":  filepath.Clean(home),
		"selection":      map[string]any{"cortex": true},
		"artifacts": []map[string]any{{
			"path":      ".config/opencode/plugins/cortex-work.ts",
			"kind":      "other",
			"origin":    "embedded",
			"digest":    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"ownership": "managed",
		}},
		"mcps":           []any{},
		"agent_models":   []any{},
		"transaction_id": "txn-retire-1",
		"updated_at":     time.Now().UTC(),
		"providers": []map[string]any{{
			"id":   "nan",
			"kind": "openai-compatible",
		}},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal stale state: %v", err)
	}
	if err := os.WriteFile(StatePath(home), raw, 0o644); err != nil {
		t.Fatalf("write stale state: %v", err)
	}
	return raw
}

// TestREQREM003StaleProvidersStateStillLoads proves the tolerant-load half of
// the passive-retirement contract: a v2 document written while the Providers
// family still existed keeps classifying as PresenceV2 and validating, because
// json.Unmarshal tolerates the unknown stale key.
func TestREQREM003StaleProvidersStateStillLoads(t *testing.T) {
	home := t.TempDir()
	raw := writeStaleProvidersState(t, home)
	if !json.Valid(raw) {
		t.Fatalf("stale fixture is not valid JSON")
	}

	load := LoadMetadataV2(home)
	if load.Presence != PresenceV2 {
		t.Fatalf("stale providers state must classify as PresenceV2, got %v (detail: %s)",
			load.Presence, load.Detail)
	}
	if err := ValidateV2(load.Metadata); err != nil {
		t.Fatalf("loaded stale state must validate, got %v", err)
	}
	if load.Metadata.TransactionID != "txn-retire-1" {
		t.Errorf("transaction_id not preserved: got %q", load.Metadata.TransactionID)
	}
	if len(load.Metadata.Artifacts) != 1 {
		t.Errorf("artifacts not preserved: got %d", len(load.Metadata.Artifacts))
	}
}

// TestREQREM003CommitDropsStaleProvidersKey proves the re-marshal half: a new
// commit rewrites the document from the Go schema, which no longer represents
// providers, so the stale key disappears while every surviving family remains.
// Reintroducing a persisted providers field would re-emit the key here and
// fail this oracle.
func TestREQREM003CommitDropsStaleProvidersKey(t *testing.T) {
	home := t.TempDir()
	writeStaleProvidersState(t, home)

	load := LoadMetadataV2(home)
	if load.Presence != PresenceV2 {
		t.Fatalf("precondition: stale state must load as v2, got %v (detail: %s)",
			load.Presence, load.Detail)
	}
	if err := SaveMetadataV2(home, load.Metadata); err != nil {
		t.Fatalf("commit state: %v", err)
	}

	raw, err := os.ReadFile(StatePath(home))
	if err != nil {
		t.Fatalf("read committed state: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("committed state is not valid JSON: %v", err)
	}
	if _, ok := fields["providers"]; ok {
		t.Errorf("committed state still carries the retired providers key: %s", raw)
	}
	for _, key := range []string{
		"schema_version", "opencode_root", "selection", "artifacts",
		"mcps", "agent_models", "transaction_id", "updated_at",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("committed state dropped surviving family %q", key)
		}
	}

	reloaded := LoadMetadataV2(home)
	if reloaded.Presence != PresenceV2 {
		t.Fatalf("re-marshalled state must still validate, got %v (detail: %s)",
			reloaded.Presence, reloaded.Detail)
	}
	if reloaded.Metadata.TransactionID != "txn-retire-1" || len(reloaded.Metadata.Artifacts) != 1 {
		t.Errorf("surviving families altered: txn=%q artifacts=%d",
			reloaded.Metadata.TransactionID, len(reloaded.Metadata.Artifacts))
	}
}

// TestREQREM003UnknownSchemaVersionFailsClosed proves fail-closed versioning:
// the schema stays pinned at 2, so documents declaring any other version are
// rejected as malformed rather than degraded to legacy or accepted.
func TestREQREM003UnknownSchemaVersionFailsClosed(t *testing.T) {
	for _, version := range []int{1, 3} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Dir(StatePath(home)), 0o755); err != nil {
			t.Fatalf("create state dir: %v", err)
		}
		raw := []byte(`{"schema_version":` + itoa(version) +
			`,"opencode_root":"/tmp/x","transaction_id":"txn","updated_at":"2026-01-01T00:00:00Z"}`)
		if err := os.WriteFile(StatePath(home), raw, 0o644); err != nil {
			t.Fatalf("write schema_version %d state: %v", version, err)
		}
		if load := LoadMetadataV2(home); load.Presence != PresenceMalformed {
			t.Errorf("schema_version %d must fail closed as malformed, got %v", version, load.Presence)
		}
		got, known, detail := probeSchemaVersion(raw)
		if !known || got != version || detail == "" {
			t.Errorf("probeSchemaVersion(%d): known=%v detail=%q; expected a rejected known version",
				version, known, detail)
		}
	}
}

// TestREQREM003SchemaVersionPinnedAtTwo is the constant guard: passive
// retirement requires no v3, so the schema version must remain 2.
func TestREQREM003SchemaVersionPinnedAtTwo(t *testing.T) {
	if MetadataSchemaV2 != 2 {
		t.Fatalf("MetadataSchemaV2 must stay pinned at 2 for passive retirement, got %d", MetadataSchemaV2)
	}
}

// itoa renders a small non-negative integer without importing strconv into the
// fixture path.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
