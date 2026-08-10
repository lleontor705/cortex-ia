package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/uninstall"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func newTestRegistry() *agents.Registry {
	r := agents.NewRegistry()
	r.Register(opencode.NewAdapter())
	return r
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_Full(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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
	if len(s.InstalledAgents) != 1 || s.InstalledAgents[0] != model.AgentOpenCode {
		t.Errorf("state agents = %v, want [opencode]", s.InstalledAgents)
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
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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

func TestInstallDryRunWithSupportedComponentsDoesNotMutate(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	sentinel := filepath.Join(homeDir, ".codex", "operator-sentinel.txt")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("operator content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := testTreeDigest(t, homeDir)
	result, err := Install(homeDir, registry, model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}, "test-v1", true)
	if err != nil {
		t.Fatalf("Install() dry-run error = %v", err)
	}
	after := testTreeDigest(t, homeDir)
	if before != after {
		t.Fatalf("dry-run mutated target tree: before=%s after=%s", before, after)
	}

	if result.WorkflowFingerprint != "" || result.WorkflowReceipt.ID != "" || result.WorkflowRollback {
		t.Fatalf("dry-run produced mutation evidence: receipt=%+v rollback=%t", result.WorkflowReceipt, result.WorkflowRollback)
	}
}

func TestPreparedWorkflowCreateUsesStrictAbsentTargetCAS(t *testing.T) {
	homeDir := t.TempDir()
	adapter, err := newTestRegistry().Get(model.AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{HomeDir: homeDir, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "test-v1", ModelRoutes: testModelRoutes()})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Plan.Creates) == 0 {
		t.Fatal("prepared plan has no create effect")
	}
	target := filepath.Join(homeDir, filepath.FromSlash(prepared.Plan.Creates[0].Path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("concurrent writer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Apply(); !errors.Is(err, sddinstall.ErrStalePlan) {
		t.Fatalf("Apply() error = %v, want strict absent-target stale-plan CAS", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "concurrent writer\n" {
		t.Fatalf("stale create overwrote concurrent target: %q", got)
	}
}

func TestPrepareWorkflowPreservesCurrentMailboxRegistration(t *testing.T) {
	homeDir := t.TempDir()
	adapter, err := newTestRegistry().Get(model.AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	target := adapter.SettingsPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"mcp":{"agent-mailbox":{"command":"agent-mailbox-mcp"},"operator-tool":{"command":"keep"}}}`)
	if err := os.WriteFile(target, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveLock(homeDir, state.Lockfile{
		Components: []model.ComponentID{model.ComponentMailbox}, Files: []string{target}, Version: "legacy-v1",
	}); err != nil {
		t.Fatal(err)
	}

	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{HomeDir: homeDir, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "test-v1", ModelRoutes: testModelRoutes()})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Retirements) != 0 {
		t.Fatalf("prepared workflow unexpectedly planned Mailbox retirement: %+v", prepared.Retirements)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(legacy) {
		t.Fatalf("Mailbox registration changed during preparation: %s", got)
	}
	if prepared.Fingerprint != prepared.Plan.Fingerprint {
		t.Fatalf("prepared fingerprint %q differs from immutable plan %q", prepared.Fingerprint, prepared.Plan.Fingerprint)
	}
}

func TestPrepareWorkflowBuildsOneDeterministicHomeRelativePlan(t *testing.T) {
	homeDir := t.TempDir()
	adapter, err := newTestRegistry().Get(model.AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	request := WorkflowRequest{HomeDir: homeDir, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "test-v1", ModelRoutes: testModelRoutes()}

	first, err := PrepareWorkflow(context.Background(), request)
	if err != nil {
		t.Fatalf("PrepareWorkflow() error = %v", err)
	}
	second, err := PrepareWorkflow(context.Background(), request)
	if err != nil {
		t.Fatalf("PrepareWorkflow() second error = %v", err)
	}
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprints = %q and %q", first.Fingerprint, second.Fingerprint)
	}
	if !reflect.DeepEqual(first.Plan, second.Plan) {
		t.Fatal("identical preparation produced different plans")
	}
	if len(first.Plan.Creates) == 0 {
		t.Fatal("prepared workflow plan has no generated assets")
	}
	for _, effect := range first.Plan.Creates {
		path := filepath.ToSlash(effect.Path)
		if !strings.HasPrefix(path, ".config/opencode/") && !strings.HasPrefix(path, ".cortex-ia/") {
			t.Fatalf("effect path %q is not rebased under adapter config root", effect.Path)
		}
	}
	receipt, err := first.Apply()
	if err != nil {
		t.Fatalf("prepared Apply() error = %v", err)
	}
	wantApplied := make([]string, len(first.Plan.Creates))
	for index, effect := range first.Plan.Creates {
		wantApplied[index] = effect.Path
	}
	sort.Strings(wantApplied)
	gotApplied := append([]string(nil), receipt.Applied...)
	sort.Strings(gotApplied)
	if !reflect.DeepEqual(gotApplied, wantApplied) {
		t.Fatalf("Apply() paths = %v, want exact prepared creates %v", gotApplied, wantApplied)
	}
}

func TestInstallDryRunWithoutWorkflowDoesNotMutate(t *testing.T) {
	homeDir := t.TempDir()
	before := testTreeDigest(t, homeDir)
	result, err := Install(homeDir, newTestRegistry(), model.Selection{
		Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentCortex},
	}, "test-v1", true)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.WorkflowFingerprint != "" {
		t.Fatalf("non-workflow install reported a workflow fingerprint: %q", result.WorkflowFingerprint)
	}
	if got := testTreeDigest(t, homeDir); got != before {
		t.Fatalf("dry-run mutated target: before=%s after=%s", before, got)
	}
}

func testTreeDigest(t *testing.T, root string) string {
	t.Helper()
	var records []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		records = append(records, fmt.Sprintf("%s:%x", filepath.ToSlash(relative), sha256.Sum256(content)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(records)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(records, "\n"))))
}

func TestInstall_WithInvalidAgent(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode, "nonexistent-agent"},
		Components: []model.ComponentID{model.ComponentCortex},
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
	configDir := filepath.Join(homeDir, ".config")
	if err := os.WriteFile(configDir, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec},
	}

	result, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}
	if len(result.ComponentsDone) == 0 {
		t.Error("expected components done")
	}
}

func TestInstall_RetiredSelectionsFailClosedWithoutMutation(t *testing.T) {
	tests := []struct {
		name          string
		selection     model.Selection
		wantErrorPart string
	}{
		{
			name: "profile name",
			selection: model.Selection{
				Agents: []model.AgentID{model.AgentOpenCode}, Preset: model.PresetFull, ProfileName: "retired-profile",
			},
			wantErrorPart: `retired selection field "profile"`,
		},
		{
			name: "model assignments",
			selection: model.Selection{
				Agents: []model.AgentID{model.AgentOpenCode}, Preset: model.PresetFull, ModelAssignments: model.ModelAssignments{"implement": "provider-test/model-test"},
			},
			wantErrorPart: `retired selection field "model-assignment"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			before := testTreeDigest(t, homeDir)

			result, err := Install(homeDir, newTestRegistry(), tt.selection, "test-v1", false)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrorPart) {
				t.Fatalf("Install() error = %v, want %q", err, tt.wantErrorPart)
			}
			if got := testTreeDigest(t, homeDir); got != before {
				t.Fatalf("retired selection mutated target tree: before=%s after=%s", before, got)
			}
			if result.BackupID != "" || len(result.FilesChanged) != 0 || len(result.ComponentsDone) != 0 || len(result.Errors) != 0 || result.WorkflowReceipt.ID != "" {
				t.Fatalf("retired selection produced install evidence: %+v", result)
			}
		})
	}
}

func TestInstallCanonicalClientsAreExactAndGeminiFailsClosed(t *testing.T) {
	registry := agents.NewDefaultRegistry()
	wantClients := []model.AgentID{
		model.AgentOpenCode,
	}
	if got := registry.IDs(); !reflect.DeepEqual(got, wantClients) {
		t.Fatalf("default registry clients = %v, want exactly %v", got, wantClients)
	}

	homeDir := t.TempDir()
	before := testTreeDigest(t, homeDir)
	result, err := Install(homeDir, registry, model.Selection{
		Agents:     []model.AgentID{"gemini"},
		Components: []model.ComponentID{model.ComponentCortex},
	}, "test-v1", false)
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("Install() error = %v, want Gemini rejection", err)
	}
	if got := testTreeDigest(t, homeDir); got != before {
		t.Fatalf("Gemini rejection mutated target tree: before=%s after=%s", before, got)
	}
	if result.BackupID != "" || len(result.FilesChanged) != 0 || len(result.Errors) != 0 {
		t.Fatalf("Gemini rejection produced install evidence: %+v", result)
	}
}

