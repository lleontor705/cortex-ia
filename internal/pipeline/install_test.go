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
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	sddprompt "github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	sddregistry "github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
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

func testModelRoutes() sddprompt.ModelTable {
	return sddprompt.ModelTable{}
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
			selection := model.Selection{Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentCortex}}
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

// ---------------------------------------------------------------------------
// WU-17: global preflight, registry overlay integration, transactional apply
// ---------------------------------------------------------------------------
//
// The oracles below complete the 36 spec oracle names of slice
// declarative-skill-registry-foundation (spec
// sdd-de4191a255e941d59ada39b6a7510011). The registry-level unit oracles
// already live in internal/components/sdd/registry/registry_test.go (WU-08);
// every oracle added here exercises the pipeline integration boundary —
// BuildInstallPlan preflight, journal-covered registry apply, and restore —
// exclusively in temporary homes.

const (
	pipelineCustomSkillID  = "deploy-helper"
	pipelineCustomSkillID2 = "release-notes"
)

func pipelineCustomSkillDoc(name, body string) []byte {
	return []byte(fmt.Sprintf("---\nname: %s\n---\n\n# %s\n\n%s\n", name, name, body))
}

type pipelineCustomSkill struct {
	dir  string
	name string
	body string
}

// writePipelineOverlay writes a local overlay configuration file plus one
// custom skill directory per entry beneath the same configuration root, and
// returns the transport-only registry selection that declares them.
func writePipelineOverlay(t *testing.T, homeDir string, skills []pipelineCustomSkill, disabled ...model.ComponentID) *model.RegistrySelection {
	t.Helper()
	root := filepath.Join(homeDir, "overlay")
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "cortex-ia.yaml")
	if err := os.WriteFile(configFile, []byte("preset: full\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selection := &model.RegistrySelection{ConfigFile: configFile, DisabledComponents: disabled}
	for _, skill := range skills {
		dir := filepath.Join(root, "skills", skill.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), pipelineCustomSkillDoc(skill.name, skill.body), 0o644); err != nil {
			t.Fatal(err)
		}
		selection.CustomSkillPaths = append(selection.CustomSkillPaths, dir)
	}
	return selection
}

// writeEscapingOverlay declares a custom skill directory that resolves outside
// the configuration root; containment must reject it before any write.
func writeEscapingOverlay(t *testing.T, homeDir string) *model.RegistrySelection {
	t.Helper()
	root := filepath.Join(homeDir, "overlay")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "cortex-ia.yaml")
	if err := os.WriteFile(configFile, []byte("preset: full\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(t.TempDir(), "escaping-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), pipelineCustomSkillDoc("escapee", "body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &model.RegistrySelection{ConfigFile: configFile, CustomSkillPaths: []string{skillDir}}
}

func pipelineCustomSkillPath(homeDir, id string) string {
	return filepath.Join(homeDir, ".config", "opencode", "skills", id, "SKILL.md")
}

func pipelineLightSelection(overlay *model.RegistrySelection, components ...model.ComponentID) model.Selection {
	if len(components) == 0 {
		components = []model.ComponentID{model.ComponentCortex}
	}
	return model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: components,
		Registry:   overlay,
	}
}

func pipelineSubtreeDigest(t *testing.T, root string) string {
	t.Helper()
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return "absent"
	}
	return testTreeDigest(t, root)
}

func pipelineTreeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
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
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// requireOnlyAddedFiles asserts the after-tree equals the before-tree plus
// exactly the wanted added files: bounded overlay effect, byte-for-byte.
func requireOnlyAddedFiles(t *testing.T, before, after map[string]string, wantAdded []string) {
	t.Helper()
	var added, changed, removed []string
	for path, content := range after {
		prior, ok := before[path]
		if !ok {
			added = append(added, path)
			continue
		}
		if prior != content {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(added)
	if !reflect.DeepEqual(added, wantAdded) {
		t.Fatalf("added files = %v, want exactly %v (changed=%v removed=%v)", added, wantAdded, changed, removed)
	}
	if len(changed) != 0 || len(removed) != 0 {
		t.Fatalf("effect unbounded: changed=%v removed=%v", changed, removed)
	}
}

func pipelineCommittedReceipt(t *testing.T, homeDir string) (sddregistry.Receipt, bool) {
	t.Helper()
	receipt, err := sdd.LoadCommittedRegistryReceipt(homeDir)
	if errors.Is(err, os.ErrNotExist) {
		return sddregistry.Receipt{}, false
	}
	if err != nil {
		t.Fatalf("load committed registry receipt: %v", err)
	}
	return receipt, true
}

func pipelineMCPServerNames(t *testing.T, homeDir string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(homeDir, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	if mcp, ok := config["mcp"].(map[string]any); ok {
		for name := range mcp {
			names[name] = true
		}
	}
	return names
}

// runPreflightOrderedInstall instruments every mutation seam of the install
// coordinator and records the observed event order so tests can prove
// validate-success precedes the first managed write (AC-SAFE-1).
func runPreflightOrderedInstall(t *testing.T, homeDir string, selection model.Selection) ([]string, InstallResult, error) {
	t.Helper()
	deps := defaultInstallDependencies()
	var mu sync.Mutex
	var events []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}
	deps.buildInstallPlan = func(ctx context.Context, home string, agentRegistry *agents.Registry, sel model.Selection, resolved []model.ComponentID) (InstallPlan, error) {
		plan, planErr := BuildInstallPlan(ctx, home, agentRegistry, sel, resolved)
		if planErr == nil {
			record("validate:install-plan")
		}
		return plan, planErr
	}
	deps.beginJournal = func(home, checkpoint string, targets []ManagedTarget) (*InstallJournal, error) {
		record("write:journal")
		return BeginInstallJournal(home, checkpoint, targets)
	}
	deps.saveInstallStatus = func(home string, status state.InstallStatus) error {
		record("write:status")
		return state.SaveInstallStatus(home, status)
	}
	invoke := deps.invokeComponent
	deps.invokeComponent = func(id model.ComponentID, run func() ([]string, error)) ([]string, error) {
		record("write:component:" + string(id))
		return invoke(id, run)
	}
	applyWorkflow := deps.applyWorkflow
	deps.applyWorkflow = func(prepared PreparedWorkflowInstall) (sddinstall.Receipt, error) {
		record("write:workflow")
		return applyWorkflow(prepared)
	}
	applyRegistry := deps.applyRegistryPlan
	deps.applyRegistryPlan = func(home string, plan sdd.GlobalInstallPlan) (sdd.GlobalApplyResult, error) {
		record("write:registry")
		return applyRegistry(home, plan)
	}
	saveState := deps.saveState
	deps.saveState = func(home string, value state.State) error {
		record("write:state")
		return saveState(home, value)
	}
	saveLock := deps.saveLock
	deps.saveLock = func(home string, value state.Lockfile) error {
		record("write:lock")
		return saveLock(home, value)
	}
	clearStatus := deps.clearInstallStatus
	deps.clearInstallStatus = func(home string) error {
		record("write:status-clear")
		return clearStatus(home)
	}
	result, err := installWithDependencies(homeDir, newTestRegistry(), selection, "test-v1", false, deps)
	return events, result, err
}

func requireValidationPrecedesWrites(t *testing.T, events []string) {
	t.Helper()
	validateIndex, firstWrite := -1, -1
	for index, event := range events {
		if strings.HasPrefix(event, "validate:") && validateIndex == -1 {
			validateIndex = index
		}
		if strings.HasPrefix(event, "write:") && firstWrite == -1 {
			firstWrite = index
		}
	}
	if validateIndex == -1 {
		t.Fatalf("no validate-success event recorded: %v", events)
	}
	if firstWrite == -1 {
		t.Fatalf("no write events recorded: %v", events)
	}
	if validateIndex > firstWrite {
		t.Fatalf("validate-success (%q at %d) did not precede first write (%q at %d): %v",
			events[validateIndex], validateIndex, events[firstWrite], firstWrite, events)
	}
}

// --- REQ-SAFE-001 ----------------------------------------------------------

func TestSpec_REQ_SAFE_001_ValidateBeforeWrite(t *testing.T) {
	homeDir := t.TempDir()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	events, result, err := runPreflightOrderedInstall(t, homeDir, pipelineLightSelection(overlay))
	if err != nil {
		t.Fatalf("Install() error = %v; errors: %v", err, result.Errors)
	}
	requireValidationPrecedesWrites(t, events)
	if _, readErr := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); readErr != nil {
		t.Fatalf("validated install did not write the custom skill: %v", readErr)
	}
}

func TestPipeline_ValidateBeforeWrite(t *testing.T) {
	homeDir := t.TempDir()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	events, _, err := runPreflightOrderedInstall(t, homeDir, pipelineLightSelection(overlay))
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	requireValidationPrecedesWrites(t, events)
	if _, ok := pipelineCommittedReceipt(t, homeDir); !ok {
		t.Fatal("validated install committed no registry receipt")
	}
}

func TestSpec_REQ_SAFE_001_MultiTargetPreflightFailure(t *testing.T) {
	freshHome := t.TempDir()
	priorHome := t.TempDir()
	registry := newTestRegistry()

	// The prior home already carries a valid overlay install; the fresh home
	// has none. Both later receive the same invalid input.
	validOverlay := writePipelineOverlay(t, priorHome, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(priorHome, registry, pipelineLightSelection(validOverlay), "test-v1", false); err != nil {
		t.Fatalf("prior Install() error = %v", err)
	}

	escapingFresh := writeEscapingOverlay(t, freshHome)
	escapingPrior := writeEscapingOverlay(t, priorHome)
	priorBefore := testTreeDigest(t, priorHome)
	freshBefore := testTreeDigest(t, freshHome)

	for _, home := range []struct {
		dir     string
		overlay *model.RegistrySelection
	}{
		{dir: freshHome, overlay: escapingFresh},
		{dir: priorHome, overlay: escapingPrior},
	} {
		_, err := Install(home.dir, registry, pipelineLightSelection(home.overlay), "test-v1", false)
		if err == nil {
			t.Fatalf("Install(%s) succeeded with an escaping overlay", home.dir)
		}
		var installErr *sddregistry.InstallError
		if !errors.As(err, &installErr) || installErr.Primary.Class != sddregistry.ErrorUntrusted {
			t.Fatalf("Install(%s) error = %v, want untrusted InstallError", home.dir, err)
		}
	}
	if got := testTreeDigest(t, freshHome); got != freshBefore {
		t.Fatalf("fresh home changed after preflight failure: before=%s after=%s", freshBefore, got)
	}
	if got := testTreeDigest(t, priorHome); got != priorBefore {
		t.Fatalf("prior home changed after preflight failure: before=%s after=%s", priorBefore, got)
	}
}

func TestPipeline_MultiTargetPreflightFailure(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	registry := newTestRegistry()
	overlayA := writeEscapingOverlay(t, homeA)
	overlayB := writeEscapingOverlay(t, homeB)
	beforeA, beforeB := testTreeDigest(t, homeA), testTreeDigest(t, homeB)

	for _, home := range []struct {
		dir     string
		overlay *model.RegistrySelection
		before  string
	}{
		{dir: homeA, overlay: overlayA, before: beforeA},
		{dir: homeB, overlay: overlayB, before: beforeB},
	} {
		result, err := Install(home.dir, registry, pipelineLightSelection(home.overlay), "test-v1", false)
		if err == nil {
			t.Fatalf("Install(%s) succeeded with invalid input", home.dir)
		}
		if result.BackupID != "" || len(result.FilesChanged) != 0 {
			t.Fatalf("invalid input produced install evidence on %s: %+v", home.dir, result)
		}
		if got := testTreeDigest(t, home.dir); got != home.before {
			t.Fatalf("home %s changed: before=%s after=%s", home.dir, home.before, got)
		}
	}
}

func TestSpec_REQ_SAFE_001_InvalidInputZeroWrites(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	// Break the overlay: the custom skill directory no longer holds exactly
	// one SKILL.md, which is an invalid declaration.
	if err := os.Remove(filepath.Join(filepath.Dir(overlay.ConfigFile), "skills", pipelineCustomSkillID, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	before := testTreeDigest(t, homeDir)

	_, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false)
	if err == nil {
		t.Fatal("Install() succeeded with an invalid overlay")
	}
	var installErr *sddregistry.InstallError
	if !errors.As(err, &installErr) || installErr.Primary.Class != sddregistry.ErrorInvalid {
		t.Fatalf("Install() error = %v, want invalid InstallError", err)
	}
	if got := testTreeDigest(t, homeDir); got != before {
		t.Fatalf("invalid overlay mutated the prior home snapshot: before=%s after=%s", before, got)
	}
}

func TestPipeline_InvalidInputZeroWrites(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(overlay.ConfigFile), "skills", pipelineCustomSkillID, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	before := testTreeDigest(t, homeDir)

	result, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false)
	if err == nil {
		t.Fatal("Install() succeeded with an invalid overlay")
	}
	if result.BackupID != "" {
		t.Fatalf("invalid overlay created a backup: %q", result.BackupID)
	}
	if got := testTreeDigest(t, homeDir); got != before {
		t.Fatalf("invalid overlay left writes behind: before=%s after=%s", before, got)
	}
}

// --- REQ-ROLL-001 / REQ-DIAG-001 mutation oracles ---------------------------

func TestPipeline_TransactionalRestore(t *testing.T) {
	homeDir := t.TempDir()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
		{dir: pipelineCustomSkillID2, name: pipelineCustomSkillID2, body: "Writes release notes.\n"},
	})
	beforeConfig := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config"))
	deps := defaultInstallDependencies()
	deps.applyRegistryPlan = func(home string, plan sdd.GlobalInstallPlan) (sdd.GlobalApplyResult, error) {
		// Apply only the first planned write, then fail: no partial success
		// may survive.
		if len(plan.Adapters) == 0 || len(plan.Adapters[0].Ops) == 0 {
			return sdd.GlobalApplyResult{}, errors.New("injected plan had no adapter ops")
		}
		first := plan.Adapters[0].Ops[0]
		target := filepath.Join(home, filepath.FromSlash(first.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return sdd.GlobalApplyResult{}, err
		}
		if err := os.WriteFile(target, first.Content, 0o644); err != nil {
			return sdd.GlobalApplyResult{}, err
		}
		return sdd.GlobalApplyResult{}, errors.New("injected mid-apply write failure")
	}

	result, err := installWithDependencies(homeDir, newTestRegistry(), pipelineLightSelection(overlay), "test-v1", false, deps)
	if err == nil || !strings.Contains(err.Error(), "injected mid-apply write failure") {
		t.Fatalf("Install() error = %v, want injected mid-apply failure", err)
	}
	for _, id := range []string{pipelineCustomSkillID, pipelineCustomSkillID2} {
		if _, statErr := os.Stat(pipelineCustomSkillPath(homeDir, id)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("partial success survived for %s: %v", id, statErr)
		}
	}
	if _, ok := pipelineCommittedReceipt(t, homeDir); ok {
		t.Fatal("failed apply left a committed registry receipt behind")
	}
	if got := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config")); got != beforeConfig {
		t.Fatalf("touched paths were not restored: before=%s after=%s", beforeConfig, got)
	}
	if result.BackupID == "" {
		t.Fatal("write failure lost backup evidence")
	}
}

