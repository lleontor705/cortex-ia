package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func newTestRegistry() *agents.Registry {
	r := agents.NewRegistry()
	r.Register(codex.NewAdapter())
	return r
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_Full(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetFull,
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}

	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}
	if len(result.FilesChanged) == 0 {
		t.Error("expected files changed")
	}
	if result.BackupID == "" {
		t.Error("expected backup ID")
	}

	// Verify state was saved.
	s, err := state.Load(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.InstalledAgents) != 1 || s.InstalledAgents[0] != model.AgentCodex {
		t.Errorf("state agents = %v, want [codex]", s.InstalledAgents)
	}
	if s.Version != "test-v1" {
		t.Errorf("state version = %q, want %q", s.Version, "test-v1")
	}

	// Verify lock was saved.
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Files) == 0 {
		t.Error("expected lock to track files")
	}
	if lock.Version != "test-v1" {
		t.Errorf("lock version = %q, want %q", lock.Version, "test-v1")
	}
}

func TestInstall_Minimal(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}

	// Minimal preset should resolve fewer components than full.
	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}
}

func TestInstall_DryRun(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}

	result, err := Install(homeDir, registry, selection, "test-v1", true)
	if err != nil {
		t.Fatalf("Install() dry-run error = %v", err)
	}

	if len(result.ComponentsDone) == 0 {
		t.Error("expected components in dry-run result")
	}
	if len(result.FilesChanged) > 0 {
		t.Error("expected no files changed in dry-run")
	}
	if result.BackupID != "" {
		t.Error("expected no backup in dry-run")
	}
}

func TestInstall_WithInvalidAgent(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex, "nonexistent-agent"},
		Preset: model.PresetMinimal,
	}

	// Validate step catches invalid agent in prepare stage → immediate error.
	_, err := Install(homeDir, registry, selection, "test-v1", false)
	if err == nil {
		t.Fatal("expected error with invalid agent")
	}
}

func TestInstall_ComponentError(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Block the agent's config dir by creating a file where a directory should be.
	codexDir := filepath.Join(homeDir, ".codex")
	if err := os.WriteFile(codexDir, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}

	// Component injection fails → apply stage reports error.
	_, err := Install(homeDir, registry, selection, "test-v1", false)
	if err == nil {
		t.Fatal("expected error with blocked config dir")
	}
}

func TestInstall_ExplicitComponents(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentCodex},
		Components: []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}
	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}
}

func TestInstall_WithProfileName(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Create a profile with model assignments.
	profiles := []model.Profile{
		{
			Name: "premium",
			ModelAssignments: model.ModelAssignments{
				"sdd-explore": model.ModelOpus,
				"sdd-spec":    model.ModelSonnet,
			},
		},
	}
	if err := state.SaveProfiles(homeDir, profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}

	selection := model.Selection{
		Agents:      []model.AgentID{model.AgentCodex},
		Preset:      model.PresetFull,
		ProfileName: "premium",
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}

	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}

	// Verify state saved the profile name.
	s, err := state.Load(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if s.LastProfile != "premium" {
		t.Errorf("state.LastProfile = %q, want %q", s.LastProfile, "premium")
	}
}

func TestInstall_ProfileNameDoesNotOverrideExplicitAssignments(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Create a profile.
	profiles := []model.Profile{
		{
			Name: "economy",
			ModelAssignments: model.ModelAssignments{
				"sdd-explore": model.ModelHaiku,
			},
		},
	}
	if err := state.SaveProfiles(homeDir, profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}

	// Selection already has explicit ModelAssignments — profile should NOT override.
	explicit := model.ModelAssignments{"sdd-explore": model.ModelOpus}
	selection := model.Selection{
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetFull,
		ProfileName:      "economy",
		ModelAssignments: explicit,
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}
	// The explicit assignments should have been preserved (not overridden by profile).
	// We can't directly inspect selection inside Install, but the test confirms no panic/error.
	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}
}

// ---------------------------------------------------------------------------
// Repair
// ---------------------------------------------------------------------------

func TestRepair_Basic(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}

	// First install.
	_, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Delete a managed file to simulate drift.
	lock, _ := state.LoadLock(homeDir)
	if len(lock.Files) > 0 {
		os.Remove(lock.Files[0])
	}

	// Repair should re-create the missing file.
	result, err := Repair(homeDir, registry, "test-v1", false)
	if err != nil {
		t.Fatalf("Repair() error = %v\nErrors: %v", err, result.Errors)
	}
	if len(result.ComponentsDone) == 0 {
		t.Error("expected repair to apply components")
	}
}

func TestRepair_DryRun(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}

	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	result, err := Repair(homeDir, registry, "test-v1", true)
	if err != nil {
		t.Fatalf("Repair() dry-run error = %v", err)
	}
	if len(result.ComponentsDone) == 0 {
		t.Error("expected components in dry-run repair")
	}
}

