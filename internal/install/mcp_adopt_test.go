package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func adoptInstalledHome(t *testing.T) (string, *Service) {
	t.Helper()
	cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	})
	t.Cleanup(cleanup)
	setV2Detector(t)

	home := t.TempDir()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := service.Install(Options{SkipEnvironment: true, SkipTUIPlugin: true, Cortex: true}); err != nil {
		t.Fatalf("install: %v", err)
	}
	// Reproduce the live defect: cortex stays selected and the entry stays in
	// the config, but its ownership record is dropped, leaving an equal,
	// unaccredited entry that blocks planning until it is adopted.
	meta := state.LoadMetadataV2(home).Metadata
	kept := make([]state.MCPV2, 0, len(meta.MCPs))
	for _, record := range meta.MCPs {
		if record.Name != "cortex" {
			kept = append(kept, record)
		}
	}
	meta.MCPs = kept
	if err := state.SaveMetadataV2(home, meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	if err := state.SaveLockV2(home, state.NewLockFromMetadataV2(meta)); err != nil {
		t.Fatalf("save lock: %v", err)
	}
	return home, service
}

func adoptCortexEntry(t *testing.T) map[string]any {
	t.Helper()
	preset, ok := mcpmanager.Lookup("cortex")
	if !ok {
		t.Fatal("cortex preset is missing from the catalog")
	}
	return preset.Entry
}