func TestInstallIdempotentCortexOnlyDoesNotCreateModelOrPackageFiles(t *testing.T) {
	homeDir := t.TempDir()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	registry := newTestRegistry()

	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	firstConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(firstConfig), "model") || strings.Contains(string(firstConfig), "package") {
		t.Fatalf("Cortex-only install wrote retired model/package configuration:\n%s", firstConfig)
	}

	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	secondConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(secondConfig) != string(firstConfig) {
		t.Fatalf("repeat install changed Cortex config:\nfirst=%s\nsecond=%s", firstConfig, secondConfig)
	}
	for _, forbidden := range []string{
		filepath.Join(homeDir, ".cortex-ia", "models.json"),
		filepath.Join(homeDir, ".cortex-ia", "packages.json"),
	} {
		if _, err := os.Stat(forbidden); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Cortex-only install created forbidden model/package artifact %q: %v", forbidden, err)
		}
	}
}

func TestInstallNoOpReinstallPreservesWorkflowLockInventory(t *testing.T) {
	homeDir := t.TempDir()
	foreignPath := filepath.Join(homeDir, "operator-owned.txt")
	if err := os.WriteFile(foreignPath, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentSDD},
	}
	registry := newTestRegistry()

	first, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("first Install() error = %v\nErrors: %v", err, first.Errors)
	}
	firstLock, err := state.LoadLock(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstLock.Files) == 0 {
		t.Fatal("first install produced an empty workflow lock inventory")
	}

	second, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("second Install() error = %v\nErrors: %v", err, second.Errors)
	}
	for _, asset := range second.WorkflowPlan.Inventory {
		for _, reported := range second.FilesChanged {
			if reported == asset.Path {
				t.Fatalf("no-op workflow asset %q leaked into current FilesChanged reports %v", asset.Path, second.FilesChanged)
			}
		}
	}
	secondLock, err := state.LoadLock(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(secondLock.Files, firstLock.Files) {
		t.Fatalf("no-op reinstall lock files = %v, want preserved inventory %v", secondLock.Files, firstLock.Files)
	}
	for _, tracked := range secondLock.Files {
		if tracked == foreignPath {
			t.Fatalf("lock claimed unrelated on-disk file %q", foreignPath)
		}
	}
}