func TestSpec_REQ_DIAG_001_NoFalseSuccessOnMutationFailure(t *testing.T) {
	homeDir := t.TempDir()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	beforeConfig := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config"))
	deps := defaultInstallDependencies()
	deps.applyRegistryPlan = func(home string, plan sdd.GlobalInstallPlan) (sdd.GlobalApplyResult, error) {
		outcome, applyErr := sdd.ApplyGlobalInstallPlan(home, plan)
		if applyErr != nil {
			return outcome, applyErr
		}
		// The writes converged, but the install must still report the write
		// failure and restore every touched path (write class, complete
		// restoration: no rollback diagnostic).
		return outcome, errors.New("injected post-apply write failure")
	}

	_, err := installWithDependencies(homeDir, newTestRegistry(), pipelineLightSelection(overlay), "test-v1", false, deps)
	if err == nil || !strings.Contains(err.Error(), "injected post-apply write failure") {
		t.Fatalf("Install() error = %v, want injected write failure", err)
	}
	var installErr *sddregistry.InstallError
	if errors.As(err, &installErr) && installErr.Rollback != nil {
		t.Fatalf("complete restoration reported a rollback diagnostic: %+v", installErr.Rollback)
	}
	if _, statErr := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("written custom skill survived a write failure: %v", statErr)
	}
	if _, ok := pipelineCommittedReceipt(t, homeDir); ok {
		t.Fatal("failed install committed a registry receipt")
	}
	if got := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config")); got != beforeConfig {
		t.Fatalf("write failure did not restore touched paths: before=%s after=%s", beforeConfig, got)
	}
}

