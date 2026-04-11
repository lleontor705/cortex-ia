package agentbuilder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstall_WritesSkillFile(t *testing.T) {
	homeDir := t.TempDir()
	agent := &GeneratedAgent{
		SkillName:    "my-agent",
		SkillContent: "# My Agent\n\nContent here.",
	}

	result, err := Install(homeDir, agent)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	if result.Err != nil {
		t.Fatalf("InstallResult.Err: %v", result.Err)
	}

	if len(result.FilesWritten) != 1 {
		t.Fatalf("FilesWritten len = %d, want 1", len(result.FilesWritten))
	}

	expectedPath := filepath.Join(homeDir, ".cortex-ia", "skills-community", "my-agent", "SKILL.md")
	if result.FilesWritten[0] != expectedPath {
		t.Errorf("FilesWritten[0] = %q, want %q", result.FilesWritten[0], expectedPath)
	}

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(data) != agent.SkillContent {
		t.Errorf("file content = %q, want %q", string(data), agent.SkillContent)
	}
}

func TestInstall_CreatesDirectory(t *testing.T) {
	homeDir := t.TempDir()
	agent := &GeneratedAgent{
		SkillName:    "nested-agent",
		SkillContent: "# Nested\n",
	}

	_, err := Install(homeDir, agent)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	skillDir := filepath.Join(homeDir, ".cortex-ia", "skills-community", "nested-agent")
	info, err := os.Stat(skillDir)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", skillDir)
	}
}

func TestInstall_NilAgent(t *testing.T) {
	_, err := Install(t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestInstall_EmptyHomeDir(t *testing.T) {
	agent := &GeneratedAgent{
		SkillName:    "test",
		SkillContent: "# Test\n",
	}

	_, err := Install("", agent)
	if err == nil {
		t.Fatal("expected error for empty homeDir")
	}
}