func TestOpenCodeLegacyUpgradeReinstallAndUninstallLifecycle(t *testing.T) {
	homeDir := t.TempDir()
	legacyPath := ".config/opencode/generic/root/contracts.md"
	legacyContent := []byte("legacy internal\n")
	writeLegacyWorkflowOwnership(t, homeDir, legacyPath, legacyContent)
	foreign := []string{"package.json", "package-lock.json", ".gitignore", "node_modules/operator.txt"}
	for _, relative := range foreign {
		fullPath := filepath.Join(homeDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("foreign\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.Save(homeDir, state.State{InstalledAgents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentSDD}}); err != nil {
		t.Fatal(err)
	}
	locked := []string{filepath.Join(homeDir, filepath.FromSlash(legacyPath))}
	for _, relative := range foreign {
		locked = append(locked, filepath.Join(homeDir, filepath.FromSlash(relative)))
	}
	if err := state.SaveLock(homeDir, state.Lockfile{InstalledAgents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentSDD}, Files: locked}); err != nil {
		t.Fatal(err)
	}

	selection := model.Selection{Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentSDD}}
	registry := newTestRegistry()
	first, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("legacy upgrade: %v; errors: %v", err, first.Errors)
	}
	for _, relative := range []string{legacyPath, legacyPath + ".cortex-ia.json", legacyPath + ".cortex-ia.base"} {
		if _, err := os.Stat(filepath.Join(homeDir, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Fatalf("legacy path %q survived native-only upgrade: %v", relative, err)
		}
	}
	second, err := Install(homeDir, registry, selection, "test-v1", false)
	if err != nil {
		t.Fatalf("native-only reinstall: %v; errors: %v", err, second.Errors)
	}
	if second.WorkflowPlan.HasBlockingConflicts() {
		t.Fatalf("native-only reinstall conflicts: %+v", second.WorkflowPlan.Conflicts)
	}
	if _, err := uninstall.NewServiceWithRegistry(homeDir, registry).Apply(uninstall.Selection{Agents: selection.Agents, Components: selection.Components}); err != nil {
		t.Fatalf("uninstall native-only workflow: %v", err)
	}
	for _, relative := range foreign {
		content, err := os.ReadFile(filepath.Join(homeDir, filepath.FromSlash(relative)))
		if err != nil || string(content) != "foreign\n" {
			t.Fatalf("foreign path %q changed across lifecycle: content=%q err=%v", relative, content, err)
		}
	}
}

