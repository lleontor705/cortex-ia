package mcpmanager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPQualificationContract(t *testing.T) {
	preset, found := Lookup("context7")
	if !found {
		t.Fatal("expected context7 preset in catalog")
	}

	cmd, ok := preset.Command()
	if !ok || len(cmd) < 3 {
		t.Fatalf("expected valid command vector for context7: %v", cmd)
	}
	if cmd[2] != "@upstash/context7-mcp@4.1.0" {
		t.Fatalf("expected pinned @upstash/context7-mcp@4.1.0, got %s", cmd[2])
	}

	t.Run("valid offline qualification with optional absent capabilities", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err != nil || !probeEv.Valid {
			t.Fatalf("expected valid qualification evidence, got err: %v, valid: %v", err, probeEv.Valid)
		}
		if !strings.Contains(probeEv.Summary, "optional absent: prompts, resources") {
			t.Errorf("expected optional absent capabilities reported in summary, got: %s", probeEv.Summary)
		}
	})

	t.Run("rejects mismatched package", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.PackageName = "@other/mcp"
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error on mismatched package")
		}
	})

	t.Run("rejects mismatched executable", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.ExecutablePath = "dist/server.js"
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error on mismatched executable")
		}
	})

	t.Run("rejects mismatched lock integrity", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.LockIntegrity = "sha512-tampered=="
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error on mismatched lock integrity")
		}
	})

	t.Run("rejects missing required schema", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.RequiredTools = []string{"other_tool"}
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error on missing required schema tools")
		}
	})

	t.Run("rejects malformed input", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.Version = ""
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error on empty version")
		}
	})

	t.Run("root npx pin cannot imply frozen transitive resolution", func(t *testing.T) {
		ev := DefaultContext7QualificationEvidence()
		ev.LockIntegrity = ""
		probeEv, err := ValidateQualificationEvidence(preset, ev)
		if err == nil || probeEv.Valid {
			t.Fatal("expected error when lock integrity is empty")
		}
	})

	t.Run("rejects unmanaged overwrite", func(t *testing.T) {
		tempDir := t.TempDir()
		cfgDir := filepath.Join(tempDir, ".config", "opencode")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Write existing unmanaged opencode.json with user context7 entry
		userJSON := `{
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp@4.1.0"],
      "enabled": true
    }
  }
}`
		cfgPath := filepath.Join(cfgDir, "opencode.json")
		if err := os.WriteFile(cfgPath, []byte(userJSON), 0o644); err != nil {
			t.Fatal(err)
		}

		mgr := New(tempDir)
		_, err := mgr.Add("context7", nil, OfflineQualificationProbe(DefaultContext7QualificationEvidence()))
		if err == nil {
			t.Fatal("expected Add to fail with conflict when entry is unmanaged")
		}
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Kind != ConflictUnaccredited {
			t.Errorf("expected ConflictUnaccredited error, got: %v", err)
		}
	})
}
