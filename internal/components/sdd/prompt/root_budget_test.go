package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func rootBudgetRepository(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}

func rootBudgetTokens(content string) int { return (utf8.RuneCountInString(content) + 2) / 3 }

func TestGeneratedRootAndCortexReferencesMeetExactBudgets(t *testing.T) {
	root := rootBudgetRepository(t)
	read := func(relative string) string {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := rootBudgetTokens(read("internal/assets/generic/sdd-orchestrator-root-index.md")); got < 900 || got > 1200 {
		t.Fatalf("root index budget: got %d tokens, want 900..1200", got)
	}
	shared := read("internal/assets/skills/_shared/cortex-convention.md")
	if got := rootBudgetTokens(shared); got < 700 || got > 1000 {
		t.Fatalf("shared Cortex convention budget: got %d tokens, want 700..1000", got)
	}
	advanced := read("internal/assets/skills/_shared/cortex-advanced.md")
	if got := rootBudgetTokens(advanced); got < 150 || got > 300 {
		t.Fatalf("advanced Cortex module budget: got %d tokens, want 150..300", got)
	}
	for _, entry := range []string{"routing-and-risk", "contracts-and-thresholds", "recovery-and-reflection", "parallel-apply", "memory-and-state", "model-routing"} {
		content := read("internal/assets/generic/sdd-root/" + entry + ".md")
		if got := rootBudgetTokens(content); got < 150 || got > 300 {
			t.Errorf("root module %s budget: got %d tokens, want 150..300", entry, got)
		}
		if strings.Contains(content, "cortex-convention.md") && !strings.Contains(content, "reference") {
			t.Errorf("root module %s must use a reference, not duplicate Cortex policy", entry)
		}
	}
}
