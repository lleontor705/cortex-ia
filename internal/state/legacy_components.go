package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// LegacySource bounds the metadata locations in which retired identifiers may
// still be decoded. Runtime/current configuration is deliberately absent.
type LegacySource string

const (
	LegacyState     LegacySource = "state"
	LegacyLock      LegacySource = "lock"
	LegacyReceipt   LegacySource = "receipt"
	LegacyManifest  LegacySource = "manifest"
	LegacyBackup    LegacySource = "backup"
	LegacyUninstall LegacySource = "uninstall"
)

var boundedLegacySources = map[LegacySource]struct{}{
	LegacyState: {}, LegacyLock: {}, LegacyReceipt: {}, LegacyManifest: {},
	LegacyBackup: {}, LegacyUninstall: {},
}

// LegacyComponentInventory separates historical component metadata from exact
// managed registrations. A component tombstone alone never implies deletion.
type LegacyComponentInventory struct {
	Source               LegacySource
	SchemaVersion        string
	Components           []model.ComponentID
	ManagedRegistrations []model.ComponentID
}

func (i LegacyComponentInventory) HasRetired(id model.ComponentID) bool {
	if _, ok := model.RetiredComponent(id); !ok {
		return false
	}
	return containsComponent(i.Components, id)
}

func (i LegacyComponentInventory) HasManagedRegistration(id model.ComponentID) bool {
	return containsComponent(i.ManagedRegistrations, id)
}

func containsComponent(components []model.ComponentID, id model.ComponentID) bool {
	for _, component := range components {
		if component == id {
			return true
		}
	}
	return false
}

// DecodeLegacyComponentInventory decodes retired component metadata only from
// explicitly bounded compatibility artifacts. Missing schema_version denotes
// the pre-schema legacy format; declared schemas must use supported major 1.
func DecodeLegacyComponentInventory(source LegacySource, data []byte) (LegacyComponentInventory, error) {
	if _, ok := boundedLegacySources[source]; !ok {
		return LegacyComponentInventory{}, fmt.Errorf("legacy component source %q is not a bounded decode location", source)
	}

	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil {
		return LegacyComponentInventory{}, fmt.Errorf("decode legacy %s metadata: %w", source, err)
	}
	for field := range fields {
		switch field {
		case "schema_version", "components", "managed_registrations":
		default:
			if !strings.HasPrefix(field, "x-") {
				return LegacyComponentInventory{}, fmt.Errorf("legacy %s metadata: unknown field %q; remove it or namespace it with x-", source, field)
			}
		}
	}

	inventory := LegacyComponentInventory{Source: source}
	if raw, ok := fields["schema_version"]; ok {
		if err := json.Unmarshal(raw, &inventory.SchemaVersion); err != nil {
			return LegacyComponentInventory{}, fmt.Errorf("legacy %s metadata schema_version: %w", source, err)
		}
		if err := validateLegacySchemaVersion(inventory.SchemaVersion); err != nil {
			return LegacyComponentInventory{}, fmt.Errorf("legacy %s metadata schema_version: %w", source, err)
		}
	}
	if raw, ok := fields["components"]; ok {
		if err := json.Unmarshal(raw, &inventory.Components); err != nil {
			return LegacyComponentInventory{}, fmt.Errorf("legacy %s metadata components: %w", source, err)
		}
	}
	if raw, ok := fields["managed_registrations"]; ok {
		if err := json.Unmarshal(raw, &inventory.ManagedRegistrations); err != nil {
			return LegacyComponentInventory{}, fmt.Errorf("legacy %s metadata managed_registrations: %w", source, err)
		}
	}
	return inventory, nil
}

func validateLegacySchemaVersion(version string) error {
	majorText, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(majorText)
	if err != nil || major < 1 {
		return fmt.Errorf("invalid semantic version %q", version)
	}
	if major != 1 {
		return fmt.Errorf("major %d is not supported; supported interval is >=1.0.0 <2.0.0", major)
	}
	return nil
}
