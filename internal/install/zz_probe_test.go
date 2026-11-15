//go:build smoke

// Ephemeral transactional smoke for the managed agent-model service wiring.
// It is excluded from the default build so the package's persistent test
// footprint stays unchanged (model-004 leases no test file); run it on demand
// with: go test -tags smoke ./internal/install/ -run '^TestZZSmoke' -count=1
package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
	"github.com/lleontor705/cortex-ia/internal/state"
)

const smokeModel = "anthropic/claude-sonnet-4-5#high"

func smokeInstalledHome(t *testing.T) (string, *Service) {
	t.Helper()
	cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	})
	t.Cleanup(cleanup)
	home := t.TempDir()
	s, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.Install(Options{SkipEnvironment: true, SkipTUIPlugin: true}); err != nil {
		t.Fatalf("install: %v", err)
	}
	return home, s
}

func smokeDesired(t *testing.T, ref string) modelmgr.Desired {
	t.Helper()
	desired, err := modelmgr.ParseDesired("plan", ref, "")
	if err != nil {
		t.Fatalf("parse %q: %v", ref, err)
	}
	return desired
}

func smokeRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func TestZZSmokeUninstalledHomeZeroWrites(t *testing.T) {
	home := t.TempDir()
	s, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("ModelSet on uninstalled home: want ErrNotInstalled, got %v", err)
	}
	if _, err := s.ModelUnset("plan", ModelOptions{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("ModelUnset on uninstalled home: want ErrNotInstalled, got %v", err)
	}
	if _, err := os.Stat(modelmgr.New(home).ConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("uninstalled home wrote a config file: %v", err)
	}
	if _, err := os.Stat(state.StatePath(home)); !os.IsNotExist(err) {
		t.Fatalf("uninstalled home wrote state: %v", err)
	}
}

func TestZZSmokeCommittedSetIsAtomic(t *testing.T) {
	home, s := smokeInstalledHome(t)
	rec, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{})
	if err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	if rec.Action != "set" || rec.Value != smokeModel || rec.Previous != "" || !rec.Changed || !rec.Managed || rec.BackupID == "" {
		t.Fatalf("unexpected receipt: %+v", rec)
	}

	configPath := modelmgr.New(home).ConfigPath()
	config, err := filemerge.DecodeJSONObject(smokeRead(t, configPath))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	agents, _ := config["agents"].(map[string]any)
	plan, _ := agents["plan"].(map[string]any)
	if plan["model"] != smokeModel {
		t.Fatalf("config agents.plan.model = %v", plan["model"])
	}

	metaLoad := state.LoadMetadataV2(home)
	lockLoad := state.LoadLockV2(home)
	if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
		t.Fatalf("state/lock disagree: %v", err)
	}
	if len(metaLoad.Metadata.AgentModels) != 1 || len(lockLoad.Lock.AgentModels) != 1 {
		t.Fatalf("agent model records: meta=%d lock=%d", len(metaLoad.Metadata.AgentModels), len(lockLoad.Lock.AgentModels))
	}
	wantDigest, err := installmeta.AgentModelIdentityDigest(installmeta.AgentModelIdentity{
		Agent: "plan", Provider: "anthropic", Model: "claude-sonnet-4-5", Variant: "high",
	})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	record := metaLoad.Metadata.AgentModels[0]
	if record.Agent != "plan" || record.ConfigPath != "opencode.jsonc" || record.Ownership != state.OwnershipManaged || record.SemanticDigest != wantDigest {
		t.Fatalf("unexpected record: %+v (want digest %s)", record, wantDigest)
	}

	doc, present, err := state.LoadFingerprintDocument(home)
	if err != nil || !present {
		t.Fatalf("fingerprint sidecar: present=%v err=%v", present, err)
	}
	if !doc.HasFingerprintRecord("agent-model/plan") {
		t.Fatalf("missing namespaced sidecar record: %+v", doc.Records)
	}
	for _, sidecar := range doc.Records {
		if sidecar.Name == "agent-model/plan" && sidecar.PostImageDigest != wantDigest {
			t.Fatalf("sidecar digest = %s, want %s", sidecar.PostImageDigest, wantDigest)
		}
	}

	list, err := s.ModelList()
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if !list.Installed {
		t.Fatal("ModelList should report installed")
	}
	found := false
	for _, entry := range list.Agents {
		if entry.Agent != "plan" {
			continue
		}
		found = true
		if !entry.Managed || entry.Source != modelmgr.SourceManaged || entry.Model != "anthropic/claude-sonnet-4-5" || entry.Variant != "high" {
			t.Fatalf("unexpected list entry: %+v", entry)
		}
	}
	if !found {
		t.Fatal("ModelList did not report plan")
	}
	get, err := s.ModelGet("plan")
	if err != nil {
		t.Fatalf("ModelGet: %v", err)
	}
	if !get.Agent.Managed || get.Agent.Source != modelmgr.SourceManaged {
		t.Fatalf("unexpected get: %+v", get.Agent)
	}

	// Overwrite discloses the previous effective value.
	second, err := s.ModelSet(smokeDesired(t, "openai/gpt-5.2"), ModelOptions{})
	if err != nil {
		t.Fatalf("second ModelSet: %v", err)
	}
	if second.Previous != smokeModel || second.Value != "openai/gpt-5.2" {
		t.Fatalf("overwrite receipt: %+v", second)
	}
}