func writeLegacyWorkflowOwnership(t *testing.T, homeDir, relative string, content []byte) {
	t.Helper()
	fullPath := filepath.Join(homeDir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	ownership, err := sddinstall.NewOwnership(relative, "1.0.0", "asset/opencode/legacy/contracts", content, content)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath+".cortex-ia.json", append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath+".cortex-ia.base", content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDurableInstallFilesCollapsesKnownSourcesAndAppliesDeletes(t *testing.T) {
	prior := []string{"prior.md", "delete.md", "prior.md"}
	workflow := PreparedWorkflowInstall{Plan: sddinstall.Plan{
		Inventory: []sddinstall.AssetInventory{{Path: "workflow.md"}, {Path: "prior.md"}},
		Deletes:   []sddinstall.Effect{{Path: "delete.md"}},
	}}
	reported := []string{"reported.md", "workflow.md", "reported.md"}

	got := durableInstallFiles(prior, workflow, reported)
	want := []string{"prior.md", "workflow.md", "reported.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("durableInstallFiles() = %v, want collapsed known inventory %v", got, want)
	}
}

func TestInstallPostBackupStatusTargetConflictPreservesConfigAndMetadata(t *testing.T) {
	homeDir := t.TempDir()
	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("# operator configuration\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(homeDir, state.State{Version: "before-state"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveLock(homeDir, state.Lockfile{Version: "before-lock"}); err != nil {
		t.Fatal(err)
	}
	statusPath := state.InstallStatusPath(homeDir)
	if err := os.MkdirAll(statusPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(statusPath, "operator-sentinel"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	protected := map[string]string{}
	for _, path := range []string{configPath, state.StatePath(homeDir), state.LockPath(homeDir), filepath.Join(statusPath, "operator-sentinel")} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		protected[path] = string(content)
	}

	result, err := Install(homeDir, newTestRegistry(), model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}, "test-v1", false)
	if err == nil || !strings.Contains(err.Error(), "install-status.json\" has incompatible existing type") {
		t.Fatalf("Install() error = %v, want install-status target conflict", err)
	}
	if result.BackupID == "" {
		t.Fatal("post-backup target conflict did not retain backup evidence")
	}
	for path, want := range protected {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Fatalf("protected target %q = %q, %v; want %q", path, got, readErr, want)
		}
	}
}

func TestInstallPostBackupFailuresRestoreExactPreimages(t *testing.T) {
	boom := errors.New("injected post-backup failure")
	for _, failure := range []struct {
		name   string
		inject func(installDependencies) installDependencies
	}{
		{
			name: "status start",
			inject: func(deps installDependencies) installDependencies {
				deps.saveInstallStatus = func(home string, status state.InstallStatus) error {
					if err := state.SaveInstallStatus(home, status); err != nil {
						return err
					}
					return boom
				}
				return deps
			},
		},
		{
			name: "component",
			inject: func(deps installDependencies) installDependencies {
				deps.invokeComponent = func(_ model.ComponentID, invoke func() ([]string, error)) ([]string, error) {
					files, err := invoke()
					if err != nil {
						return files, err
					}
					return files, boom
				}
				return deps
			},
		},
		{
			name: "persona",
			inject: func(deps installDependencies) installDependencies {
				deps.invokePersona = func(invoke func() ([]string, error)) ([]string, error) {
					files, err := invoke()
					if err != nil {
						return files, err
					}
					return files, boom
				}
				return deps
			},
		},
		{
			name: "state",
			inject: func(deps installDependencies) installDependencies {
				deps.saveState = func(home string, value state.State) error {
					if err := state.Save(home, value); err != nil {
						return err
					}
					return boom
				}
				return deps
			},
		},
		{
			name: "lock",
			inject: func(deps installDependencies) installDependencies {
				deps.saveLock = func(home string, value state.Lockfile) error {
					if err := state.SaveLock(home, value); err != nil {
						return err
					}
					return boom
				}
				return deps
			},
		},
		{
			name: "status clear",
			inject: func(deps installDependencies) installDependencies {
				deps.clearInstallStatus = func(home string) error {
					if err := state.ClearInstallStatus(home); err != nil {
						return err
					}
					return boom
				}
				return deps
			},
		},
		{
			name: "checkpoint",
			inject: func(deps installDependencies) installDependencies {
				deps.recordJournalOutcome = func(*InstallJournal, MutationOutcome) error { return boom }
				return deps
			},
		},
		{
			name: "commit",
			inject: func(deps installDependencies) installDependencies {
				deps.commitJournal = func(*InstallJournal) error { return boom }
				return deps
			},
		},
	} {
		t.Run(failure.name, func(t *testing.T) {
			homeDir, before := installCoordinatorPreimages(t, "existing")
			selection := model.Selection{Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentCortex}, Persona: model.PersonaProfessional}
			if failure.name == "component" {
				selection.Persona = ""
			}
			result, err := installWithDependencies(homeDir, newTestRegistry(), selection, "test-v1", false, failure.inject(defaultInstallDependencies()))
			if !errors.Is(err, boom) {
				t.Fatalf("Install() error = %v, want injected failure", err)
			}
			if result.BackupID == "" {
				t.Fatal("post-backup failure lost backup evidence")
			}
			assertInstallCoordinatorPreimages(t, before)
		})
	}
}