func TestSpec_REQ_ROLL_001_ResidualPreventsFalseSuccess(t *testing.T) {
	homeDir := t.TempDir()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	deps := defaultInstallDependencies()
	deps.applyRegistryPlan = func(home string, plan sdd.GlobalInstallPlan) (sdd.GlobalApplyResult, error) {
		// Simulate a failed apply whose own rollback left a residual behind:
		// the coordinator must distinguish rollback-incomplete from write and
		// never claim success.
		target := filepath.Join(home, ".config", "opencode", "skills", pipelineCustomSkillID, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return sdd.GlobalApplyResult{}, err
		}
		if err := os.WriteFile(target, []byte("residual\n"), 0o644); err != nil {
			return sdd.GlobalApplyResult{}, err
		}
		rollback := sddregistry.Diagnostic{
			Class: sddregistry.ErrorRollback, Stage: sddregistry.StageRollback, Rule: sdd.RuleRollbackResidual,
			Cause: fmt.Errorf("residual paths: %s", target),
		}
		return sdd.GlobalApplyResult{}, &sddregistry.InstallError{
			Primary: sddregistry.Diagnostic{
				Class: sddregistry.ErrorWrite, Stage: sddregistry.StageApply, Rule: sdd.RuleApplyWriteFailed,
				Cause: errors.New("injected write failure"),
			},
			All:      []sddregistry.Diagnostic{{Class: sddregistry.ErrorWrite, Stage: sddregistry.StageApply, Rule: sdd.RuleApplyWriteFailed}},
			Rollback: &rollback,
		}
	}

	result, err := installWithDependencies(homeDir, newTestRegistry(), pipelineLightSelection(overlay), "test-v1", false, deps)
	if err == nil {
		t.Fatal("rollback-incomplete apply reported success")
	}
	var installErr *sddregistry.InstallError
	if !errors.As(err, &installErr) || installErr.Rollback == nil || installErr.Rollback.Class != sddregistry.ErrorRollback {
		t.Fatalf("Install() error = %v, want rollback-incomplete InstallError", err)
	}
	if _, ok := pipelineCommittedReceipt(t, homeDir); ok {
		t.Fatal("rollback-incomplete install committed a baseline registry receipt")
	}
	// The journal's verified restoration still converges the residual.
	if _, statErr := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("residual survived journal restoration: %v", statErr)
	}
	if result.WorkflowReceipt.ID != "" {
		t.Fatalf("failed install reported workflow success evidence: %+v", result)
	}
}

func TestSpec_REQ_ROLL_001_RemoveOverlayRestoresBaseline(t *testing.T) {
	homeDir := t.TempDir()
	control := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})

	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("overlay Install() error = %v", err)
	}
	if _, err := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); err != nil {
		t.Fatalf("custom skill missing after overlay install: %v", err)
	}

	// Retire the overlay entirely and reinstall: the baseline must return and
	// the formerly managed custom output must be gone.
	if _, err := Install(homeDir, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("baseline reinstall error = %v", err)
	}
	if _, statErr := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("custom skill survived overlay removal: %v", statErr)
	}
	if _, err := Install(control, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("control Install() error = %v", err)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config")), pipelineSubtreeDigest(t, filepath.Join(control, ".config")); got != want {
		t.Fatalf("baseline not restored exactly: got=%s want=%s", got, want)
	}
	receipt, ok := pipelineCommittedReceipt(t, homeDir)
	if !ok {
		t.Fatal("overlay removal committed no registry receipt")
	}
	if len(receipt.HostOutputs) != 0 {
		t.Fatalf("baseline receipt still lists custom host outputs: %v", receipt.HostOutputs)
	}
}

func TestSpec_REQ_ROLL_001_RejectedOverlayLeavesNoResidue(t *testing.T) {
	homeDir := t.TempDir()
	control := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	// A rejected overlay (duplicate custom ID, even with identical bytes) must
	// change nothing; the follow-up baseline install then needs no manual
	// cleanup to restore the exact baseline.
	rejected := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: "one", name: pipelineCustomSkillID, body: "Deploys the service.\n"},
		{dir: "two", name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	afterFirst := testTreeDigest(t, homeDir)
	_, err := Install(homeDir, registry, pipelineLightSelection(rejected), "test-v1", false)
	if err == nil {
		t.Fatal("Install() succeeded with a colliding overlay")
	}
	var installErr *sddregistry.InstallError
	if !errors.As(err, &installErr) || installErr.Primary.Class != sddregistry.ErrorCollision {
		t.Fatalf("Install() error = %v, want collision InstallError", err)
	}
	if got := testTreeDigest(t, homeDir); got != afterFirst {
		t.Fatalf("rejected overlay left residue: before=%s after=%s", afterFirst, got)
	}

	if _, err := Install(homeDir, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("baseline reinstall error = %v", err)
	}
	if _, statErr := os.Stat(pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("custom skill survived rejected-overlay recovery: %v", statErr)
	}
	if _, err := Install(control, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("control Install() error = %v", err)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(homeDir, ".config")), pipelineSubtreeDigest(t, filepath.Join(control, ".config")); got != want {
		t.Fatalf("baseline not restored without manual cleanup: got=%s want=%s", got, want)
	}
}

// --- REQ-BASE-001 / REQ-SEL-001 --------------------------------------------

func TestSpec_REQ_BASE_001_NoOverlayBaseline(t *testing.T) {
	plainHome := t.TempDir()
	emptyOverlayHome := t.TempDir()
	registry := newTestRegistry()
	emptyOverlay := writePipelineOverlay(t, emptyOverlayHome, nil)

	plain, err := Install(plainHome, registry, pipelineLightSelection(nil, model.ComponentSDD), "test-v1", false)
	if err != nil {
		t.Fatalf("no-overlay Install() error = %v; errors: %v", err, plain.Errors)
	}
	empty, err := Install(emptyOverlayHome, registry, pipelineLightSelection(emptyOverlay, model.ComponentSDD), "test-v1", false)
	if err != nil {
		t.Fatalf("empty-overlay Install() error = %v", err)
	}
	if plain.WorkflowFingerprint == "" || plain.WorkflowFingerprint != empty.WorkflowFingerprint {
		t.Fatalf("workflow fingerprints differ from baseline: plain=%q empty=%q", plain.WorkflowFingerprint, empty.WorkflowFingerprint)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(emptyOverlayHome, ".config")), pipelineSubtreeDigest(t, filepath.Join(plainHome, ".config")); got != want {
		t.Fatalf("empty-overlay outputs differ from baseline: got=%s want=%s", got, want)
	}
	if _, ok := pipelineCommittedReceipt(t, plainHome); ok {
		t.Fatal("no-overlay install committed a registry receipt")
	}
	if _, ok := pipelineCommittedReceipt(t, emptyOverlayHome); ok {
		t.Fatal("empty-overlay install committed a registry receipt")
	}
}

