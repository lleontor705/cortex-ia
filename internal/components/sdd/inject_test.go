package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
)

func TestInjectSDD_ClaudeCode(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, nil)
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
	// Verify {{SKILLS_DIR}} was replaced with the shared skills dir.
	if strings.Contains(string(content), "{{SKILLS_DIR}}") {
		t.Error("expected {{SKILLS_DIR}} placeholder to be replaced")
	}
	expectedSkillsDir := filepath.ToSlash(filepath.Join(tmpDir, ".cortex-ia", "skills"))
	if !strings.Contains(string(content), expectedSkillsDir) {
		t.Errorf("expected orchestrator to contain shared skills dir %q", expectedSkillsDir)
	}

	// Verify skill files written to shared dir (~/.cortex-ia/skills/).
	bootstrapSkill := filepath.Join(tmpDir, ".cortex-ia", "skills", "bootstrap", "SKILL.md")
	if _, err := os.Stat(bootstrapSkill); os.IsNotExist(err) {
		t.Error("expected bootstrap skill in shared dir")
	}

	implementSkill := filepath.Join(tmpDir, ".cortex-ia", "skills", "implement", "SKILL.md")
	if _, err := os.Stat(implementSkill); os.IsNotExist(err) {
		t.Error("expected implement skill in shared dir")
	}

	// Verify convention refs replaced with absolute path (not inlined).
	implContent, err := os.ReadFile(implementSkill)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(implContent), "../_shared/cortex-convention.md") {
		t.Error("expected relative convention references to be replaced with absolute path")
	}
	if !strings.Contains(string(implContent), ".cortex-ia/skills/_shared/cortex-convention.md") {
		t.Error("expected absolute convention path in implement skill")
	}

	// Verify convention file exists in shared dir.
	conventionPath := filepath.Join(tmpDir, ".cortex-ia", "skills", "_shared", "cortex-convention.md")
	convData, err := os.ReadFile(conventionPath)
	if err != nil {
		t.Fatalf("convention not written: %v", err)
	}
	if !strings.Contains(string(convData), "Skill Loading Protocol") {
		t.Error("expected convention to contain Skill Loading Protocol")
	}
}

func TestInjectSDD_SkillCount(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Count skill files written to shared dir (19 skills + 1 convention = 20+).
	skillCount := 0
	for _, f := range result.Files {
		if strings.Contains(f, ".cortex-ia") && strings.HasSuffix(f, ".md") {
			skillCount++
		}
	}
	if skillCount < 20 {
		t.Errorf("expected at least 20 shared files (19 skills + convention), got %d", skillCount)
	}
}

func TestInlineConvention_AllSkills(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	_, err := Inject(tmpDir, adapter, nil)
	if err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(tmpDir, ".cortex-ia", "skills")
	for _, id := range sddSkillIDs {
		path := filepath.Join(skillsDir, id, "SKILL.md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("skill %q: %v", id, err)
			continue
		}
		s := string(content)
		if strings.Contains(s, "../_shared/cortex-convention.md") {
			t.Errorf("skill %q still has relative convention reference", id)
		}
	}

	// Verify convention file IS written to shared dir.
	convention := filepath.Join(skillsDir, "_shared", "cortex-convention.md")
	if _, err := os.Stat(convention); os.IsNotExist(err) {
		t.Error("convention should be written to shared _shared/ dir")
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

func TestInjectSDD_FileReplaceIsIdempotent(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := codex.NewAdapter()

	result, err := Inject(tmpDir, adapter, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected first inject to change files")
	}

	promptFile := adapter.SystemPromptFile(tmpDir)
	firstContent, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	firstText := string(firstContent)
	if strings.Count(firstText, "<!-- cortex-ia:sdd-orchestrator -->") != 1 {
		t.Fatalf("expected exactly one managed SDD open marker after first inject, got %d", strings.Count(firstText, "<!-- cortex-ia:sdd-orchestrator -->"))
	}

	second, err := Inject(tmpDir, adapter, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Fatal("expected second inject to be idempotent")
	}

	secondContent, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	secondText := string(secondContent)
	if secondText != firstText {
		t.Fatal("expected prompt file content to remain unchanged on second inject")
	}
	if strings.Count(secondText, "<!-- cortex-ia:sdd-orchestrator -->") != 1 {
		t.Fatalf("expected exactly one managed SDD open marker after second inject, got %d", strings.Count(secondText, "<!-- cortex-ia:sdd-orchestrator -->"))
	}
}
