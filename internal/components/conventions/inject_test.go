package conventions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestInjectConventions_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	// Verify cortex-convention.md was written to skills/_shared/.
	convention := filepath.Join(tmpDir, ".claude", "skills", "_shared", "cortex-convention.md")
	content, err := os.ReadFile(convention)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Cortex Convention") {
		t.Error("expected cortex convention content")
	}

	// Verify cortex-protocol was injected into system prompt.
	promptFile := filepath.Join(tmpDir, ".claude", "CLAUDE.md")
	prompt, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), "cortex-ia:cortex-protocol") {
		t.Error("expected cortex-protocol marker in CLAUDE.md")
	}
	if !strings.Contains(string(prompt), "Persistent Memory") {
		t.Error("expected cortex protocol content")
	}
}
