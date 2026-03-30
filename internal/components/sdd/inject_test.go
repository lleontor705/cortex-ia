package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestInjectSDD_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	// Verify orchestrator was injected into CLAUDE.md.
	promptFile := filepath.Join(tmpDir, ".claude", "CLAUDE.md")
	content, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "cortex-ia:sdd-orchestrator") {
		t.Error("expected SDD orchestrator marker in CLAUDE.md")
	}
	// Claude Code supports task delegation → should get multi-agent orchestrator.
	if !strings.Contains(string(content), "Principal Orchestrator") {
		t.Error("expected multi-agent orchestrator (Claude supports TaskDelegation)")
	}

	// Verify skill files were written.
	bootstrapSkill := filepath.Join(tmpDir, ".claude", "skills", "bootstrap", "SKILL.md")
	if _, err := os.Stat(bootstrapSkill); os.IsNotExist(err) {
		t.Error("expected bootstrap skill to be written")
	}

	implementSkill := filepath.Join(tmpDir, ".claude", "skills", "implement", "SKILL.md")
	if _, err := os.Stat(implementSkill); os.IsNotExist(err) {
		t.Error("expected implement skill to be written")
	}
}

func TestInjectSDD_SkillCount(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}

	// Count skill files written (should be 19 skills + 1 convention = minimum 20 files).
	skillCount := 0
	for _, f := range result.Files {
		if strings.Contains(f, "skills") && strings.HasSuffix(f, ".md") {
			skillCount++
		}
	}
	if skillCount < 19 {
		t.Errorf("expected at least 19 skill files, got %d", skillCount)
	}
}

func TestFilesToBackup(t *testing.T) {
	adapter := claude.NewAdapter()
	paths := FilesToBackup("/home/test", adapter)

	if len(paths) == 0 {
		t.Error("expected non-empty backup paths")
	}

	hasPrompt := false
	hasSkill := false
	for _, p := range paths {
		if strings.HasSuffix(p, "CLAUDE.md") {
			hasPrompt = true
		}
		if strings.Contains(p, "bootstrap") {
			hasSkill = true
		}
	}
	if !hasPrompt {
		t.Error("expected CLAUDE.md in backup paths")
	}
	if !hasSkill {
		t.Error("expected skill files in backup paths")
	}
}
