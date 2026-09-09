package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initStudioTest(t *testing.T, initial string) (model, string, string, StudioArchetype) {
	t.Helper()
	dir := t.TempDir()
	arch := studioArchetypes[0]
	targetDir := filepath.Join(dir, ".config", "opencode", "agents")
	targetPath := filepath.Join(targetDir, arch.ID+".md")
	if initial != "" {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("create target dir: %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(initial), 0o644); err != nil {
			t.Fatalf("write initial file: %v", err)
		}
	}
	m := sized(newModel(&fakeService{}, dir, "vtest"))
	m = press(m, "5")
	return m, targetDir, targetPath, arch
}

func TestAgentStudioNewTargetCreates(t *testing.T) {
	m, _, targetPath, arch := initStudioTest(t, "")
	if m.screen != screenAgentStudio || m.studioStep != StudioStepSelect {
		t.Fatalf("expected select step, got screen=%v step=%d", m.screen, m.studioStep)
	}
	m = press(m, "enter")
	if m.studioStep != StudioStepPreview || !strings.Contains(m.View(), arch.Name) {
		t.Fatalf("expected preview step with archetype name, got step=%d", m.studioStep)
	}
	m = press(m, "enter")
	if m.studioStep != StudioStepResult || strings.HasPrefix(m.studioResultMsg, "Error") || !strings.Contains(m.studioResultMsg, targetPath) {
		t.Fatalf("expected success result, got step=%d msg=%s", m.studioStep, m.studioResultMsg)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil || string(data) != generateAgentMarkdown(arch) {
		t.Fatalf("created agent content mismatch: %v", err)
	}
	if view := m.View(); !strings.Contains(view, "exitosamente") || !strings.Contains(view, "✅") {
		t.Fatalf("result view missing success indication:\n%s", view)
	}
	m = press(m, "enter")
	if m.screen != screenHome {
		t.Fatalf("expected enter from result to return home, got %v", m.screen)
	}
}

func TestAgentStudioExistingTargetRequiresBoundConfirmation(t *testing.T) {
	initial := "original unmanaged agent file content"
	m, _, targetPath, arch := initStudioTest(t, initial)

	m = press(m, "enter") // Step 0 -> Step 1 (Preview)
	if m.studioStep != StudioStepPreview {
		t.Fatalf("expected preview step, got %d", m.studioStep)
	}
	m = press(m, "enter") // Step 1 -> Step 3 (Overwrite confirmation)
	if m.studioStep != StudioStepOverwrite {
		t.Fatalf("expected overwrite confirmation step, got %d", m.studioStep)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil || string(data) != initial {
		t.Fatalf("existing file modified before confirmation: %v", err)
	}
	view := m.View()
	if !strings.Contains(view, "Confirm overwrite") || (!strings.Contains(view, arch.ID+".md") && !strings.Contains(view, targetPath)) ||
		!strings.Contains(view, "[y] yes, proceed") || !strings.Contains(view, "[n]/esc no, cancel") {
		t.Fatalf("confirmation view missing bound target or prompt:\n%s", view)
	}
	m = press(m, "enter")
	if m.studioStep != StudioStepOverwrite {
		t.Fatalf("enter must be inert during overwrite confirmation, got %d", m.studioStep)
	}
}

func TestAgentStudioCancelPreservesBytes(t *testing.T) {
	precious := "PRESERVE ME: important custom user configuration bytes"
	m, _, targetPath, _ := initStudioTest(t, precious)

	m = press(m, "enter") // Select (0) -> Preview (1)
	if m.studioStep != StudioStepPreview {
		t.Fatalf("expected preview step, got %d", m.studioStep)
	}

	for _, cancelKey := range []string{"n", "esc", "b"} {
		m = press(m, "enter") // Preview (1) -> Overwrite (3)
		if m.studioStep != StudioStepOverwrite {
			t.Fatalf("expected overwrite confirmation step, got %d", m.studioStep)
		}
		m = press(m, cancelKey)
		if m.studioStep != StudioStepPreview {
			t.Fatalf("cancelling with %q must return to preview, got %d", cancelKey, m.studioStep)
		}
		data, err := os.ReadFile(targetPath)
		if err != nil || string(data) != precious {
			t.Fatalf("bytes on disk modified after %q: %v", cancelKey, err)
		}
	}
}

func TestAgentStudioConfirmedReplacement(t *testing.T) {
	m, _, targetPath, arch := initStudioTest(t, "old obsolete content to be replaced")

	m = press(m, "enter") // Step 0 -> Step 1
	m = press(m, "enter") // Step 1 -> Step 3
	if m.studioStep != StudioStepOverwrite {
		t.Fatalf("expected overwrite confirmation step, got %d", m.studioStep)
	}
	m = press(m, "y")
	if m.studioStep != StudioStepResult || strings.HasPrefix(m.studioResultMsg, "Error") || !strings.Contains(m.studioResultMsg, targetPath) {
		t.Fatalf("unexpected error on confirmed replacement: %s", m.studioResultMsg)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil || string(data) != generateAgentMarkdown(arch) {
		t.Fatalf("replaced content mismatch: %v", err)
	}
	if view := m.View(); !strings.Contains(view, "✅") || !strings.Contains(view, "exitosamente") {
		t.Fatalf("result view missing success indication:\n%s", view)
	}
}

func TestAgentStudioCreateRaceNoClobber(t *testing.T) {
	m, targetDir, targetPath, _ := initStudioTest(t, "")

	m = press(m, "enter") // Step 0 -> Step 1 (preview)
	if m.studioStep != StudioStepPreview {
		t.Fatalf("expected preview step, got %d", m.studioStep)
	}

	// Concurrent race: external writer creates targetPath right before user presses enter to write
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	foreignBytes := []byte("foreign concurrent agent file - must not be clobbered")
	if err := os.WriteFile(targetPath, foreignBytes, 0o644); err != nil {
		t.Fatalf("failed to write foreign file: %v", err)
	}

	// User presses enter expecting new target creation
	m = press(m, "enter")

	// The file MUST NOT be clobbered
	data, err := os.ReadFile(targetPath)
	if err != nil || !bytes.Equal(data, foreignBytes) {
		t.Fatalf("race condition clobbered foreign bytes: got %s, want %s (err: %v)", string(data), string(foreignBytes), err)
	}

	// Tightened race contract: exclusive-create os.IsExist must render StudioStepResult error without overwrite confirmation
	if m.studioStep != StudioStepResult {
		t.Fatalf("expected result step on race error, got %d", m.studioStep)
	}
	if !strings.HasPrefix(m.studioResultMsg, "Error") {
		t.Fatalf("expected error result message on race, got %q", m.studioResultMsg)
	}
	view := m.View()
	if !strings.Contains(view, "❌") || !strings.Contains(view, "Error") {
		t.Fatalf("race failure view missing error indicator:\n%s", view)
	}
	if strings.Contains(view, "Confirm overwrite") || strings.Contains(view, "⚠ Confirm overwrite") {
		t.Fatalf("race condition must not offer overwrite confirmation:\n%s", view)
	}
	if strings.Contains(view, "✅") || strings.Contains(view, "¿Cómo invocar este subagente?") {
		t.Fatalf("race failure view reported false success:\n%s", view)
	}
}

func TestAgentStudioWriteFailureNoFalseSuccess(t *testing.T) {
	t.Run("NewTargetDirectoryConflict", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".config"), []byte("blocker"), 0o644); err != nil {
			t.Fatalf("failed to create blocker: %v", err)
		}
		m := sized(newModel(&fakeService{}, dir, "vtest"))
		m = press(m, "5")
		m = press(m, "enter")
		m = press(m, "enter")

		if m.studioStep != StudioStepResult || !strings.HasPrefix(m.studioResultMsg, "Error") {
			t.Fatalf("expected error result step, got step=%d msg=%q", m.studioStep, m.studioResultMsg)
		}
		if view := m.View(); !strings.Contains(view, "❌") || strings.Contains(view, "✅") {
			t.Fatalf("invalid view on failure:\n%s", view)
		}
	})

	t.Run("ConfirmedReplacementTargetIsDirectory", func(t *testing.T) {
		dir := t.TempDir()
		arch := studioArchetypes[0]
		targetPath := filepath.Join(dir, ".config", "opencode", "agents", arch.ID+".md")
		if err := os.MkdirAll(targetPath, 0o755); err != nil {
			t.Fatalf("failed to create directory target: %v", err)
		}

		m := sized(newModel(&fakeService{}, dir, "vtest"))
		m = press(m, "5")
		m = press(m, "enter") // Step 0 -> Step 1
		m = press(m, "enter") // Step 1 -> Step 3
		if m.studioStep != StudioStepOverwrite {
			t.Fatalf("expected overwrite confirmation step, got %d", m.studioStep)
		}

		m = press(m, "y")
		if m.studioStep != StudioStepResult || !strings.HasPrefix(m.studioResultMsg, "Error") {
			t.Fatalf("expected error result on directory replace, got step=%d msg=%q", m.studioStep, m.studioResultMsg)
		}
		if view := m.View(); !strings.Contains(view, "❌") || strings.Contains(view, "✅") {
			t.Fatalf("invalid view on failure:\n%s", view)
		}
	})
}
