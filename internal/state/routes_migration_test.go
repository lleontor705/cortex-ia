package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfilesMigratesMappedLegacyAssignmentsAndBacksUpBytes(t *testing.T) {
	home := t.TempDir()
	legacy := `[{"name":"portable","model_assignments":{"sdd-apply":"route/v1/implementation"}}]`
	if err := os.MkdirAll(filepath.Dir(ProfilesPath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProfilesPath(home), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadProfiles(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Routes["sdd-apply"].RouteID.String() != "route/v1/implementation" {
		t.Fatalf("legacy route was not migrated: %#v", profiles)
	}
	backup, err := os.ReadFile(ProfilesBackupPath(home))
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if string(backup) != legacy {
		t.Fatalf("historical bytes changed: %q", backup)
	}
}

func TestLoadProfilesRejectsUnmappedLegacyAlias(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(ProfilesPath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProfilesPath(home), []byte(`[{"name":"x","model_assignments":{"sdd-apply":"legacy-tier"}}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProfiles(home)
	if err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("expected fail-closed unmapped error, got %v", err)
	}
}
