package assets

import (
	"strings"
	"testing"
)

func TestInventory(t *testing.T) {
	files, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory() failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("Inventory() returned 0 files")
	}

	foundConfig, foundDoc, foundAgent, foundCommand, foundSkill, foundPlugin := false, false, false, false, false, false
	for _, f := range files {
		switch f.Kind {
		case KindConfig:
			foundConfig = true
		case KindAgentsDoc:
			foundDoc = true
		case KindAgent:
			foundAgent = true
		case KindCommand:
			foundCommand = true
		case KindSkill:
			foundSkill = true
		case KindPlugin:
			foundPlugin = true
		}
	}

	if !foundConfig || !foundDoc || !foundAgent || !foundCommand || !foundSkill || !foundPlugin {
		t.Fatalf("inventory missing kinds: config=%v doc=%v agent=%v command=%v skill=%v plugin=%v",
			foundConfig, foundDoc, foundAgent, foundCommand, foundSkill, foundPlugin)
	}
}

func TestReadSkill(t *testing.T) {
	content, err := Read("skills/implement/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty implement skill")
	}
}

func TestReadConvention(t *testing.T) {
	content, err := Read("skills/_shared/cortex-convention.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Cortex Convention") {
		t.Error("expected cortex convention content")
	}
}

func TestReadNonExistent(t *testing.T) {
	_, err := Read("nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