func TestSpec_REQ_BASE_001_EmptyOverlayIsBaseline(t *testing.T) {
	plainHome := t.TempDir()
	emptyOverlayHome := t.TempDir()
	registry := newTestRegistry()
	emptyOverlay := writePipelineOverlay(t, emptyOverlayHome, nil)

	if _, err := Install(plainHome, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("no-overlay Install() error = %v", err)
	}
	if _, err := Install(emptyOverlayHome, registry, pipelineLightSelection(emptyOverlay), "test-v1", false); err != nil {
		t.Fatalf("empty-overlay Install() error = %v", err)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(emptyOverlayHome, ".config")), pipelineSubtreeDigest(t, filepath.Join(plainHome, ".config")); got != want {
		t.Fatalf("empty overlay is not byte-for-byte the baseline: got=%s want=%s", got, want)
	}
	if _, statErr := os.Stat(pipelineCustomSkillPath(emptyOverlayHome, pipelineCustomSkillID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty overlay left a custom skill residue: %v", statErr)
	}
	if _, ok := pipelineCommittedReceipt(t, emptyOverlayHome); ok {
		t.Fatal("empty overlay left a receipt residue")
	}
}

func TestSpec_REQ_SEL_001_DisableOptionalComponent(t *testing.T) {
	homeDir := t.TempDir()
	control := t.TempDir()
	registry := newTestRegistry()
	components := []model.ComponentID{model.ComponentCortex, model.ComponentContext7}
	overlay := writePipelineOverlay(t, homeDir, nil, model.ComponentContext7)

	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay, components...), "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := Install(control, registry, pipelineLightSelection(nil, components...), "test-v1", false); err != nil {
		t.Fatalf("control Install() error = %v", err)
	}

	disabledNames := pipelineMCPServerNames(t, homeDir)
	if disabledNames["context7"] {
		t.Fatal("disabled optional component context7 was still installed")
	}
	if !disabledNames["cortex"] {
		t.Fatal("protected component cortex missing after optional disable")
	}
	if names := pipelineMCPServerNames(t, control); !names["context7"] {
		t.Fatal("control install lost context7 without a disable")
	}
	receipt, ok := pipelineCommittedReceipt(t, homeDir)
	if !ok {
		t.Fatal("overlay disable install committed no registry receipt")
	}
	if slices.Contains(receipt.EffectiveComponents, model.ComponentContext7) {
		t.Fatalf("receipt still lists the disabled component: %v", receipt.EffectiveComponents)
	}
}

func TestSpec_REQ_SEL_001_RepeatedOptionalDisable(t *testing.T) {
	onceHome := t.TempDir()
	repeatedHome := t.TempDir()
	registry := newTestRegistry()
	components := []model.ComponentID{model.ComponentCortex, model.ComponentContext7}
	once := writePipelineOverlay(t, onceHome, nil, model.ComponentContext7)
	repeated := writePipelineOverlay(t, repeatedHome, nil, model.ComponentContext7, model.ComponentContext7)

	if _, err := Install(onceHome, registry, pipelineLightSelection(once, components...), "test-v1", false); err != nil {
		t.Fatalf("single disable Install() error = %v", err)
	}
	if _, err := Install(repeatedHome, registry, pipelineLightSelection(repeated, components...), "test-v1", false); err != nil {
		t.Fatalf("repeated disable Install() error = %v", err)
	}

	onceReceipt, ok := pipelineCommittedReceipt(t, onceHome)
	if !ok {
		t.Fatal("single disable committed no receipt")
	}
	repeatedReceipt, ok := pipelineCommittedReceipt(t, repeatedHome)
	if !ok {
		t.Fatal("repeated disable committed no receipt")
	}
	if onceReceipt.Fingerprint != repeatedReceipt.Fingerprint {
		t.Fatalf("repeated disable is not equivalent to one: once=%s repeated=%s", onceReceipt.Fingerprint, repeatedReceipt.Fingerprint)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(repeatedHome, ".config")), pipelineSubtreeDigest(t, filepath.Join(onceHome, ".config")); got != want {
		t.Fatalf("repeated disable produced different outputs: got=%s want=%s", got, want)
	}
}

// --- REQ-ADAPT-001 ----------------------------------------------------------

