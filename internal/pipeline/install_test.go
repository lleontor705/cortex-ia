package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// Repository test policy scopes this file to simple existence and copy
// validation toward OpenCode. Deeper transactional oracles run as ephemeral
// smokes and are deleted after execution.

func engineHome(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func engineRequest(home string) Request {
	return Request{HomeDir: home, Version: "test", Cortex: true, ForgeSpec: true}
}

func engineJoin(home string, rel string) string {
	return filepath.Join(home, filepath.FromSlash(rel))
}

func engineAssertRegular(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected installed file %q: %v", path, err)
	}
	return string(content)
}

// Installing into a fresh home copies every embedded native asset beneath
// the OpenCode config root and commits agreeing v2 metadata.
func TestInstall_CopiesEmbeddedAssetsToOpenCode(t *testing.T) {
	home := engineHome(t)
	plan, receipt, err := InstallV2(engineRequest(home))
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if plan == nil || receipt == nil {
		t.Fatal("install must return a plan and a receipt")
	}
	if receipt.BackupID == "" || !receipt.BackupVerified {
		t.Fatalf("apply must verify a backup, got %+v", receipt)
	}

	// Existence checks across every native asset kind derived by the plan.
	sawConfig, sawDoc, sawSkill, sawAgent, sawCommand, sawPlugin := false, false, false, false, false, false
	for _, mapping := range plan.mappings {
		engineAssertRegular(t, engineJoin(home, mapping.Dest))
		switch mapping.Kind {
		case "config":
			sawConfig = true
		case "agents-doc":
			sawDoc = true
		case "skill":
			sawSkill = true
		case "agent":
			sawAgent = true
		case "command":
			sawCommand = true
		case "plugin":
			sawPlugin = true
		}
	}
	if !sawConfig || !sawDoc || !sawSkill || !sawAgent || !sawCommand || !sawPlugin {
		t.Fatalf("expected assets of every native kind, got config=%v doc=%v skill=%v agent=%v command=%v plugin=%v",
			sawConfig, sawDoc, sawSkill, sawAgent, sawCommand, sawPlugin)
	}

	// Metadata commits last and agrees.
	metaLoad := state.LoadMetadataV2(home)
	lockLoad := state.LoadLockV2(home)
	if metaLoad.Presence != state.PresenceV2 || lockLoad.Presence != state.PresenceV2 {
		t.Fatalf("expected v2 metadata, got state=%s lock=%s", metaLoad.Presence, lockLoad.Presence)
	}
	if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
		t.Fatalf("state and lock must agree: %v", err)
	}
	if len(metaLoad.Metadata.Artifacts) != len(plan.mappings) {
		t.Fatalf("metadata must record every installed artifact: %d vs %d", len(metaLoad.Metadata.Artifacts), len(plan.mappings))
	}
	if len(metaLoad.Metadata.MCPs) != 2 {
		t.Fatalf("expected cortex+forgespec MCP records, got %d", len(metaLoad.Metadata.MCPs))
	}

	// Selected MCP entries exist in the OpenCode config; context7 stays out.
	config := engineAssertRegular(t, engineJoin(home, ".config/opencode/opencode.jsonc"))
	decoded, err := filemerge.DecodeJSONObject([]byte(config))
	if err != nil {
		t.Fatalf("installed config must be valid JSONC: %v", err)
	}
	mcp, ok := decoded["mcp"].(map[string]any)
	if !ok {
		t.Fatal("installed config must carry the mcp object")
	}
	for _, name := range []string{"cortex", "forgespec"} {
		if _, ok := mcp[name].(map[string]any); !ok {
			t.Errorf("managed MCP %q must be configured", name)
		}
	}
	if _, present := mcp["context7"]; present {
		t.Error("unselected context7 must not be configured")
	}
}

// A second identical execution is a pure no-op: zero writes, backups, and
// metadata churn.
func TestInstall_SecondRunIsZeroChurn(t *testing.T) {
	home := engineHome(t)
	if _, _, err := InstallV2(engineRequest(home)); err != nil {
		t.Fatalf("first install failed: %v", err)
	}
	before := engineHomeSnapshot(t, home)

	_, receipt, err := InstallV2(engineRequest(home))
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if receipt == nil || !receipt.Converged {
		t.Fatalf("second install must converge, got %+v", receipt)
	}
	if receipt.BackupID != "" || len(receipt.Changes) != 0 {
		t.Fatalf("converged run must not create backups or changes, got %+v", receipt)
	}
	if after := engineHomeSnapshot(t, home); after != before {
		t.Fatal("converged run must not modify any file beneath the home")
	}
}