func adoptWriteEntry(t *testing.T, home string, entry map[string]any) string {
	t.Helper()
	path := mcpmanager.New(home).ConfigPath()
	config := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		decoded, err := filemerge.DecodeJSONObject(raw)
		if err != nil {
			t.Fatalf("decode existing config: %v", err)
		}
		config = decoded
	}
	mcp, _ := config["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	servers, _ := mcp["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["cortex"] = entry
	mcp["servers"] = servers
	config["mcp"] = mcp
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func adoptRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func planHasMCPConflict(plan *pipeline.Plan, target string) bool {
	for _, conflict := range plan.Conflicts {
		if conflict.Target == target && conflict.Kind == pipeline.ConflictMCP {
			return true
		}
	}
	return false
}

func TestMCPAdoptAccreditsInPlaceWithoutConfigWrite(t *testing.T) {
	home, service := adoptInstalledHome(t)
	path := adoptWriteEntry(t, home, adoptCortexEntry(t))
	before := adoptRead(t, path)

	planBefore, err := service.Plan(Options{Cortex: true})
	if err != nil {
		t.Fatalf("plan before adopt: %v", err)
	}
	if !planHasMCPConflict(planBefore, "cortex") {
		t.Fatalf("expected the reproduced mcp-conflict before adopt, got %+v", planBefore.Conflicts)
	}
	doctorBefore, err := service.Doctor()
	if err != nil {
		t.Fatalf("doctor before adopt: %v", err)
	}
	if !findingContains(doctorBefore.Findings, "cortex-ia mcp adopt cortex") {
		t.Fatalf("doctor did not surface the adopt remedy: %v", doctorBefore.Findings)
	}
	receipt, err := service.MCPAdopt("cortex", MCPOptions{})
	if err != nil {
		t.Fatalf("MCPAdopt: %v", err)
	}
	if receipt.Action != "adopted" || !receipt.Configured || receipt.Changed || receipt.BackupID == "" {
		t.Fatalf("unexpected adopt receipt: %+v", receipt)
	}
	if !bytes.Equal(before, adoptRead(t, path)) {
		t.Fatal("adopt rewrote the OpenCode config")
	}

	meta := state.LoadMetadataV2(home).Metadata
	if len(meta.MCPs) != 1 || meta.MCPs[0].Name != "cortex" || meta.MCPs[0].Ownership != state.OwnershipManaged {
		t.Fatalf("adopt did not record managed ownership: %+v", meta.MCPs)
	}
	if meta.MCPs[0].SemanticDigest == "" || meta.MCPs[0].ConfigPath == "" {
		t.Fatalf("incomplete ownership record: %+v", meta.MCPs[0])
	}
	doc, present, err := state.LoadFingerprintDocument(home)
	if err != nil || !present {
		t.Fatalf("fingerprint sidecar: present=%v err=%v", present, err)
	}
	if !doc.HasFingerprintRecord("cortex") {
		t.Fatalf("adopt did not record a postimage fingerprint: %+v", doc.Records)
	}

	planAfter, err := service.Plan(Options{Cortex: true})
	if err != nil {
		t.Fatalf("plan after adopt: %v", err)
	}
	if planHasMCPConflict(planAfter, "cortex") {
		t.Fatalf("mcp-conflict survived adopt: %+v", planAfter.Conflicts)
	}
	doctorAfter, err := service.Doctor()
	if err != nil {
		t.Fatalf("doctor after adopt: %v", err)
	}
	if findingContains(doctorAfter.Findings, "mcp adopt cortex") {
		t.Fatalf("doctor still suggests adopt after accreditation: %v", doctorAfter.Findings)
	}

	again, err := service.MCPAdopt("cortex", MCPOptions{})
	if err != nil {
		t.Fatalf("re-adopt: %v", err)
	}
	if again.Action != "already-present" || again.BackupID != "" {
		t.Fatalf("re-adopt was not an idempotent no-op: %+v", again)
	}
}

// TestMCPAdoptAccreditsEnvNameDivergentEquivalentEntry covers the defect where
// semantic equality tolerates env/header NAME-set divergence while the identity
// digest pins those names: adoption must persist the OBSERVED identity so the
// remediation sticks instead of leaving the entry permanently unaccredited.
func TestMCPAdoptAccreditsEnvNameDivergentEquivalentEntry(t *testing.T) {
	home, service := adoptInstalledHome(t)
	entry := adoptCortexEntry(t)
	entry["env"] = map[string]any{"CORTEX_USER_TOKEN": "placeholder"}
	entry["headers"] = map[string]any{"X-Cortex-Session": "placeholder"}
	path := adoptWriteEntry(t, home, entry)
	before := adoptRead(t, path)

	planBefore, err := service.Plan(Options{Cortex: true})
	if err != nil {
		t.Fatalf("plan before adopt: %v", err)
	}
	if !planHasMCPConflict(planBefore, "cortex") {
		t.Fatalf("the name-divergent entry was not treated as an unmanaged-equivalent conflict: %+v", planBefore.Conflicts)
	}

	receipt, err := service.MCPAdopt("cortex", MCPOptions{})
	if err != nil {
		t.Fatalf("MCPAdopt: %v", err)
	}
	if receipt.Action != "adopted" || !receipt.Configured || receipt.Changed {
		t.Fatalf("unexpected adopt receipt: %+v", receipt)
	}
	if !bytes.Equal(before, adoptRead(t, path)) {
		t.Fatal("adopt rewrote the OpenCode config")
	}

	planAfter, err := service.Plan(Options{Cortex: true})
	if err != nil {
		t.Fatalf("plan after adopt: %v", err)
	}
	if planHasMCPConflict(planAfter, "cortex") {
		t.Fatalf("mcp-conflict survived adopt of a name-divergent equivalent entry: %+v", planAfter.Conflicts)
	}
	doctorAfter, err := service.Doctor()
	if err != nil {
		t.Fatalf("doctor after adopt: %v", err)
	}
	if findingContains(doctorAfter.Findings, "not ownership-accredited") {
		t.Fatalf("doctor still reports the entry as unaccredited after adopt: %v", doctorAfter.Findings)
	}
	if findingContains(doctorAfter.Findings, "mcp adopt cortex") {
		t.Fatalf("doctor still suggests adopt after accreditation: %v", doctorAfter.Findings)
	}
}

func TestMCPAdoptDryRunWritesNothing(t *testing.T) {
	home, service := adoptInstalledHome(t)
	path := adoptWriteEntry(t, home, adoptCortexEntry(t))
	configBefore := adoptRead(t, path)
	stateBefore := adoptRead(t, state.StatePath(home))

	receipt, err := service.MCPAdopt("cortex", MCPOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run adopt: %v", err)
	}
	if receipt.Action != "adopted" || !receipt.DryRun || receipt.BackupID != "" {
		t.Fatalf("unexpected dry-run receipt: %+v", receipt)
	}
	if !bytes.Equal(configBefore, adoptRead(t, path)) {
		t.Fatal("dry-run adopt changed the config")
	}
	if !bytes.Equal(stateBefore, adoptRead(t, state.StatePath(home))) {
		t.Fatal("dry-run adopt changed the state")
	}
	plan, err := service.Plan(Options{Cortex: true})
	if err != nil {
		t.Fatalf("plan after dry-run: %v", err)
	}
	if !planHasMCPConflict(plan, "cortex") {
		t.Fatal("dry-run adopt accredited the entry")
	}
}

func TestMCPAdoptFailsClosedOnDifferentEntry(t *testing.T) {
	home, service := adoptInstalledHome(t)
	path := adoptWriteEntry(t, home, map[string]any{
		"type":    "local",
		"command": []any{"different-binary", "serve"},
	})
	before := adoptRead(t, path)

	_, err := service.MCPAdopt("cortex", MCPOptions{})
	var conflict *mcpmanager.ConflictError
	if !errors.As(err, &conflict) || conflict.Kind != mcpmanager.ConflictModified {
		t.Fatalf("want ConflictModified, got %v", err)
	}
	if !bytes.Equal(before, adoptRead(t, path)) {
		t.Fatal("a failed adopt mutated the config")
	}
	if mcpRecords := state.LoadMetadataV2(home).Metadata.MCPs; len(mcpRecords) != 0 {
		t.Fatalf("failed adopt recorded ownership: %+v", mcpRecords)
	}
	report, err := service.Doctor()
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if findingContains(report.Findings, "mcp adopt") {
		t.Fatalf("doctor suggested adopt for a modified entry: %v", report.Findings)
	}
}

func TestMCPAdoptRequiresInstalledHomeAndKnownName(t *testing.T) {
	home := t.TempDir()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := service.MCPAdopt("cortex", MCPOptions{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("want ErrNotInstalled, got %v", err)
	}
	_, err = service.MCPAdopt("not-a-preset", MCPOptions{})
	var conflict *mcpmanager.ConflictError
	if !errors.As(err, &conflict) || conflict.Kind != mcpmanager.ConflictUnmanaged {
		t.Fatalf("want ConflictUnmanaged, got %v", err)
	}
}