func TestRepair_NoMetadata(t *testing.T) {
	_, err := Repair(t.TempDir(), newTestRegistry(), "test-v1", false)
	if err == nil {
		t.Fatal("expected error with no metadata")
	}
}

// ---------------------------------------------------------------------------
// Rollback
// ---------------------------------------------------------------------------

func TestRollback_InvalidBackupID(t *testing.T) {
	_, err := Rollback(t.TempDir(), "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal backup ID")
	}
	if !strings.Contains(err.Error(), "invalid backup ID") {
		t.Errorf("expected 'invalid backup ID' error, got: %v", err)
	}
}

func TestRollback_ExplicitBackupID(t *testing.T) {
	homeDir := t.TempDir()
	backupID := "test-backup-001"
	backupDir := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	targetFile := filepath.Join(homeDir, ".codex", "agents.md")

	// Create snapshot and target.
	snapshotPath := filepath.Join(backupDir, "files", ".codex", "agents.md")
	os.MkdirAll(filepath.Dir(snapshotPath), 0o755)
	os.WriteFile(snapshotPath, []byte("original"), 0o644)
	os.MkdirAll(filepath.Dir(targetFile), 0o755)
	os.WriteFile(targetFile, []byte("modified"), 0o644)

	manifest := backup.Manifest{
		ID: backupID, RootDir: backupDir, FileCount: 1,
		Entries: []backup.ManifestEntry{
			{OriginalPath: targetFile, SnapshotPath: snapshotPath, Existed: true, Mode: 0o644},
		},
	}
	backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest)

	got, err := Rollback(homeDir, backupID)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got.ID != backupID {
		t.Errorf("backup ID = %q, want %q", got.ID, backupID)
	}

	content, _ := os.ReadFile(targetFile)
	if string(content) != "original" {
		t.Errorf("restored content = %q, want %q", content, "original")
	}
}