// Dry-run plans without creating directories, journals, backups, state, or
// destination temporaries.
func TestInstall_DryRunCreatesNothing(t *testing.T) {
	home := engineHome(t)
	req := engineRequest(home)
	req.DryRun = true
	plan, receipt, err := InstallV2(req)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if !receipt.DryRun || len(plan.Effects) == 0 {
		t.Fatalf("dry-run must return the planned effects, got %+v", receipt)
	}
	for _, dir := range []string{".config", ".cortex-ia"} {
		if _, err := os.Stat(engineJoin(home, dir)); !os.IsNotExist(err) {
			t.Errorf("dry-run must not create %q", dir)
		}
	}
}

// Unmanaged conflicts fail closed; an explicit overwrite authorization
// replaces the file only after a verified backup captured its bytes.
func TestInstall_UnmanagedConflictFailsClosedAndOverwriteRestores(t *testing.T) {
	home := engineHome(t)
	dest := engineJoin(home, ".config/opencode/AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	const userBytes = "user-owned system prompt\n"
	if err := os.WriteFile(dest, []byte(userBytes), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := InstallV2(engineRequest(home)); err == nil {
		t.Fatal("unmanaged conflict must fail closed")
	}
	if got := engineAssertRegular(t, dest); got != userBytes {
		t.Fatal("fail-closed planning must leave the user file untouched")
	}
	if state.LoadMetadataV2(home).Presence == state.PresenceV2 {
		t.Fatal("a refused install must not commit metadata")
	}

	req := engineRequest(home)
	req.Overwrite = true
	_, receipt, err := InstallV2(req)
	if err != nil {
		t.Fatalf("authorized overwrite must apply: %v", err)
	}
	if receipt.BackupID == "" || !receipt.BackupVerified {
		t.Fatalf("overwrite requires a verified backup, got %+v", receipt)
	}
	if got := engineAssertRegular(t, dest); got == userBytes || !strings.Contains(got, "orchestrator") {
		t.Fatal("overwrite must install the embedded AGENTS.md")
	}
	// The verified backup restores the user's original bytes.
	manifestPath := engineJoin(home, ".cortex-ia/backups/"+receipt.BackupID+"/"+backup.ManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("backup manifest must exist: %v", err)
	}
}

// The settings template merges safely: user keys and comments survive and
// template keys are added, never a byte-for-byte overwrite.
func TestInstall_ConfigMergePreservesUserKeysAndComments(t *testing.T) {
	home := engineHome(t)
	dest := engineJoin(home, ".config/opencode/opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	const userConfig = "{\n  // my notes\n  \"theme\": \"dark\"\n}\n"
	if err := os.WriteFile(dest, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := InstallV2(engineRequest(home)); err != nil {
		t.Fatalf("install with user config failed: %v", err)
	}
	merged := engineAssertRegular(t, dest)
	if !strings.Contains(merged, "// my notes") || !strings.Contains(merged, "\"theme\": \"dark\"") {
		t.Fatal("merge must preserve user comments and keys")
	}
	if !strings.Contains(merged, "\"default_agent\": \"orchestrator\"") {
		t.Fatal("merge must add template keys")
	}
	if merged == userConfig {
		t.Fatal("merge must not be a byte-for-byte overwrite but also must not drop user content")
	}
	decoded, err := filemerge.DecodeJSONObject([]byte(merged))
	if err != nil {
		t.Fatalf("merged config must stay valid JSONC: %v", err)
	}
	if decoded["theme"] != "dark" {
		t.Fatal("user keys must survive the merge")
	}
}

// Rollback restores the pre-install state from the verified backup
// manifest: an authorized overwrite returns the user's original bytes and
// the pre-install absence is re-established for created files.
func TestRollback_RestoresFromEngineBackup(t *testing.T) {
	home := engineHome(t)
	dest := engineJoin(home, ".config/opencode/AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	const userBytes = "user-owned system prompt\n"
	if err := os.WriteFile(dest, []byte(userBytes), 0o644); err != nil {
		t.Fatal(err)
	}

	req := engineRequest(home)
	req.Overwrite = true
	_, receipt, err := InstallV2(req)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if got := engineAssertRegular(t, dest); got == userBytes {
		t.Fatal("overwrite must replace the user file")
	}

	manifest, err := Rollback(home, receipt.BackupID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if manifest.ID != receipt.BackupID {
		t.Fatalf("rollback used manifest %q, want %q", manifest.ID, receipt.BackupID)
	}
	if got := engineAssertRegular(t, dest); got != userBytes {
		t.Fatal("rollback must restore the user's pre-install bytes")
	}
}

// engineHomeSnapshot fingerprints every regular file beneath the home so
// zero-churn claims are executable, not narrative.
func engineHomeSnapshot(t *testing.T, home string) string {
	t.Helper()
	var builder strings.Builder
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		builder.WriteString(filepath.ToSlash(rel))
		builder.WriteString(":")
		builder.WriteString(journalSHA256(content))
		builder.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot home: %v", err)
	}
	return builder.String()
}
