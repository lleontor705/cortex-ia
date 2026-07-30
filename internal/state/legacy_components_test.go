package state

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestDecodeLegacyComponentInventory(t *testing.T) {
	for _, source := range []LegacySource{
		LegacyState,
		LegacyLock,
		LegacyReceipt,
		LegacyManifest,
		LegacyBackup,
		LegacyUninstall,
	} {
		t.Run(string(source), func(t *testing.T) {
			inventory, err := DecodeLegacyComponentInventory(source, []byte(`{
				"schema_version":"1.0.0",
				"components":["cortex","agent-mailbox"],
				"managed_registrations":["agent-mailbox"]
			}`))
			if err != nil {
				t.Fatal(err)
			}
			if !inventory.HasRetired(model.ComponentMailbox) {
				t.Fatal("legacy Mailbox metadata was not classified as retired")
			}
			if !inventory.HasManagedRegistration(model.ComponentMailbox) {
				t.Fatal("exact managed registration was not inventoried")
			}
		})
	}
}

func TestDecodeLegacyMetadataOnlyDoesNotInventRegistration(t *testing.T) {
	inventory, err := DecodeLegacyComponentInventory(LegacyState, []byte(`{
		"components":["agent-mailbox"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.HasRetired(model.ComponentMailbox) {
		t.Fatal("legacy component was not decoded")
	}
	if inventory.HasManagedRegistration(model.ComponentMailbox) {
		t.Fatal("metadata-only legacy state invented a managed registration")
	}
}

func TestDecodeLegacyInventoryRejectsUnsupportedSchemaMajor(t *testing.T) {
	_, err := DecodeLegacyComponentInventory(LegacyReceipt, []byte(`{
		"schema_version":"2.0.0",
		"components":["agent-mailbox"]
	}`))
	if err == nil || !strings.Contains(err.Error(), "schema_version") || !strings.Contains(err.Error(), "supported") {
		t.Fatalf("expected actionable schema error, got %v", err)
	}
}

func TestDecodeLegacyInventoryRejectsUnknownUnnamespacedField(t *testing.T) {
	_, err := DecodeLegacyComponentInventory(LegacyManifest, []byte(`{
		"schema_version":"1.0.0",
		"components":["agent-mailbox"],
		"surprise":true
	}`))
	if err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("expected unknown field error with path, got %v", err)
	}
}

func TestDecodeLegacyInventoryRejectsUnknownSource(t *testing.T) {
	_, err := DecodeLegacyComponentInventory(LegacySource("runtime"), []byte(`{"components":[]}`))
	if err == nil {
		t.Fatal("expected unbounded runtime source to be rejected")
	}
}
