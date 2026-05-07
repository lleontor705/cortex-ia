package agentbuilder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func newAgent(name string) *GeneratedAgent {
	return &GeneratedAgent{
		Name:    name,
		Title:   "T",
		Content: "# " + name + "\n\n## Description\nx\n\n## Trigger\nx\n\n## Instructions\nx\n",
	}
}

func TestInstallToAdapters_HappyPath(t *testing.T) {
	home := t.TempDir()
	skillsA := filepath.Join(home, "claude-skills")
	skillsB := filepath.Join(home, "opencode-skills")

	res, err := InstallToAdapters(newAgent("my-tool"), []AdapterInfo{
		{ID: model.AgentClaudeCode, SkillsDir: skillsA, HasSkills: true},
		{ID: model.AgentOpenCode, SkillsDir: skillsB, HasSkills: true},
	})
	if err != nil {
		t.Fatalf("InstallToAdapters: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	for _, r := range res {
		if !r.Success {
			t.Errorf("result %v failed: %v", r.AgentID, r.Err)
		}
		if _, err := os.Stat(r.Path); err != nil {
			t.Errorf("file not written for %v: %v", r.AgentID, err)
		}
	}
}

func TestInstallToAdapters_SkillsUnsupported(t *testing.T) {
	res, err := InstallToAdapters(newAgent("my-tool"), []AdapterInfo{
		{ID: model.AgentClaudeCode, HasSkills: false},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 || res[0].Success {
		t.Errorf("expected single failed result, got %v", res)
	}
}

func TestInstallToAdapters_NilAgent(t *testing.T) {
	if _, err := InstallToAdapters(nil, nil); err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestInstallToAdapters_EmptyName(t *testing.T) {
	if _, err := InstallToAdapters(&GeneratedAgent{Content: "x"}, nil); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestInstallToAdapters_EmptyContent(t *testing.T) {
	if _, err := InstallToAdapters(&GeneratedAgent{Name: "x"}, nil); err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestInstallToAdapters_RollbackOnFailure(t *testing.T) {
	home := t.TempDir()
	good := filepath.Join(home, "good-skills")

	// Second adapter points at a path under a regular file (mkdir will fail).
	blocker := filepath.Join(home, "blocker-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	bad := filepath.Join(blocker, "child", "skills")

	_, err := InstallToAdapters(newAgent("rollback-me"), []AdapterInfo{
		{ID: model.AgentClaudeCode, SkillsDir: good, HasSkills: true},
		{ID: model.AgentOpenCode, SkillsDir: bad, HasSkills: true},
	})
	if err == nil {
		t.Fatal("expected error from second adapter")
	}

	// First adapter's file must be rolled back.
	rolled := filepath.Join(good, "rollback-me", "SKILL.md")
	if _, err := os.Stat(rolled); !os.IsNotExist(err) {
		t.Errorf("first adapter file not rolled back: err=%v", err)
	}
}
