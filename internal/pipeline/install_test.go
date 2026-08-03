package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func newTestRegistry() *agents.Registry {
	r := agents.NewRegistry()
	r.Register(codex.NewAdapter())
	r.Register(opencode.NewAdapter())
	return r
}

func explicitTestModelAssignments(providerModel string) model.ModelAssignments {
	assignments := model.ModelAssignments{}
	for _, phase := range []string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize", "orchestrator"} {
		assignments[phase] = providerModel
	}
	return assignments
}

func explicitTestProfile(name, providerModel string) model.Profile {
	assignments := model.OpenCodeModelAssignments{}
	for _, phase := range []string{"sdd-init", "sdd-explore", "sdd-propose", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive"} {
		assignments[phase] = model.OpenCodeModelAssignment{Provider: strings.SplitN(providerModel, "/", 2)[0], Model: strings.SplitN(providerModel, "/", 2)[1]}
	}
	return model.Profile{Name: name, ConfiguredAssignments: assignments}
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_Full(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	selection := model.Selection{
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetFull,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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

func TestInstallDryRunComposesProbedWorkflowWithoutMutation(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := qualifiedForgeSpecSnapshot(now)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/capabilities" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(snapshot)
	}))
	defer server.Close()
	t.Setenv("CORTEX_IA_FORGESPEC_CAPABILITIES_URL", server.URL)
	// Keep the mutation sentinel outside the managed workflow target. Placing
	// operator bytes in AGENTS.md correctly creates an ownership blocker and is
	// not a valid healthy-doctor fixture.
	sentinel := filepath.Join(homeDir, ".codex", "operator-sentinel.txt")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("operator content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := testTreeDigest(t, homeDir)
	result, err := Install(homeDir, registry, model.Selection{
		Agents:           []model.AgentID{model.AgentCodex},
		Components:       []model.ComponentID{model.ComponentSDD},
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
	}, "test-v1", true)
	if err != nil {
		t.Fatalf("Install() dry-run error = %v", err)
	}
	after := testTreeDigest(t, homeDir)
	if before != after {
		t.Fatalf("dry-run mutated target tree: before=%s after=%s", before, after)
	}

	if result.WorkflowCutover.Mode != forgespec.CoordinationDirectV1 {
		t.Fatalf("workflow mode = %q, want probed direct-v1", result.WorkflowCutover.Mode)
	}
	if result.WorkflowDoctor.Profile == "" || !result.WorkflowDoctor.Qualified {
		t.Fatalf("workflow doctor = %+v, want qualified production report", result.WorkflowDoctor)
	}
	if result.WorkflowFingerprint == "" || result.WorkflowFingerprint != result.WorkflowPlan.Fingerprint {
		t.Fatalf("workflow fingerprints = result %q plan %q", result.WorkflowFingerprint, result.WorkflowPlan.Fingerprint)
	}
	if result.WorkflowReceipt.ID != "" || result.WorkflowRollback {
		t.Fatalf("dry-run produced mutation evidence: receipt=%+v rollback=%t", result.WorkflowReceipt, result.WorkflowRollback)
	}
}

func TestPreparedWorkflowCreateUsesStrictAbsentTargetCAS(t *testing.T) {
	homeDir := t.TempDir()
	adapter, err := newTestRegistry().Get(model.AgentCodex)
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

func qualifiedForgeSpecSnapshot(now time.Time) forgespec.CapabilitySnapshot {
	requirements := forgespec.RequiredP0Capabilities()
	capabilities := make([]forgespec.NegotiatedCapability, 0, len(requirements))
	for _, requirement := range requirements {
		capabilities = append(capabilities, forgespec.NegotiatedCapability{
			ID: requirement.ID, Version: ir.MustParseVersion("1.0.0"), Provider: "forgespec",
			ProviderVersion: ir.MustParseVersion("2.0.0"), Interval: requirement.Versions,
			EvidenceClass: capability.EvidenceExecutableProbe, EvidenceRef: "probe://forgespec/capabilities",
			ObservedAt: now.Add(-time.Minute), FreshUntil: now.Add(time.Hour), Confidence: 1,
			ProbeID: "probe/forgespec/capabilities", Enforcement: capability.EnforcementMCP,
		})
	}
	return forgespec.CapabilitySnapshot{
		SchemaVersion: ir.MustParseVersion("1.0.0"), ServerVersion: ir.MustParseVersion("2.0.0"),
		ProtocolVersion: ir.MustParseVersion("1.0.0"), ProbeStatus: forgespec.ProbeQualified,
		Capabilities: capabilities,
	}
}

func TestPrepareWorkflowBuildsOneDeterministicHomeRelativePlan(t *testing.T) {
	homeDir := t.TempDir()
	adapter, err := newTestRegistry().Get(model.AgentCodex)
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
		if !strings.HasPrefix(filepath.ToSlash(effect.Path), ".codex/") {
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

func TestInstallDryRunExposesCanonicalWorkflowFingerprintWithoutMutation(t *testing.T) {
	homeDir := t.TempDir()
	before := testTreeDigest(t, homeDir)
	result, err := Install(homeDir, newTestRegistry(), model.Selection{
		Agents: []model.AgentID{model.AgentCodex}, Components: []model.ComponentID{model.ComponentSDD}, ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
	}, "test-v1", true)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.WorkflowFingerprint == "" {
		t.Fatal("shipped pipeline did not expose canonical workflow fingerprint")
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
		Agents:           []model.AgentID{model.AgentCodex, "nonexistent-agent"},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Components:       []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
	profiles := []model.Profile{explicitTestProfile("premium", "provider-test/model-test")}
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

// TestInstall_ProfileAutoAppliesToOpenCodeJSON verifies that when an OpenCode
// adapter is in the selection AND a profile is resolved, the per-phase model
// assignments are written to opencode.json without requiring `profiles apply`.
func TestInstall_ProfileAutoAppliesToOpenCodeJSON(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	cheap := explicitTestProfile("cheap", "openai/gpt-4o-mini")
	cheap.ConfiguredAssignments["sdd-apply"] = model.OpenCodeModelAssignment{Provider: "provider-test", Model: "model-test"}
	profiles := []model.Profile{cheap}
	if err := state.SaveProfiles(homeDir, profiles); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	selection := model.Selection{
		Agents:      []model.AgentID{model.AgentOpenCode},
		Preset:      model.PresetFull,
		ProfileName: "cheap",
	}
	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	cfgPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile opencode.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, string(data))
	}
	agentSection, ok := parsed["agent"].(map[string]any)
	if !ok {
		t.Fatalf("agent section missing in opencode.json: %s", string(data))
	}
	design, ok := agentSection["architect"].(map[string]any)
	if !ok {
		t.Fatalf("architect entry missing: %v", agentSection)
	}
	if design["model"] != "openai/gpt-4o-mini" {
		t.Errorf("architect.model = %v, want openai/gpt-4o-mini", design["model"])
	}
	if _, exists := agentSection["team-lead"]; exists {
		t.Fatal("portable install must not create a team-lead entry")
	}
	worker, ok := agentSection["implement"].(map[string]any)
	if !ok {
		t.Fatalf("implement entry missing")
	}
	if worker["model"] != "provider-test/model-test" {
		t.Errorf("implement.model = %v", worker["model"])
	}
	if _, hasLegacy := agentSection["sdd-apply"]; hasLegacy {
		t.Error("profile auto-apply should not create legacy sdd-apply entry")
	}
}

func TestInstall_ModelAssignmentsAutoApplyToOpenCodeJSON(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	selection := model.Selection{
		Agents:           []model.AgentID{model.AgentOpenCode},
		Preset:           model.PresetFull,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
	}
	selection.ModelAssignments["architect"] = "provider-test/architect-model"
	selection.ModelAssignments["implement"] = "provider-test/implement-model"
	if _, err := Install(homeDir, registry, selection, "test-v1", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	cfgPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile opencode.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, string(data))
	}
	agentSection, ok := parsed["agent"].(map[string]any)
	if !ok {
		t.Fatalf("agent section missing in opencode.json: %s", string(data))
	}
	assertAgentModel := func(agent, want string) {
		t.Helper()
		entry, _ := agentSection[agent].(map[string]any)
		if entry == nil {
			t.Fatalf("%s entry missing", agent)
		}
		if entry["model"] != want {
			t.Errorf("%s.model = %v, want %s", agent, entry["model"], want)
		}
	}
	assertAgentModel("architect", "provider-test/architect-model")
	assertAgentModel("implement", "provider-test/implement-model")
	assertAgentModel("orchestrator", "provider-test/model-test")
}

func TestInstall_ProfileNameDoesNotOverrideExplicitAssignments(t *testing.T) {
	homeDir := t.TempDir()
	registry := newTestRegistry()

	// Create a profile.
	profiles := []model.Profile{explicitTestProfile("economy", "provider-test/profile-model")}
	if err := state.SaveProfiles(homeDir, profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}

	// Selection already has explicit ModelAssignments — profile should NOT override.
	explicit := explicitTestModelAssignments("provider-test/explicit-model")
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
		Agents:           []model.AgentID{model.AgentCodex},
		Preset:           model.PresetMinimal,
		ModelAssignments: explicitTestModelAssignments("provider-test/model-test"),
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
