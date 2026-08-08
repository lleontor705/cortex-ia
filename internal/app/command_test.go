package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/config"
)

func TestRunCLI_RemovedGGACommand(t *testing.T) {
	err := runCLI([]string{"gga", "--list"})
	if err == nil {
		t.Fatal("runCLI(gga) returned nil; removed commands must be rejected")
	}
	if !strings.Contains(err.Error(), "unknown command: gga") {
		t.Fatalf("runCLI(gga) error = %q, want unknown-command error", err)
	}
}

func TestPreflightCLIRetiredSurfacesRejectsCompleteInvocation(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"agent-builder", "list"},
		{"auto-install", "--dry-run"},
		{"profiles", "list"},
		{"profile", "legacy"},
		{"list", "profiles"},
		{"install", "--local", "--profile", "legacy"},
		{"sync", "--dry-run", "--model-preset=balanced"},
		{"install", "--agent", "codex", "--model-preset", "balanced"},
	} {
		err := preflightCLI(args)
		if err == nil {
			t.Errorf("preflightCLI(%q) returned nil", args)
			continue
		}
		if !strings.Contains(err.Error(), "retired surface") {
			t.Errorf("preflightCLI(%q) error = %q, want retired-surface migration error", args, err)
		}
	}
}

func TestRunCLIRetiredSurfacesFailBeforeDispatch(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"agent-builder", "list"},
		{"auto-install", "--dry-run"},
		{"profiles", "list"},
		{"profile", "legacy"},
		{"list", "profiles"},
		{"install", "--local", "--profile", "legacy"},
		{"sync", "--dry-run", "--model-preset=balanced"},
		{"install", "--agent", "codex", "--model-preset", "balanced"},
	} {
		err := runCLI(args)
		var retired RetiredSurfaceError
		if !errors.As(err, &retired) {
			t.Errorf("runCLI(%q) error = %v, want RetiredSurfaceError", args, err)
		}
	}
}

func TestPreflightCLISupportedLifecycleRemainsEligible(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"install", "--agent", "claude-code", "--dry-run"},
		{"install", "--agent", "opencode", "--dry-run"},
		{"install", "--agent", "vscode-copilot", "--dry-run"},
		{"install", "--agent", "codex", "--dry-run"},
		{"sync", "--persona", "mentor", "--dry-run"},
		{"repair", "--dry-run"},
		{"doctor"},
		{"rollback"},
		{"uninstall", "--dry-run"},
		{"list", "agents"},
	} {
		if err := preflightCLI(args); err != nil {
			t.Errorf("preflightCLI(%q) error = %v, want nil", args, err)
		}
	}
}

func TestPreflightCLIGeminiClientIsRejected(t *testing.T) {
	t.Parallel()

	err := preflightCLI([]string{"install", "--agent", "gemini", "--dry-run"})
	var unsupported UnsupportedClientError
	if !errors.As(err, &unsupported) {
		t.Fatalf("preflightCLI() error = %v, want UnsupportedClientError", err)
	}
}

func TestRunInstallLocalConfig(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir project directory: %v", err)
	}

	t.Run("valid supported fields apply before installation", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(projectDir, config.FileName), []byte("agents:\n  - unavailable-test-agent\n"), 0o644); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		var installErr error
		output := captureStdout(t, func() {
			installErr = runInstall([]string{"--local", "--dry-run"})
		})
		if installErr == nil || !strings.Contains(installErr.Error(), "agent not found in registry") {
			t.Fatalf("runInstall valid local config error = %v, want post-application unknown-agent error", installErr)
		}
		if !strings.Contains(output, "Loaded project config from .cortex-ia.yaml") {
			t.Fatalf("runInstall output = %q, want loaded-project-config message", output)
		}
	})

	t.Run("retired fields fail closed before success output", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(projectDir, config.FileName), []byte("profile: retired\n"), 0o644); err != nil {
			t.Fatalf("write retired project config: %v", err)
		}

		var installErr error
		output := captureStdout(t, func() {
			installErr = runInstall([]string{"--local", "--dry-run"})
		})
		var retired *config.RetiredProjectFieldError
		if !errors.As(installErr, &retired) {
			t.Fatalf("runInstall retired local config error = %v, want RetiredProjectFieldError", installErr)
		}
		if strings.Contains(output, "Loaded project config from .cortex-ia.yaml") || strings.Contains(output, "Installation complete.") {
			t.Fatalf("runInstall emitted success output after retired config error: %q", output)
		}
		entries, err := os.ReadDir(homeDir)
		if err != nil {
			t.Fatalf("read temporary home: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("retired local config mutated home before failing: %v", entries)
		}
	})
}
