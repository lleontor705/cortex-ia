package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/config"
	"github.com/lleontor705/cortex-ia/internal/model"
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
		{"install", "--agent", "opencode", "--model-preset", "balanced"},
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
		{"install", "--agent", "opencode", "--model-preset", "balanced"},
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
		{"install", "--agent", "opencode", "--dry-run"},
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

func TestRunCLIInstallHelpReturnsBeforeHomeResolutionOrInstallation(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"--unknown", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Setenv("HOME", "")
			t.Setenv("USERPROFILE", "")

			var installErr error
			output := captureStdout(t, func() {
				installErr = runCLI(append([]string{"install"}, args...))
			})
			if installErr != nil {
				t.Fatalf("runCLI(install %v) error = %v, want nil", args, installErr)
			}
			if !strings.Contains(output, "Usage:") || !strings.Contains(output, "cortex-ia install") {
				t.Fatalf("runCLI(install %v) output = %q, want install help", args, output)
			}
			if strings.Contains(output, "Auto-detected agents:") || strings.Contains(output, "Installation complete.") {
				t.Fatalf("runCLI(install %v) continued into detection or installation: %q", args, output)
			}
		})
	}
}

func TestRunCLIInstallUnknownFlagFailsBeforeHomeResolutionOrMutation(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	err := runCLI([]string{"install", "--unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --unknown") {
		t.Fatalf("runInstall unknown flag error = %v, want unknown-flag error", err)
	}
	entries, readErr := os.ReadDir(homeDir)
	if readErr != nil {
		t.Fatalf("read temporary home: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("unknown flag mutated home before failing: %v", entries)
	}

	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	err = runCLI([]string{"install", "--still-unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --still-unknown") {
		t.Fatalf("runInstall resolved home before parsing unknown flag: %v", err)
	}
}

func TestParseInstallArgsPreservesValidFlags(t *testing.T) {
	selection, local, help, err := parseInstallArgs([]string{
		"--agent", "opencode",
		"--preset", "minimal",
		"--persona", "mentor",
		"--local",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("parseInstallArgs() error = %v", err)
	}
	if help {
		t.Fatal("parseInstallArgs() help = true, want false")
	}
	if !local {
		t.Fatal("parseInstallArgs() local = false, want true")
	}
	if got, want := selection.Agents, []model.AgentID{model.AgentOpenCode}; !slices.Equal(got, want) {
		t.Fatalf("parseInstallArgs() agents = %v, want %v", got, want)
	}
	if selection.Preset != model.PresetMinimal {
		t.Fatalf("parseInstallArgs() preset = %q, want %q", selection.Preset, model.PresetMinimal)
	}
	if selection.Persona != model.PersonaMentor {
		t.Fatalf("parseInstallArgs() persona = %q, want %q", selection.Persona, model.PersonaMentor)
	}
	if !selection.DryRun {
		t.Fatal("parseInstallArgs() dry-run = false, want true")
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
