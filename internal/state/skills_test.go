package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListCommunitySkills_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the community skills directory but leave it empty.
	dir := CommunitySkillsDir(tmpDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	skills := ListCommunitySkills(tmpDir)
	if skills != nil {
		t.Errorf("expected nil, got %v", skills)
	}
}

func TestListCommunitySkills_NoDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create the directory — should return nil without error.
	skills := ListCommunitySkills(tmpDir)
	if skills != nil {
		t.Errorf("expected nil, got %v", skills)
	}
}

func TestListCommunitySkills_WithSkills(t *testing.T) {
	tmpDir := t.TempDir()
	dir := CommunitySkillsDir(tmpDir)

	// Create two valid skill directories with SKILL.md.
	for _, name := range []string{"my-skill", "another-skill"} {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a directory WITHOUT SKILL.md — should be skipped.
	noSkill := filepath.Join(dir, "not-a-skill")
	if err := os.MkdirAll(noSkill, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a regular file — should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills := ListCommunitySkills(tmpDir)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(skills), skills)
	}

	// os.ReadDir returns entries sorted by name.
	if skills[0] != "another-skill" {
		t.Errorf("skills[0] = %q, want %q", skills[0], "another-skill")
	}
	if skills[1] != "my-skill" {
		t.Errorf("skills[1] = %q, want %q", skills[1], "my-skill")
	}
}
