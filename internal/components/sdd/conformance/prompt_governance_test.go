package conformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptGovernanceCommandsAndRootAssets(t *testing.T) {
	root := repositoryRoot(t)
	assets := loadPromptAssets(t, root)
	report := ScanPromptGovernance(assets)
	if len(report.Violations) != 0 {
		t.Fatalf("prompt governance violations: %v", report.Violations)
	}
	if report.AssetsScanned == 0 || !report.Complete {
		t.Fatalf("incomplete prompt inventory: %+v", report)
	}
}

func TestPromptGovernanceRejectsSeededFailures(t *testing.T) {
	assets := []PromptAsset{
		{Path: "source/one.md", Layer: PromptLayerSource, Content: "# Shared\n\nSame paragraph."},
		{Path: "generated/two.md", Layer: PromptLayerGenerated, Content: "# Shared\n\nSame paragraph."},
		{Path: "installed/three.md", Layer: PromptLayerInstalled, Content: "status: pass\nKiro supports direct-child execution.\nagent-mailbox"},
		{Path: "source/commands/command.md", Layer: PromptLayerSource, Content: "---\nagent: orchestrator\n---\nActivate this workflow and capture the user-supplied context before dispatching the executable phase handler.\n"},
	}
	report := ScanPromptGovernance(assets)
	for _, want := range []PromptViolationCode{
		ViolationDuplicate,
		ViolationStaleTool,
		ViolationTerminalVocabulary,
		ViolationUnqualifiedCapability,
		ViolationBudget,
	} {
		if !report.Has(want) {
			t.Errorf("seeded corpus did not report %s: %+v", want, report.Violations)
		}
	}
}

func TestPromptGovernanceRejectsMissingCrossLayerAsset(t *testing.T) {
	report := ValidatePromptInventory([]PromptAsset{{Path: "source/root.md", Layer: PromptLayerSource, Content: "content"}}, []string{"source/root.md", "generated/root.md", "installed/root.md"})
	if report.Complete || !report.Has(ViolationInventory) {
		t.Fatalf("missing cross-layer asset was accepted: %+v", report)
	}
}

func TestPromptGovernanceTokenEstimatorUsesUTF8RuneCeiling(t *testing.T) {
	content := strings.Repeat("é", 2701)
	if got, want := promptTokenCount(content), 901; got != want {
		t.Fatalf("prompt token estimate: got %d, want %d", got, want)
	}
}

func loadPromptAssets(t *testing.T, root string) []PromptAsset {
	t.Helper()
	assets := make([]PromptAsset, 0)
	add := func(rel string, layer PromptLayer) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read prompt asset %s: %v", rel, err)
		}
		assets = append(assets, PromptAsset{Path: rel, Layer: layer, Content: string(data)})
	}
	add("internal/assets/generic/sdd-orchestrator-root-index.md", PromptLayerSource)
	entries, err := os.ReadDir(filepath.Join(root, "internal/assets/generic/sdd-root"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			add("internal/assets/generic/sdd-root/"+entry.Name(), PromptLayerSource)
		}
	}
	commands := filepath.Join(root, "internal/assets/opencode/commands")
	entries, err = os.ReadDir(commands)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			add("internal/assets/opencode/commands/"+entry.Name(), PromptLayerCataloged)
		}
	}
	return assets
}
