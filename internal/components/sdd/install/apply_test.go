package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestApplierVerifiesCompleteBackupBeforeFirstMutationAndStopsAfterFailure(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(t.TempDir(), "backups")
	paths := []string{
		"agents/a.md",
		"agents/a.md.cortex-ia.json",
		"agents/a.md.cortex-ia.base",
		"agents/b.md",
		"agents/b.md.cortex-ia.json",
		"agents/b.md.cortex-ia.base",
		"install/compatibility.json",
	}
	for _, path := range paths {
		writeTarget(t, root, path, []byte("old:"+path+"\n"), 0o600)
	}

	plan := Plan{
		Updates: []Effect{
			{Path: "agents/a.md", Content: []byte("new:a\n"), AfterMode: 0o600},
			{Path: "agents/b.md", Content: []byte("new:b\n"), AfterMode: 0o600},
		},
		Backup: BackupScope{Required: true, Paths: paths},
	}
	writes := 0
	applier := NewApplier(root, backupRoot)
	applier.beforeMutation = func(receipt Receipt, effect Effect) error {
		writes++
		if !receipt.BackupVerified || !receipt.RestoreAvailable {
			t.Fatal("first mutation started before a verified restorable backup")
		}
		if err := backup.Verify(receipt.Backup); err != nil {
			t.Fatalf("backup not verifiable before mutation: %v", err)
		}
		if writes == 2 {
			return errors.New("injected second-write failure")
		}
		return nil
	}

	receipt, err := applier.Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "injected second-write failure") {
		t.Fatalf("Apply() error = %v, want injected failure", err)
	}
	if !receipt.BackupVerified || !receipt.RestoreAvailable || receipt.Backup.ID == "" {
		t.Fatalf("receipt = %+v, want verified restoration evidence", receipt)
	}
	if got := manifestOriginalPaths(receipt.Backup); !reflect.DeepEqual(got, absolutePaths(root, paths)) {
		t.Fatalf("backed-up paths = %v, want %v", got, absolutePaths(root, paths))
	}
	assertTarget(t, root, "agents/a.md", "new:a\n")
	assertTarget(t, root, "agents/b.md", "old:agents/b.md\n")
}

func TestApplierNoOpCreatesNoReplacementBackup(t *testing.T) {
	backupRoot := filepath.Join(t.TempDir(), "backups")
	receipt, err := NewApplier(t.TempDir(), backupRoot).Apply(Plan{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if receipt.Backup.ID != "" || receipt.BackupVerified || receipt.RestoreAvailable {
		t.Fatalf("no-op receipt = %+v, want no backup", receipt)
	}
	if _, err := os.Stat(backupRoot); !os.IsNotExist(err) {
		t.Fatalf("no-op install created backup root: %v", err)
	}
}

func TestPlannerBacksUpChangedTargetsOwnershipAndCompatibilityMetadata(t *testing.T) {
	root := t.TempDir()
	content := []byte("old\n")
	writeTarget(t, root, "agents/implement.md", content, 0o600)
	writeTarget(t, root, "agents/implement.md.cortex-ia.json", []byte("ownership\n"), 0o600)
	writeTarget(t, root, "agents/implement.md.cortex-ia.base", content, 0o600)
	writeTarget(t, root, "install/compatibility.json", []byte("compat\n"), 0o600)

	plan, err := NewPlanner(root).Plan(PlanRequest{
		Bundle:                bundleWithAsset("agents/implement.md", "asset/agent/implement", []byte("new\n")),
		Managed:               []ManagedAsset{managedAsset(t, "agents/implement.md", "asset/agent/implement", content, 0o600)},
		Profile:               "portable-sequential",
		CompatibilityMetadata: []string{"install/compatibility.json"},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := []string{
		"agents/implement.md",
		"agents/implement.md.cortex-ia.base",
		"agents/implement.md.cortex-ia.json",
		"install/compatibility.json",
	}
	if !plan.Backup.Required || !reflect.DeepEqual(plan.Backup.Paths, want) {
		t.Fatalf("backup scope = %+v, want %v", plan.Backup, want)
	}
}

func manifestOriginalPaths(manifest backup.Manifest) []string {
	paths := make([]string, len(manifest.Entries))
	for i, entry := range manifest.Entries {
		paths[i] = entry.OriginalPath
	}
	return paths
}

func absolutePaths(root string, paths []string) []string {
	result := make([]string, len(paths))
	for i, path := range paths {
		result[i] = filepath.Join(root, filepath.FromSlash(path))
	}
	return result
}

func assertTarget(t *testing.T, root, path, want string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}

func bundleWithAsset(path string, semanticID ir.SemanticID, content []byte) renderers.Bundle {
	return renderers.Bundle{Assets: []renderers.Asset{{
		Path: path, SemanticID: semanticID, Kind: renderers.AssetAgent, Content: content, Mode: 0o600,
	}}}
}