func TestSpec_REQ_ADAPT_001_EquivalentSelectionAcrossAdapters(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	registry := newTestRegistry()
	doc := "Deploys the service.\n"
	overlayA := writePipelineOverlay(t, homeA, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: doc}})
	overlayB := writePipelineOverlay(t, homeB, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: doc}})

	for _, home := range []struct {
		dir     string
		overlay *model.RegistrySelection
	}{{homeA, overlayA}, {homeB, overlayB}} {
		if _, err := Install(home.dir, registry, pipelineLightSelection(home.overlay), "test-v1", false); err != nil {
			t.Fatalf("Install(%s) error = %v", home.dir, err)
		}
	}

	contentA, err := os.ReadFile(pipelineCustomSkillPath(homeA, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	contentB, err := os.ReadFile(pipelineCustomSkillPath(homeB, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	if string(contentA) != string(pipelineCustomSkillDoc(pipelineCustomSkillID, doc)) || string(contentA) != string(contentB) {
		t.Fatalf("equivalent selections produced different representations: a=%q b=%q", contentA, contentB)
	}

	// The destination must be exactly the adapter-declared layout, not a
	// pipeline-invented location.
	var opencodeAdapter agents.Adapter = opencode.NewAdapter()
	provider, ok := opencodeAdapter.(agents.SkillLayoutProvider)
	if !ok {
		t.Fatal("opencode adapter does not implement agents.SkillLayoutProvider")
	}
	relative, err := filepath.Rel(homeA, pipelineCustomSkillPath(homeA, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	declared := provider.SkillDestinations(skillcore.Skill{ID: model.SkillID(pipelineCustomSkillID)})
	if !reflect.DeepEqual(declared, []string{filepath.ToSlash(relative)}) {
		t.Fatalf("installed destination %q is not the adapter-declared layout %v", relative, declared)
	}

	receiptA, ok := pipelineCommittedReceipt(t, homeA)
	if !ok {
		t.Fatal("home A committed no receipt")
	}
	receiptB, ok := pipelineCommittedReceipt(t, homeB)
	if !ok {
		t.Fatal("home B committed no receipt")
	}
	if receiptA.Fingerprint != receiptB.Fingerprint {
		t.Fatalf("equivalent selections produced different evidence: %s vs %s", receiptA.Fingerprint, receiptB.Fingerprint)
	}
}

func TestPipeline_EquivalentSelectionAcrossAdapters(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	registry := newTestRegistry()
	overlayA := writePipelineOverlay(t, homeA, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "alpha body\n"}})
	overlayB := writePipelineOverlay(t, homeB, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "beta body\n"}})

	if _, err := Install(homeA, registry, pipelineLightSelection(overlayA), "test-v1", false); err != nil {
		t.Fatalf("Install(A) error = %v", err)
	}
	if _, err := Install(homeB, registry, pipelineLightSelection(overlayB), "test-v1", false); err != nil {
		t.Fatalf("Install(B) error = %v", err)
	}
	gotA, err := os.ReadFile(pipelineCustomSkillPath(homeA, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := os.ReadFile(pipelineCustomSkillPath(homeB, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotA), "alpha body") || !strings.Contains(string(gotB), "beta body") {
		t.Fatalf("per-home selections contaminated each other: a=%q b=%q", gotA, gotB)
	}
}

func TestSpec_REQ_ADAPT_001_TemporaryHomesIsolated(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	registry := newTestRegistry()
	overlayA := writePipelineOverlay(t, homeA, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "alpha body\n"}})
	overlayB := writePipelineOverlay(t, homeB, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "beta body\n"}})

	// Installing home A must not touch home B, and vice versa.
	baselineB := testTreeDigest(t, homeB)
	if _, err := Install(homeA, registry, pipelineLightSelection(overlayA), "test-v1", false); err != nil {
		t.Fatalf("Install(A) error = %v", err)
	}
	if got := testTreeDigest(t, homeB); got != baselineB {
		t.Fatalf("installing A mutated B: before=%s after=%s", baselineB, got)
	}
	baselineA := testTreeDigest(t, homeA)
	if _, err := Install(homeB, registry, pipelineLightSelection(overlayB), "test-v1", false); err != nil {
		t.Fatalf("Install(B) error = %v", err)
	}
	if got := testTreeDigest(t, homeA); got != baselineA {
		t.Fatalf("installing B mutated A: before=%s after=%s", baselineA, got)
	}
	gotA, err := os.ReadFile(pipelineCustomSkillPath(homeA, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := os.ReadFile(pipelineCustomSkillPath(homeB, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotA) == string(gotB) {
		t.Fatalf("temporary homes cross-contaminated: both hold %q", gotA)
	}
}

func TestSpec_REQ_ADAPT_001_DeclarationCannotGrantAuthority(t *testing.T) {
	overlayHome := t.TempDir()
	controlHome := t.TempDir()
	registry := newTestRegistry()

	// The declaration asks for tools, permissions, agents, and bindings; the
	// registry surface carries only identity and content.
	authorityDoc := "---\n" +
		"name: " + pipelineCustomSkillID + "\n" +
		"allowed-tools: [bash, webfetch]\n" +
		"permissions: [filesystem:write]\n" +
		"agents: [deploy-agent]\n" +
		"bindings: [role/implement]\n" +
		"---\n\n# " + pipelineCustomSkillID + "\n\nDeclares authority it must never receive.\n"
	overlay := writePipelineOverlay(t, overlayHome, []pipelineCustomSkill{{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "placeholder"}})
	// Overwrite the skill document with the authority-seeking declaration.
	if err := os.WriteFile(filepath.Join(filepath.Dir(overlay.ConfigFile), "skills", pipelineCustomSkillID, "SKILL.md"), []byte(authorityDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(controlHome, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("control Install() error = %v", err)
	}
	if _, err := Install(overlayHome, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("overlay Install() error = %v", err)
	}

	before := pipelineTreeFiles(t, filepath.Join(controlHome, ".config"))
	after := pipelineTreeFiles(t, filepath.Join(overlayHome, ".config"))
	requireOnlyAddedFiles(t, before, after, []string{"opencode/skills/" + pipelineCustomSkillID + "/SKILL.md"})

	// The SKILL.md is data only: byte-exact declaration bytes, no binding.
	written, err := os.ReadFile(pipelineCustomSkillPath(overlayHome, pipelineCustomSkillID))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != authorityDoc {
		t.Fatalf("custom skill bytes were not preserved verbatim: %q", written)
	}
	controlNames := pipelineMCPServerNames(t, controlHome)
	overlayNames := pipelineMCPServerNames(t, overlayHome)
	for name := range overlayNames {
		if !controlNames[name] {
			t.Fatalf("custom declaration granted MCP authority %q", name)
		}
	}
}

// --- REQ-COMPAT-001 ----------------------------------------------------------

func TestSpec_REQ_COMPAT_001_LegacyConfigUnchanged(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"share":"disabled"}`)
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(homeDir, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if config["share"] != "disabled" {
		t.Fatalf("legacy user setting not preserved: %v", config)
	}
	if !pipelineMCPServerNames(t, homeDir)["cortex"] {
		t.Fatal("baseline output missing from legacy install")
	}
	if _, ok := pipelineCommittedReceipt(t, homeDir); ok {
		t.Fatal("legacy no-overlay install triggered registry migration")
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, "overlay")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("legacy install invented overlay configuration: %v", statErr)
	}
}

func TestPipeline_LegacyConfigUnchanged(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	registry := newTestRegistry()

	resultA, err := Install(homeA, registry, pipelineLightSelection(nil), "test-v1", false)
	if err != nil {
		t.Fatalf("Install(A) error = %v; errors: %v", err, resultA.Errors)
	}
	resultB, err := Install(homeB, registry, pipelineLightSelection(nil), "test-v1", false)
	if err != nil {
		t.Fatalf("Install(B) error = %v", err)
	}
	if got, want := pipelineSubtreeDigest(t, filepath.Join(homeA, ".config")), pipelineSubtreeDigest(t, filepath.Join(homeB, ".config")); got != want {
		t.Fatalf("legacy baseline is not deterministic: got=%s want=%s", got, want)
	}
	if resultA.WorkflowFingerprint != "" || resultB.WorkflowFingerprint != "" {
		t.Fatal("legacy no-overlay install produced workflow evidence")
	}
	for _, home := range []string{homeA, homeB} {
		if _, ok := pipelineCommittedReceipt(t, home); ok {
			t.Fatalf("legacy install in %s committed a registry receipt", home)
		}
	}
	lock, err := state.LoadLock(homeA)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Files) == 0 || lock.Version != "test-v1" {
		t.Fatalf("legacy install lost lock evidence: %+v", lock)
	}
}

func TestSpec_REQ_COMPAT_001_OverlayHasBoundedEffect(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	if _, err := Install(homeDir, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("baseline Install() error = %v", err)
	}
	before := pipelineTreeFiles(t, filepath.Join(homeDir, ".config"))

	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("overlay Install() error = %v", err)
	}
	after := pipelineTreeFiles(t, filepath.Join(homeDir, ".config"))
	requireOnlyAddedFiles(t, before, after, []string{"opencode/skills/" + pipelineCustomSkillID + "/SKILL.md"})
}

func TestSpec_REQ_COMPAT_001_OutOfScopeExtensionNotEnabled(t *testing.T) {
	overlayHome := t.TempDir()
	controlHome := t.TempDir()
	registry := newTestRegistry()

	extensionDoc := "---\n" +
		"name: " + pipelineCustomSkillID2 + "\n" +
		"mcp:\n" +
		"  evil-server:\n" +
		"    command: curl\n" +
		"plugins: [everything]\n" +
		"---\n\n# " + pipelineCustomSkillID2 + "\n\nTries to enable an MCP server and plugins.\n"
	overlay := writePipelineOverlay(t, overlayHome, []pipelineCustomSkill{{dir: pipelineCustomSkillID2, name: pipelineCustomSkillID2, body: "placeholder"}})
	if err := os.WriteFile(filepath.Join(filepath.Dir(overlay.ConfigFile), "skills", pipelineCustomSkillID2, "SKILL.md"), []byte(extensionDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(controlHome, registry, pipelineLightSelection(nil), "test-v1", false); err != nil {
		t.Fatalf("control Install() error = %v", err)
	}
	if _, err := Install(overlayHome, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("overlay Install() error = %v", err)
	}

	before := pipelineTreeFiles(t, filepath.Join(controlHome, ".config"))
	after := pipelineTreeFiles(t, filepath.Join(overlayHome, ".config"))
	requireOnlyAddedFiles(t, before, after, []string{"opencode/skills/" + pipelineCustomSkillID2 + "/SKILL.md"})

	controlNames := pipelineMCPServerNames(t, controlHome)
	overlayNames := pipelineMCPServerNames(t, overlayHome)
	for name := range overlayNames {
		if !controlNames[name] {
			t.Fatalf("out-of-scope extension enabled MCP server %q", name)
		}
	}
	if overlayNames["evil-server"] {
		t.Fatal("declared MCP server escaped the unsupported surface")
	}
}

// --- REQ-REM-B1 --------------------------------------------------------------

// loadMetadataDocument reads a state or lock JSON file as a raw document so
// tests can assert persisted registry intent without depending on struct
// field access.
func loadMetadataDocument(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func rewriteMetadataDocument(t *testing.T, path string, doc map[string]json.RawMessage) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// stripRegistrySelectionKey rewrites a metadata file without the persisted
// registry intent, simulating state last written by a legacy binary.
func stripRegistrySelectionKey(t *testing.T, path string) {
	t.Helper()
	doc := loadMetadataDocument(t, path)
	delete(doc, "registry_selection")
	rewriteMetadataDocument(t, path, doc)
}

// setLockRegistrySelection overwrites the lock's persisted registry intent
// with a conflicting declaration.
func setLockRegistrySelection(t *testing.T, homeDir string, value model.RegistrySelection) {
	t.Helper()
	path := state.LockPath(homeDir)
	doc := loadMetadataDocument(t, path)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	doc["registry_selection"] = encoded
	rewriteMetadataDocument(t, path, doc)
}

func requirePersistedRegistrySelection(t *testing.T, path string, want *model.RegistrySelection, context string) {
	t.Helper()
	doc := loadMetadataDocument(t, path)
	node, ok := doc["registry_selection"]
	if !ok {
		t.Fatalf("%s: no registry_selection persisted in %s", context, path)
	}
	var got model.RegistrySelection
	if err := json.Unmarshal(node, &got); err != nil {
		t.Fatalf("%s: decode registry_selection in %s: %v", context, path, err)
	}
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("%s: persisted registry selection = %+v, want %+v", context, got, *want)
	}
}

func assertMetadataLacksRegistrySelection(t *testing.T, path string) {
	t.Helper()
	if doc := loadMetadataDocument(t, path); doc["registry_selection"] != nil {
		t.Fatalf("legacy metadata %s unexpectedly carries registry_selection", path)
	}
}

// TestSpec_REM_B1_RepairReconstructsRegistrySelection covers SC-B1-PERSIST: a
// successful install persists the declared RegistrySelection in both state and
// lock, and Repair reconstructs it semantically unchanged, revalidates
// provenance, and retains the custom outputs and disable intent.
func TestSpec_REM_B1_RepairReconstructsRegistrySelection(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	}, model.ComponentContext7)
	selection := pipelineLightSelection(overlay, model.ComponentCortex, model.ComponentContext7)

	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	requirePersistedRegistrySelection(t, state.StatePath(homeDir), overlay, "state")
	requirePersistedRegistrySelection(t, state.LockPath(homeDir), overlay, "lock")

	customOutput := pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("setup: custom output missing after install: %v", err)
	}

	// Provenance is revalidated through normal preflight: a disappeared
	// configuration file must fail Repair before any mutation.
	configPath := overlay.ConfigFile
	if err := os.Rename(configPath, configPath+".gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := Repair(homeDir, registry, "test-v1", false); err == nil {
		t.Fatal("expected Repair to fail after the verified config file disappeared")
	}
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("failed provenance repair mutated outputs: %v", err)
	}
	if err := os.Rename(configPath+".gone", configPath); err != nil {
		t.Fatal(err)
	}

	// Drift: remove the custom output; Repair must reconstruct it from the
	// persisted intent instead of retiring it as stale.
	if err := os.Remove(customOutput); err != nil {
		t.Fatal(err)
	}
	result, err := Repair(homeDir, registry, "test-v1", false)
	if err != nil {
		t.Fatalf("Repair() error = %v; errors: %v", err, result.Errors)
	}
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("Repair() did not reconstruct the custom output: %v", err)
	}

	// Disable intent is retained: context7 stays disabled, cortex stays.
	names := pipelineMCPServerNames(t, homeDir)
	if names["context7"] {
		t.Fatal("repair re-enabled the disabled optional component context7")
	}
	if !names["cortex"] {
		t.Fatal("repair lost the protected cortex component")
	}

	// The reconstructed intent survives the repair transaction itself.
	requirePersistedRegistrySelection(t, state.StatePath(homeDir), overlay, "state after repair")
	requirePersistedRegistrySelection(t, state.LockPath(homeDir), overlay, "lock after repair")
	if _, ok := pipelineCommittedReceipt(t, homeDir); !ok {
		t.Fatal("repair lost the committed registry receipt")
	}
}

// TestSpec_REM_B1_LegacyReceiptWithoutIntentFailsClosed covers SC-B1-LEGACY-
// FAIL: a committed registry receipt without persisted intent must fail before
// BuildInstallPlan or any mutation and must not retire prior host outputs.
func TestSpec_REM_B1_LegacyReceiptWithoutIntentFailsClosed(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})

	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	// Simulate metadata last written by a legacy binary: strip the intent
	// from both copies while keeping the committed receipt.
	stripRegistrySelectionKey(t, state.StatePath(homeDir))
	stripRegistrySelectionKey(t, state.LockPath(homeDir))
	if _, ok := pipelineCommittedReceipt(t, homeDir); !ok {
		t.Fatal("setup: expected a committed registry receipt")
	}

	customOutput := pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("setup: custom output missing: %v", err)
	}
	before := pipelineSubtreeDigest(t, homeDir)

	_, err := Repair(homeDir, registry, "test-v1", false)
	if err == nil {
		t.Fatal("expected fail-closed error for committed receipt without registry intent")
	}
	if !strings.Contains(err.Error(), "committed registry receipt without persisted registry intent") {
		t.Fatalf("unexpected repair error: %v", err)
	}
	if after := pipelineSubtreeDigest(t, homeDir); after != before {
		t.Fatal("fail-closed repair mutated the home tree")
	}
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("fail-closed repair retired prior host outputs: %v", err)
	}
}

// TestSpec_REM_B1_LegacyMetadataRemainsOverlayFree covers SC-B1-LEGACY-BASE:
// a no-overlay install persists no registry intent and Repair stays on the
// legacy no-overlay path.
func TestSpec_REM_B1_LegacyMetadataRemainsOverlayFree(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	assertMetadataLacksRegistrySelection(t, state.StatePath(homeDir))
	assertMetadataLacksRegistrySelection(t, state.LockPath(homeDir))
	if _, ok := pipelineCommittedReceipt(t, homeDir); ok {
		t.Fatal("no-overlay install committed a registry receipt")
	}
	if _, err := Repair(homeDir, registry, "test-v1", false); err != nil {
		t.Fatalf("legacy Repair() error = %v", err)
	}
}

// TestSpec_REM_B1_SingleMetadataCopyRecovers proves one present intent copy
// recovers the missing one (design D2).
func TestSpec_REM_B1_SingleMetadataCopyRecovers(t *testing.T) {
	for _, strip := range []string{"lock", "state"} {
		t.Run(strip, func(t *testing.T) {
			homeDir := t.TempDir()
			registry := newTestRegistry()
			overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
				{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
			})
			if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
				t.Fatalf("Install() error = %v", err)
			}
			if strip == "lock" {
				stripRegistrySelectionKey(t, state.LockPath(homeDir))
			} else {
				stripRegistrySelectionKey(t, state.StatePath(homeDir))
			}

			customOutput := pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)
			if err := os.Remove(customOutput); err != nil {
				t.Fatal(err)
			}
			result, err := Repair(homeDir, registry, "test-v1", false)
			if err != nil {
				t.Fatalf("Repair() error = %v; errors: %v", err, result.Errors)
			}
			if _, err := os.Stat(customOutput); err != nil {
				t.Fatalf("Repair() did not recover intent after stripping the %s copy: %v", strip, err)
			}
		})
	}
}

// TestSpec_REM_B1_ConflictingMetadataCopiesFailClosed proves disagreeing
// non-nil intent copies fail closed without mutation (design D2).
func TestSpec_REM_B1_ConflictingMetadataCopiesFailClosed(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	overlay := writePipelineOverlay(t, homeDir, []pipelineCustomSkill{
		{dir: pipelineCustomSkillID, name: pipelineCustomSkillID, body: "Deploys the service.\n"},
	})
	if _, err := Install(homeDir, registry, pipelineLightSelection(overlay), "test-v1", false); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	conflicting := *overlay
	conflicting.ConfigFile = filepath.Join(homeDir, "overlay", "other-cortex-ia.yaml")
	setLockRegistrySelection(t, homeDir, conflicting)

	customOutput := pipelineCustomSkillPath(homeDir, pipelineCustomSkillID)
	before := pipelineSubtreeDigest(t, homeDir)
	_, err := Repair(homeDir, registry, "test-v1", false)
	if err == nil {
		t.Fatal("expected fail-closed error for conflicting registry intent copies")
	}
	if !strings.Contains(err.Error(), "disagree on persisted registry intent") {
		t.Fatalf("unexpected repair error: %v", err)
	}
	if after := pipelineSubtreeDigest(t, homeDir); after != before {
		t.Fatal("conflict repair mutated the home tree")
	}
	if _, err := os.Stat(customOutput); err != nil {
		t.Fatalf("conflict repair mutated prior host outputs: %v", err)
	}
}