func TestInstallReceiptFailureRestoresWorkflowPreimage(t *testing.T) {
	homeDir, before := installCoordinatorPreimages(t, "existing")
	workflowPath := filepath.Join(homeDir, "workflow.md")
	if err := os.WriteFile(workflowPath, []byte("workflow before\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before[workflowPath] = captureInstallTestFile(t, workflowPath)
	boom := errors.New("injected workflow apply failure")
	deps := defaultInstallDependencies()
	deps.prepareWorkflow = func(context.Context, WorkflowRequest) (PreparedWorkflowInstall, error) {
		return PreparedWorkflowInstall{Plan: sddinstall.Plan{
			Updates: []sddinstall.Effect{{Path: "workflow.md"}},
			Backup:  sddinstall.BackupScope{Required: true, Paths: []string{"workflow.md"}},
		}}, nil
	}
	deps.applyWorkflow = func(PreparedWorkflowInstall) (sddinstall.Receipt, error) {
		if err := os.WriteFile(workflowPath, []byte("workflow after\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return sddinstall.Receipt{ID: "workflow-receipt", Applied: []string{"workflow.md"}}, boom
	}

	result, err := installWithDependencies(homeDir, newTestRegistry(), model.Selection{
		Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentSDD},
	}, "test-v1", false, deps)
	if !errors.Is(err, boom) || result.BackupID == "" {
		t.Fatalf("Install() = (%+v, %v), want workflow failure with backup evidence", result, err)
	}
	assertInstallCoordinatorPreimages(t, before)
}

func TestInstallSkipsNoOpComponentWithoutDeclaredTargets(t *testing.T) {
	homeDir := t.TempDir()
	result, err := Install(homeDir, newTestRegistry(), model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentSkills},
	}, "test-v1", false)
	if err != nil {
		t.Fatalf("Install() error = %v\nErrors: %v", err, result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Install() produced warnings: %v", result.Errors)
	}
	skillsRoot := filepath.Join(homeDir, ".config", "opencode", "skills")
	if entries, readErr := os.ReadDir(skillsRoot); readErr == nil && len(entries) != 0 {
		t.Fatalf("no-op skills step wrote files under %s: %v", skillsRoot, entries)
	}
}

func TestInstallRestoreFailureAndVerificationAreReported(t *testing.T) {
	for _, name := range []string{"restore", "verification"} {
		t.Run(name, func(t *testing.T) {
			homeDir, before := installCoordinatorPreimages(t, "empty")
			primary := errors.New("injected component failure")
			recovery := fmt.Errorf("injected %s failure", name)
			deps := defaultInstallDependencies()
			deps.invokeComponent = func(_ model.ComponentID, invoke func() ([]string, error)) ([]string, error) {
				files, err := invoke()
				if err != nil {
					return files, err
				}
				return files, primary
			}
			deps.restoreAndVerify = func(journal *InstallJournal) error {
				if err := journal.RestoreAndVerify(); err != nil {
					return err
				}
				return recovery
			}
			_, err := installWithDependencies(homeDir, newTestRegistry(), model.Selection{
				Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentCortex},
			}, "test-v1", false, deps)
			if !errors.Is(err, primary) || !strings.Contains(err.Error(), recovery.Error()) {
				t.Fatalf("Install() error = %v, want primary and %s evidence", err, name)
			}
			assertInstallCoordinatorPreimages(t, before)
		})
	}
}

type installTestFile struct {
	present bool
	mode    os.FileMode
	content []byte
}

func installCoordinatorPreimages(t *testing.T, configState string) (string, map[string]installTestFile) {
	t.Helper()
	homeDir := t.TempDir()
	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	if configState != "absent" {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			t.Fatal(err)
		}
		content := []byte("# existing config\n")
		if configState == "empty" {
			content = nil
		}
		if err := os.WriteFile(configPath, content, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.Save(homeDir, state.State{Version: "before-state"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveLock(homeDir, state.Lockfile{Version: "before-lock"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveInstallStatus(homeDir, state.InstallStatus{Status: "before", BackupID: "before"}); err != nil {
		t.Fatal(err)
	}
	paths := []string{configPath, state.StatePath(homeDir), state.LockPath(homeDir), state.InstallStatusPath(homeDir)}
	before := make(map[string]installTestFile, len(paths))
	for _, path := range paths {
		before[path] = captureInstallTestFile(t, path)
	}
	return homeDir, before
}

func captureInstallTestFile(t *testing.T, path string) installTestFile {
	t.Helper()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return installTestFile{}
	}
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return installTestFile{present: true, mode: info.Mode().Perm(), content: content}
}

func assertInstallCoordinatorPreimages(t *testing.T, want map[string]installTestFile) {
	t.Helper()
	for path, image := range want {
		got := captureInstallTestFile(t, path)
		if got.present != image.present || got.mode != image.mode || string(got.content) != string(image.content) {
			t.Fatalf("restored %q = %+v, want %+v", path, got, image)
		}
	}
}

func TestInstall_OpenCodeCoexistenceMutatesOnlyJSONC(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(configDir, "opencode.json")
	jsonBefore := []byte(`{"lower_precedence":"unchanged"}`)
	if err := os.WriteFile(jsonPath, jsonBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	jsoncPath := filepath.Join(configDir, "opencode.jsonc")
	jsoncBefore := []byte("{\n  // Effective user configuration.\n  \"share\": \"disabled\",\n}\n")
	if err := os.WriteFile(jsoncPath, jsoncBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	if _, err := Install(homeDir, newTestRegistry(), selection, "test-v1", false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	jsonAfter, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(jsonAfter) != string(jsonBefore) {
		t.Fatalf("lower-precedence JSON changed:\n%s", jsonAfter)
	}
	jsoncAfter, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsoncAfter), "// Effective user configuration.") {
		t.Fatalf("pipeline discarded JSONC comments:\n%s", jsoncAfter)
	}
	config, err := filemerge.DecodeJSONObject(jsoncAfter)
	if err != nil {
		t.Fatal(err)
	}
	if config["share"] != "disabled" {
		t.Fatalf("effective JSONC lost user setting: %#v", config)
	}
}

// ---------------------------------------------------------------------------
// Repair
// ---------------------------------------------------------------------------

func TestRepair_Basic(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
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
	result := dedupeAgents([]model.AgentID{"", model.AgentOpenCode, ""})
	if len(result) != 1 || result[0] != model.AgentOpenCode {
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
	paths := collectBackupPaths(homeDir, registry, []model.AgentID{model.AgentOpenCode}, components)

	if len(paths) == 0 {
		t.Error("expected non-empty backup paths")
	}

	hasPrompt := false
	hasSettings := false
	for _, p := range paths {
		if strings.EqualFold(filepath.Base(p), "AGENTS.md") {
			hasPrompt = true
		}
		if strings.Contains(p, "opencode.json") {
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
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	_, err := Install(homeDir, registry, selection, "v1", false)
	if err == nil || !strings.Contains(err.Error(), "state.json\" has incompatible existing type") {
		t.Fatalf("Install() error = %v, want incompatible state target error", err)
	}
}

func TestInstall_LockSaveError(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Pre-create lock file as a directory → SaveLock will fail.
	os.MkdirAll(filepath.Join(homeDir, ".cortex-ia", "cortex-ia.lock"), 0o755)

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	_, err := Install(homeDir, registry, selection, "v1", false)
	if err == nil || !strings.Contains(err.Error(), "cortex-ia.lock\" has incompatible existing type") {
		t.Fatalf("Install() error = %v, want incompatible lock target error", err)
	}
}

func TestCollectBackupPaths_WithExistingMCPConfig(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Create MCP config file so os.Stat succeeds.
	mcpPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	os.MkdirAll(filepath.Dir(mcpPath), 0o755)
	os.WriteFile(mcpPath, []byte("{}"), 0o644)

	components := []model.ComponentID{model.ComponentCortex}
	paths := collectBackupPaths(homeDir, registry, []model.AgentID{model.AgentOpenCode}, components)

	hasMCP := false
	for _, p := range paths {
		if strings.Contains(p, "opencode.json") {
			hasMCP = true
			break
		}
	}
	if !hasMCP {
		t.Error("expected MCP config in backup paths when file exists")
	}
}

func TestOpenCodeInstallation_AssetsAndConfigValidation(t *testing.T) {
	homeDir := t.TempDir()
	registry := agents.NewDefaultRegistry()
	selection := model.Selection{
		Agents:    []model.AgentID{model.AgentOpenCode},
		Preset:    model.PresetFull,
		Persona:   model.PersonaProfessional,
		StrictTDD: true,
	}

	result, err := Install(homeDir, registry, selection, "v1.0.0", false)
	if err != nil {
		t.Fatalf("Install() error = %v, errors: %v", err, result.Errors)
	}

	// 1. Validate AGENTS.md exists and contains expected persona/header content.
	agentsMDPath := filepath.Join(homeDir, ".config", "opencode", "AGENTS.md")
	content, err := os.ReadFile(agentsMDPath)
	if err != nil {
		t.Fatalf("AGENTS.md missing: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("AGENTS.md is empty")
	}

	// 2. Validate opencode.json exists and contains valid JSON with MCP servers.
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("opencode.json missing: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		t.Fatalf("opencode.json is not valid JSON: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok || len(mcp) == 0 {
		t.Errorf("opencode.json missing 'mcp' section or has no servers, got: %v", cfg)
	}

	// 3. Validate subagents copied to .config/opencode/agents/
	agentsDir := filepath.Join(homeDir, ".config", "opencode", "agents")
	agentEntries, err := os.ReadDir(agentsDir)
	if err != nil || len(agentEntries) == 0 {
		t.Fatalf("no subagents found in %s: %v", agentsDir, err)
	}

	// 4. Validate skills copied to .config/opencode/skills/
	skillsDir := filepath.Join(homeDir, ".config", "opencode", "skills")
	skillEntries, err := os.ReadDir(skillsDir)
	if err != nil || len(skillEntries) == 0 {
		t.Fatalf("no skills found in %s: %v", skillsDir, err)
	}

	// 5. Validate commands copied to .config/opencode/commands/
	commandsDir := filepath.Join(homeDir, ".config", "opencode", "commands")
	cmdEntries, err := os.ReadDir(commandsDir)
	if err != nil || len(cmdEntries) == 0 {
		t.Fatalf("no commands found in %s: %v", commandsDir, err)
	}
}
