package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMetadataV2Lifecycle(t *testing.T) {
	tempHome := t.TempDir()
	cleanHome := filepath.Clean(tempHome)

	// 1. Initial load on empty home -> PresenceAbsent
	metaLoad := LoadMetadataV2(tempHome)
	if metaLoad.Presence != PresenceAbsent {
		t.Fatalf("expected PresenceAbsent, got %v", metaLoad.Presence)
	}

	// 2. Save Metadata V2
	now := time.Now().UTC()
	doc := MetadataV2{
		SchemaVersion: MetadataSchemaV2,
		OpencodeRoot:  cleanHome,
		TransactionID: "txn-123",
		Selection: SelectionV2{
			Cortex:   true,
			Context7: false,
		},
		UpdatedAt: now,
		Artifacts: []ArtifactV2{
			{
				Path:      ".config/opencode/plugins/herdr-bridge.ts",
				Kind:      KindOther,
				Origin:    "embedded",
				Ownership: OwnershipManaged,
				Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
	}

	if err := SaveMetadataV2(tempHome, doc); err != nil {
		t.Fatalf("SaveMetadataV2 failed: %v", err)
	}

	// 3. Re-load Metadata V2 -> PresenceV2
	reloaded := LoadMetadataV2(tempHome)
	if reloaded.Presence != PresenceV2 {
		t.Fatalf("expected PresenceV2, got %v (detail: %s)", reloaded.Presence, reloaded.Detail)
	}
	if reloaded.Metadata.TransactionID != "txn-123" {
		t.Errorf("expected txn-123, got %s", reloaded.Metadata.TransactionID)
	}
	if len(reloaded.Metadata.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(reloaded.Metadata.Artifacts))
	}
}

func TestLockV2Lifecycle(t *testing.T) {
	tempHome := t.TempDir()
	cleanHome := filepath.Clean(tempHome)

	// 1. Initial lock load -> PresenceAbsent
	lockLoad := LoadLockV2(tempHome)
	if lockLoad.Presence != PresenceAbsent {
		t.Fatalf("expected PresenceAbsent, got %v", lockLoad.Presence)
	}

	// 2. Save Lock V2
	lockDoc := LockV2{
		SchemaVersion: MetadataSchemaV2,
		OpencodeRoot:  cleanHome,
		TransactionID: "txn-123",
		GeneratedAt:   time.Now().UTC(),
		Artifacts: []ArtifactV2{
			{
				Path:      ".config/opencode/plugins/herdr-bridge.ts",
				Kind:      KindOther,
				Origin:    "embedded",
				Ownership: OwnershipManaged,
				Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
	}
	if err := SaveLockV2(tempHome, lockDoc); err != nil {
		t.Fatalf("SaveLockV2 failed: %v", err)
	}

	// 3. Re-load Lock V2 -> PresenceV2
	reloaded := LoadLockV2(tempHome)
	if reloaded.Presence != PresenceV2 {
		t.Fatalf("expected PresenceV2, got %v (detail: %s)", reloaded.Presence, reloaded.Detail)
	}
	if reloaded.Lock.TransactionID != "txn-123" {
		t.Errorf("expected txn-123, got %s", reloaded.Lock.TransactionID)
	}
}