func TestZZSmokeDryRunNeverLocksOrWrites(t *testing.T) {
	home, s := smokeInstalledHome(t)
	configPath := modelmgr.New(home).ConfigPath()
	before := smokeRead(t, configPath)
	beforeInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	backupsDir := filepath.Join(home, ".cortex-ia", "backups")
	backupsBefore, _ := os.ReadDir(backupsDir)

	release, err := s.lockForMutation(0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	rec, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{DryRun: true, LockTimeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("dry-run while the home lock is held: %v", err)
	}
	release()
	if rec.Action != "set" || rec.Value != smokeModel || !rec.DryRun {
		t.Fatalf("unexpected dry-run receipt: %+v", rec)
	}
	if !bytes.Equal(before, smokeRead(t, configPath)) {
		t.Fatal("dry-run changed config bytes")
	}
	afterInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Fatal("dry-run changed config mtime")
	}
	backupsAfter, _ := os.ReadDir(backupsDir)
	if len(backupsAfter) != len(backupsBefore) {
		t.Fatalf("dry-run created backups: %d -> %d", len(backupsBefore), len(backupsAfter))
	}
	if _, err := os.Stat(state.StatePath(home)); err != nil {
		t.Fatalf("dry-run removed state: %v", err)
	}
}

func TestZZSmokeFailureRestoresPreimages(t *testing.T) {
	home, s := smokeInstalledHome(t)
	if _, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{}); err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	configPath := modelmgr.New(home).ConfigPath()
	configBefore := smokeRead(t, configPath)
	stateBefore := smokeRead(t, state.StatePath(home))
	lockBefore := smokeRead(t, state.LockPath(home))
	sidecarBefore := smokeRead(t, state.FingerprintPath(home))

	original := commitLockV2
	commitLockV2 = func(string, state.LockV2) error { return errors.New("injected lock commit failure") }
	defer func() { commitLockV2 = original }()

	rec, err := s.ModelSet(smokeDesired(t, "openai/gpt-5.2"), ModelOptions{})
	commitLockV2 = original
	if err == nil {
		t.Fatal("expected the injected commit failure")
	}
	if !rec.Restored || rec.RestoreError != "" {
		t.Fatalf("restore verdict: restored=%v err=%q", rec.Restored, rec.RestoreError)
	}
	if !bytes.Equal(configBefore, smokeRead(t, configPath)) {
		t.Fatal("config preimage was not restored")
	}
	if !bytes.Equal(stateBefore, smokeRead(t, state.StatePath(home))) {
		t.Fatal("state preimage was not restored")
	}
	if !bytes.Equal(lockBefore, smokeRead(t, state.LockPath(home))) {
		t.Fatal("lock preimage was not restored")
	}
	if !bytes.Equal(sidecarBefore, smokeRead(t, state.FingerprintPath(home))) {
		t.Fatal("fingerprint preimage was not restored")
	}
}