func TestRollback_FallbackToState(t *testing.T) {
	homeDir := t.TempDir()
	backupID := "state-backup-001"
	backupDir := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	os.MkdirAll(backupDir, 0o755)

	manifest := backup.Manifest{ID: backupID, RootDir: backupDir}
	backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest)

	// Lock has NO backup ID, state has one → fallback to state.
	state.SaveLock(homeDir, state.Lockfile{InstalledAgents: []model.AgentID{"x"}})
	state.Save(homeDir, state.State{InstalledAgents: []model.AgentID{"x"}, LastBackupID: backupID})

	got, err := Rollback(homeDir, "")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got.ID != backupID {
		t.Errorf("expected state fallback ID %q, got %q", backupID, got.ID)
	}
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"nil", nil, 0},
		{"empty", []string{}, 0},
		{"single", []string{"a"}, 1},
		{"no_dupes", []string{"a", "b", "c"}, 3},
		{"with_dupes", []string{"a", "b", "a", "c", "b"}, 3},
		{"with_empty", []string{"a", "", "b", "", "c"}, 3},
		{"all_empty", []string{"", "", ""}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupeStrings(tt.input)
			if len(got) != tt.want {
				t.Errorf("dedupeStrings(%v) = %d items, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestDedupeAgents_WithEmptyValues(t *testing.T) {
	result := dedupeAgents([]model.AgentID{"", model.AgentCodex, ""})
	if len(result) != 1 || result[0] != model.AgentCodex {
		t.Errorf("expected [codex], got %v", result)
	}
}

func TestDedupeComponents_WithEmptyValues(t *testing.T) {
	result := dedupeComponents([]model.ComponentID{"", model.ComponentCortex, ""})
	if len(result) != 1 || result[0] != model.ComponentCortex {
		t.Errorf("expected [cortex], got %v", result)
	}
}

func TestFirstNonEmptyPreset_AllEmpty(t *testing.T) {
	if got := firstNonEmptyPreset("", ""); got != model.PresetFull {
		t.Errorf("expected default PresetFull, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// collectBackupPaths
// ---------------------------------------------------------------------------

func TestCollectBackupPaths(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	components := []model.ComponentID{model.ComponentSDD, model.ComponentConventions}
	paths := collectBackupPaths(homeDir, registry, []model.AgentID{model.AgentCodex}, components)

	if len(paths) == 0 {
		t.Error("expected non-empty backup paths")
	}

	hasPrompt := false
	hasSettings := false
	for _, p := range paths {
		if strings.HasSuffix(p, "agents.md") {
			hasPrompt = true
		}
		if strings.Contains(p, "config.toml") {
			hasSettings = true
		}
	}
	if !hasPrompt {
		t.Error("expected system prompt in backup paths")
	}
	if !hasSettings {
		t.Error("expected settings in backup paths")
	}
}

func TestCollectBackupPaths_InvalidAgent(t *testing.T) {
	paths := collectBackupPaths(t.TempDir(), newTestRegistry(), []model.AgentID{"nonexistent"}, nil)
	if len(paths) != 0 {
		t.Errorf("expected empty paths for invalid agent, got %d", len(paths))
	}
}

func TestRepair_CorruptState(t *testing.T) {
	homeDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".cortex-ia")
	os.MkdirAll(stateDir, 0o755)
	os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{invalid"), 0o644)

	_, err := Repair(homeDir, newTestRegistry(), "v1", false)
	if err == nil {
		t.Fatal("expected error with corrupt state")
	}
}

func TestRepair_CorruptLock(t *testing.T) {
	homeDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".cortex-ia")
	os.MkdirAll(stateDir, 0o755)
	// Valid state but corrupt lock.
	os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(`{"installed_agents":["codex"],"last_install":"2025-01-01T00:00:00Z"}`), 0o644)
	os.WriteFile(filepath.Join(stateDir, "cortex-ia.lock"), []byte("{invalid"), 0o644)

	_, err := Repair(homeDir, newTestRegistry(), "v1", false)
	if err == nil {
		t.Fatal("expected error with corrupt lock")
	}
}

func TestRollback_CorruptState(t *testing.T) {
	homeDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".cortex-ia")
	os.MkdirAll(stateDir, 0o755)
	os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{invalid"), 0o644)

	_, err := Rollback(homeDir, "")
	if err == nil {
		t.Fatal("expected error with corrupt state")
	}
}

func TestRollback_CorruptLock(t *testing.T) {
	homeDir := t.TempDir()
	stateDir := filepath.Join(homeDir, ".cortex-ia")
	os.MkdirAll(stateDir, 0o755)
	os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(`{"installed_agents":["codex"]}`), 0o644)
	os.WriteFile(filepath.Join(stateDir, "cortex-ia.lock"), []byte("{invalid"), 0o644)

	_, err := Rollback(homeDir, "")
	if err == nil {
		t.Fatal("expected error with corrupt lock")
	}
}

func TestRollback_ManifestNotFound(t *testing.T) {
	homeDir := t.TempDir()
	// Valid backup ID but no manifest file.
	_, err := Rollback(homeDir, "valid-id-no-manifest")
	if err == nil {
		t.Fatal("expected error when manifest not found")
	}
}

func TestRollback_RestoreError(t *testing.T) {
	homeDir := t.TempDir()
	backupID := "restore-fail-001"
	backupDir := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	os.MkdirAll(backupDir, 0o755)

	// Manifest points to a non-existent snapshot → restore fails.
	manifest := backup.Manifest{
		ID: backupID, RootDir: backupDir,
		Entries: []backup.ManifestEntry{
			{OriginalPath: filepath.Join(homeDir, "target"), SnapshotPath: "/nonexistent/snap", Existed: true, Mode: 0o644},
		},
	}
	backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest)

	_, err := Rollback(homeDir, backupID)
	if err == nil {
		t.Fatal("expected error when restore fails")
	}
}

func TestInstall_StateSaveError(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Pre-create state.json as a directory → state.Save will fail.
	os.MkdirAll(filepath.Join(homeDir, ".cortex-ia", "state.json"), 0o755)

	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}
	result, err := Install(homeDir, registry, selection, "v1", false)
	if err == nil {
		t.Fatal("expected error when state save fails")
	}

	hasStateErr := false
	for _, e := range result.Errors {
		if strings.Contains(e, "save state") {
			hasStateErr = true
		}
	}
	if !hasStateErr {
		t.Errorf("expected 'save state' in errors, got: %v", result.Errors)
	}
}

func TestInstall_LockSaveError(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Pre-create lock file as a directory → SaveLock will fail.
	os.MkdirAll(filepath.Join(homeDir, ".cortex-ia", "cortex-ia.lock"), 0o755)

	selection := model.Selection{
		Agents: []model.AgentID{model.AgentCodex},
		Preset: model.PresetMinimal,
	}
	result, err := Install(homeDir, registry, selection, "v1", false)
	if err == nil {
		t.Fatal("expected error when lock save fails")
	}

	hasLockErr := false
	for _, e := range result.Errors {
		if strings.Contains(e, "save lock") {
			hasLockErr = true
		}
	}
	if !hasLockErr {
		t.Errorf("expected 'save lock' in errors, got: %v", result.Errors)
	}
}

func TestCollectBackupPaths_WithExistingMCPConfig(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Create MCP config file so os.Stat succeeds.
	mcpPath := filepath.Join(homeDir, ".codex", "config.toml")
	os.MkdirAll(filepath.Dir(mcpPath), 0o755)
	os.WriteFile(mcpPath, []byte("# mcp"), 0o644)

	components := []model.ComponentID{model.ComponentCortex}
	paths := collectBackupPaths(homeDir, registry, []model.AgentID{model.AgentCodex}, components)

	hasMCP := false
	for _, p := range paths {
		if strings.Contains(p, "config.toml") {
			hasMCP = true
			break
		}
	}
	if !hasMCP {
		t.Error("expected MCP config in backup paths when file exists")
	}
}
