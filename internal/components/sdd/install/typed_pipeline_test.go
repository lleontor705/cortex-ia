package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestTypedInstallWritesOwnershipReceiptAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	bundle := renderers.Bundle{Assets: []renderers.Asset{{
		Path: "skills/bootstrap/SKILL.md", SemanticID: "skill/bootstrap", Kind: renderers.AssetSkill,
		Content: []byte("bootstrap\n"), Mode: 0o600,
	}}}
	request := PlanRequest{Bundle: bundle, Profile: "portable-sequential", OwnershipMarkers: true}
	plan, err := NewPlanner(root).Plan(request)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := NewApplier(root, filepath.Join(root, "backups")).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != ReceiptCommitted || len(receipt.Installed) != 1 {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, err := os.Stat(filepath.Join(root, "skills/bootstrap/SKILL.md.cortex-ia.json")); err != nil {
		t.Fatalf("ownership marker missing: %v", err)
	}
	managed, err := loadManagedForTest(root, bundle)
	if err != nil {
		t.Fatal(err)
	}
	reinstall, err := NewPlanner(root).Plan(PlanRequest{Bundle: bundle, Managed: managed, Profile: "portable-sequential", OwnershipMarkers: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(reinstall.Creates)+len(reinstall.Updates)+len(reinstall.Deletes) != 0 {
		t.Fatalf("unchanged reinstall mutated assets: %+v", reinstall)
	}
}

func TestTypedInstallRejectsMissingRequiredInventoryBeforeMutation(t *testing.T) {
	root := t.TempDir()
	plan, err := NewPlanner(root).Plan(PlanRequest{
		Bundle:         renderers.Bundle{Assets: []renderers.Asset{{Path: "root.md", SemanticID: "root/index", Kind: renderers.AssetInstruction, Content: []byte("root"), Mode: 0o600}}},
		RequiredAssets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: SHA256([]byte("different"))}},
		Profile:        "portable-sequential",
	})
	if err == nil || plan.Fingerprint != "" {
		t.Fatalf("missing required asset was accepted: plan=%+v err=%v", plan, err)
	}
}

func loadManagedForTest(root string, bundle renderers.Bundle) ([]ManagedAsset, error) {
	store := NewOwnershipStore(root)
	result := make([]ManagedAsset, 0, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		ownership, base, err := store.Read(asset.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, ManagedAsset{Path: asset.Path, Ownership: ownership, Base: base, Mode: asset.Mode})
	}
	return result, nil
}