func TestZZSmokeUnsetPrunesRecords(t *testing.T) {
	home, s := smokeInstalledHome(t)
	if _, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{}); err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	rec, err := s.ModelUnset("PLAN", ModelOptions{})
	if err != nil {
		t.Fatalf("ModelUnset: %v", err)
	}
	if rec.Action != "unset" || rec.Agent != "plan" || rec.Previous != smokeModel || rec.Value != "" {
		t.Fatalf("unexpected unset receipt: %+v", rec)
	}
	config, err := filemerge.DecodeJSONObject(smokeRead(t, modelmgr.New(home).ConfigPath()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if agents, ok := config["agents"].(map[string]any); ok {
		if entry, present := agents["plan"].(map[string]any); present {
			if _, hasModel := entry["model"]; hasModel {
				t.Fatalf("model member survived unset: %+v", entry)
			}
		}
	}
	metaLoad := state.LoadMetadataV2(home)
	if err := state.CheckAgreementV2(metaLoad.Metadata, state.LoadLockV2(home).Lock); err != nil {
		t.Fatalf("state/lock disagree after unset: %v", err)
	}
	if len(metaLoad.Metadata.AgentModels) != 0 {
		t.Fatalf("ownership record survived unset: %+v", metaLoad.Metadata.AgentModels)
	}
	doc, present, err := state.LoadFingerprintDocument(home)
	if err != nil {
		t.Fatalf("load sidecar: %v", err)
	}
	if present && doc.HasFingerprintRecord("agent-model/plan") {
		t.Fatal("sidecar record survived unset")
	}
}

func TestZZSmokeUninstallAndRollbackEnumerateModels(t *testing.T) {
	home, s := smokeInstalledHome(t)
	configPath := modelmgr.New(home).ConfigPath()
	pristine := smokeRead(t, configPath)

	rec, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{})
	if err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	afterSet := smokeRead(t, configPath)

	dry, err := s.Uninstall(UninstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry uninstall: %v", err)
	}
	if len(dry.ModelRemoved) != 1 || dry.ModelRemoved[0] != "plan" {
		t.Fatalf("dry uninstall model enumeration: %+v", dry.ModelRemoved)
	}
	if !bytes.Equal(afterSet, smokeRead(t, configPath)) {
		t.Fatal("dry uninstall wrote config")
	}

	rb, err := s.Rollback(rec.BackupID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !rb.Verified {
		t.Fatal("rollback not verified")
	}
	if !bytes.Equal(pristine, smokeRead(t, configPath)) {
		t.Fatal("rollback did not restore the pre-set config")
	}
	if models := state.LoadMetadataV2(home).Metadata.AgentModels; len(models) != 0 {
		t.Fatalf("rollback left ownership records: %+v", models)
	}

	// A fresh home proves the real uninstall removes the managed entry.
	home2, s2 := smokeInstalledHome(t)
	if _, err := s2.ModelSet(smokeDesired(t, smokeModel), ModelOptions{}); err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	full, err := s2.Uninstall(UninstallOptions{})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(full.ModelRemoved) != 1 || full.ModelRemoved[0] != "plan" {
		t.Fatalf("uninstall model enumeration: %+v", full.ModelRemoved)
	}
	if !full.Complete || !full.StateRemoved {
		t.Fatalf("uninstall not complete: %+v", full)
	}
	if _, err := os.Stat(state.StatePath(home2)); !os.IsNotExist(err) {
		t.Fatalf("state survived uninstall: %v", err)
	}
}

func TestZZSmokeRollbackDetectsModelDrift(t *testing.T) {
	home, s := smokeInstalledHome(t)
	rec, err := s.ModelSet(smokeDesired(t, smokeModel), ModelOptions{})
	if err != nil {
		t.Fatalf("ModelSet: %v", err)
	}
	configPath := modelmgr.New(home).ConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	drifted := bytes.Replace(raw, []byte("claude-sonnet-4-5"), []byte("claude-opus-4-1"), 1)
	if bytes.Equal(drifted, raw) {
		t.Fatal("drift replacement did not apply")
	}
	if err := os.WriteFile(configPath, drifted, 0o644); err != nil {
		t.Fatalf("write drift: %v", err)
	}
	if _, err := s.Rollback(rec.BackupID); !errors.Is(err, ErrRollbackDrift) {
		t.Fatalf("want ErrRollbackDrift, got %v", err)
	}
	if !bytes.Equal(drifted, smokeRead(t, configPath)) {
		t.Fatal("drift rollback mutated the config")
	}
}
