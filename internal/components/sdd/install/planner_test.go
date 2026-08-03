package install

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestPlannerReportsExactEffectsWithoutMutatingTargets(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "update.md", []byte("old\n"), 0o600)
	writeTarget(t, root, "delete.md", []byte("remove\n"), 0o600)
	writeTarget(t, root, "conflict.md", []byte("user-owned\n"), 0o600)
	writeTarget(t, root, "mode.md", []byte("same\n"), 0o600)

	prior := []ManagedAsset{
		managedAsset(t, "update.md", "asset/update", []byte("old\n"), 0o600),
		managedAsset(t, "delete.md", "asset/delete", []byte("remove\n"), 0o600),
		managedAsset(t, "mode.md", "asset/mode", []byte("same\n"), 0o600),
	}
	request := PlanRequest{
		Bundle: renderers.Bundle{Assets: []renderers.Asset{
			{Path: "create.md", SemanticID: ir.SemanticID("asset/create"), Kind: renderers.AssetInstruction, Content: []byte("created\n"), Mode: 0o600},
			{Path: "update.md", SemanticID: ir.SemanticID("asset/update"), Kind: renderers.AssetInstruction, Content: []byte("new\n"), Mode: 0o600},
			{Path: "conflict.md", SemanticID: ir.SemanticID("asset/conflict"), Kind: renderers.AssetInstruction, Content: []byte("generated\n"), Mode: 0o600},
			{Path: "mode.md", SemanticID: ir.SemanticID("asset/mode"), Kind: renderers.AssetInstruction, Content: []byte("same\n"), Mode: 0o644},
		}},
		Managed:      prior,
		Profile:      "portable-sequential",
		Degradations: []string{"capability/delegation: unavailable"},
	}

	before := targetHashes(t, root, "update.md", "delete.md", "conflict.md", "mode.md")
	plan, err := NewPlanner(root).Plan(request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	after := targetHashes(t, root, "update.md", "delete.md", "conflict.md", "mode.md")
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("dry-run changed target hashes: before=%v after=%v", before, after)
	}
	if _, err := os.Stat(filepath.Join(root, "create.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created target: stat error = %v", err)
	}

	if got, want := effectPaths(plan.Creates), []string{"create.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("create paths = %v, want %v", got, want)
	}
	if got, want := effectPaths(plan.Updates), []string{"mode.md", "update.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("update paths = %v, want %v", got, want)
	}
	if got, want := effectPaths(plan.Deletes), []string{"delete.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("delete paths = %v, want %v", got, want)
	}
	if got, want := conflictPaths(plan.Conflicts), []string{"conflict.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("conflict paths = %v, want %v", got, want)
	}
	if got, want := permissionPaths(plan.PermissionChanges), []string{"mode.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("permission paths = %v, want %v", got, want)
	}
	if plan.Profile != request.Profile || !reflect.DeepEqual(plan.Degradations, request.Degradations) {
		t.Fatalf("disclosure = profile %q degradations %v", plan.Profile, plan.Degradations)
	}
	if !plan.Backup.Required || !reflect.DeepEqual(plan.Backup.Paths, []string{
		"create.md",
		"delete.md", "delete.md.cortex-ia.base", "delete.md.cortex-ia.json",
		"mode.md", "mode.md.cortex-ia.base", "mode.md.cortex-ia.json",
		"update.md", "update.md.cortex-ia.base", "update.md.cortex-ia.json",
	}) {
		t.Fatalf("backup = %+v", plan.Backup)
	}
	if plan.HasBlockingConflicts() != true {
		t.Fatal("plan with unknown ownership must be blocking")
	}
}

func TestPlannerUnchangedReinstallHasNoManagedChangesOrBackup(t *testing.T) {
	root := t.TempDir()
	content := []byte("unchanged\n")
	writeTarget(t, root, "agents/implement.md", content, 0o600)
	managed := managedAsset(t, "agents/implement.md", "asset/agent/implement", content, 0o600)
	request := PlanRequest{
		Bundle: renderers.Bundle{Assets: []renderers.Asset{{
			Path: "agents/implement.md", SemanticID: "asset/agent/implement", Kind: renderers.AssetAgent, Content: content, Mode: 0o600,
		}}},
		Managed: []ManagedAsset{managed},
		Profile: "portable-flat",
	}

	plan, err := NewPlanner(root).Plan(request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan.Creates)+len(plan.Updates)+len(plan.Deletes) != 0 {
		t.Fatalf("unchanged reinstall managed effects = creates:%v updates:%v deletes:%v", plan.Creates, plan.Updates, plan.Deletes)
	}
	if plan.Backup.Required || len(plan.Backup.Paths) != 0 {
		t.Fatalf("unchanged reinstall backup = %+v", plan.Backup)
	}
}

func managedAsset(t *testing.T, path string, semanticID ir.SemanticID, content []byte, mode os.FileMode) ManagedAsset {
	t.Helper()
	ownership, err := NewOwnership(path, "1.0.0", semanticID, content, content)
	if err != nil {
		t.Fatalf("NewOwnership() error = %v", err)
	}
	return ManagedAsset{Path: path, Ownership: ownership, Base: content, Mode: mode}
}

func writeTarget(t *testing.T, root, path string, content []byte, mode os.FileMode) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(fullPath, content, mode); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func targetHashes(t *testing.T, root string, paths ...string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		result[path] = SHA256(content)
	}
	return result
}

func effectPaths(effects []Effect) []string {
	paths := make([]string, len(effects))
	for index, effect := range effects {
		paths[index] = effect.Path
	}
	return paths
}

func conflictPaths(conflicts []PlanConflict) []string {
	paths := make([]string, len(conflicts))
	for index, conflict := range conflicts {
		paths[index] = conflict.Path
	}
	return paths
}

func permissionPaths(changes []PermissionChange) []string {
	paths := make([]string, len(changes))
	for index, change := range changes {
		paths[index] = change.Path
	}
	return paths
}
