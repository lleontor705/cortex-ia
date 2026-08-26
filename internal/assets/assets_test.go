package assets

import (
	"errors"
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

func TestReadBytes(t *testing.T) {
	data, err := ReadBytes("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadBytes(AGENTS.md) failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty AGENTS.md bytes")
	}

	// Non-existent
	_, err = ReadBytes("non-existent-asset.md")
	if err == nil {
		t.Error("expected error for non-existent asset")
	}
}

func TestSafeRelative(t *testing.T) {
	// Valid
	valid, err := SafeRelative("skills/implement/SKILL.md")
	if err != nil || valid != "skills/implement/SKILL.md" {
		t.Errorf("unexpected SafeRelative valid result: %s, %v", valid, err)
	}

	// Invalid empty
	if _, err := SafeRelative(""); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("expected ErrUnsafePath for empty, got: %v", err)
	}

	// Invalid backslash
	if _, err := SafeRelative("skills\\implement"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("expected ErrUnsafePath for backslash, got: %v", err)
	}

	// Invalid absolute
	if _, err := SafeRelative("/skills/implement"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("expected ErrUnsafePath for absolute, got: %v", err)
	}

	// Invalid volume
	if _, err := SafeRelative("C:skills/implement"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("expected ErrUnsafePath for volume, got: %v", err)
	}

	// Invalid traversal
	if _, err := SafeRelative("../skills/implement"); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("expected ErrUnsafePath for traversal, got: %v", err)
	}
}

func TestClassify(t *testing.T) {
	// Known roots
	cases := []struct {
		path string
		kind Kind
	}{
		{"AGENTS.md", KindAgentsDoc},
		{"opencode.jsonc", KindConfig},
		{"_shared/contract.md", KindShared},
		{"agents/implement.md", KindAgent},
		{"commands/work.md", KindCommand},
		{"skills/test/SKILL.md", KindSkill},
		{"plugins/cortex.ts", KindPlugin},
	}

	for _, tc := range cases {
		k, err := Classify(tc.path)
		if err != nil {
			t.Errorf("Classify(%s) failed: %v", tc.path, err)
		}
		if k != tc.kind {
			t.Errorf("Classify(%s) expected %s, got %s", tc.path, tc.kind, k)
		}
	}

	// Unmapped root
	if _, err := Classify("unmapped/root/file.txt"); !errors.Is(err, ErrUnmappedRoot) {
		t.Errorf("expected ErrUnmappedRoot for unknown root, got: %v", err)
	}
}
