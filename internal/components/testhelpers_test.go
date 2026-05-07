package components_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/antigravity"
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	codexagent "github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/cursor"
	"github.com/lleontor705/cortex-ia/internal/agents/gemini"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/agents/vscode"
	"github.com/lleontor705/cortex-ia/internal/agents/windsurf"
)

var update = flag.Bool("update", false, "update golden files")

func claudeAdapter() agents.Adapter      { return claude.NewAdapter() }
func opencodeAdapter() agents.Adapter    { return opencode.NewAdapter() }
func cursorAdapter() agents.Adapter      { return cursor.NewAdapter() }
func geminiAdapter() agents.Adapter      { return gemini.NewAdapter() }
func vscodeAdapter() agents.Adapter      { return vscode.NewAdapter() }
func codexAdapter() agents.Adapter       { return codexagent.NewAdapter() }
func antigravityAdapter() agents.Adapter { return antigravity.NewAdapter() }
func windsurfAdapter() agents.Adapter    { return windsurf.NewAdapter() }

func goldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "golden")
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func assertGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	goldenPath := filepath.Join(goldenDir(t), name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("MkdirAll for golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", goldenPath, err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v\n\nRun with -update to generate golden files:\n  go test ./internal/components/ -run %s -update", goldenPath, err, t.Name())
	}

	if string(actual) != string(expected) {
		diffIdx := firstDiffIndex(string(expected), string(actual))
		const ctxLen = 80
		start := diffIdx - ctxLen
		if start < 0 {
			start = 0
		}
		end := diffIdx + ctxLen
		if end > len(expected) {
			end = len(expected)
		}
		t.Fatalf("golden mismatch for %s\nFirst diff at byte %d.\n\nExpected (around diff):\n%s\n\nActual (around diff):\n%s\n\nRun with -update to regenerate.", name, diffIdx, string(expected)[start:end], string(actual)[start:end])
	}
}

func firstDiffIndex(a, b string) int {
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return maxLen
	}
	return -1
}
