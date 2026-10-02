package app

import (
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

func TestRunUpdateHelp(t *testing.T) {
	if err := runUpdate([]string{"--help"}); err != nil {
		t.Errorf("expected nil error on --help, got %v", err)
	}
	if err := runUpdate([]string{"-h"}); err != nil {
		t.Errorf("expected nil error on -h, got %v", err)
	}
}

func TestRunUpdateUnknownFlag(t *testing.T) {
	err := runUpdate([]string{"--invalid-flag"})
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

func TestRunUpdateDevAndNonCanonicalBuildNotice(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{"dev build", "dev"},
		{"empty version", ""},
		{"git-describe version", "v0.4.53-5-gabc1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CORTEX_IA_HOME", t.TempDir())
			withVersion(t, tc.version)

			out, err := captureStdout(func() error {
				return runUpdate(nil)
			})
			if err != nil {
				t.Fatalf("expected nil error for dev build notice, got %v", err)
			}
			expectedNotice := updater.SelfUpdateDisabledNotice(tc.version)
			if !strings.Contains(out, expectedNotice) {
				t.Errorf("expected output to contain notice %q, got:\n%s", expectedNotice, out)
			}
		})

		t.Run(tc.name+"_check_only", func(t *testing.T) {
			t.Setenv("CORTEX_IA_HOME", t.TempDir())
			withVersion(t, tc.version)

			out, err := captureStdout(func() error {
				return runUpdate([]string{"--check"})
			})
			if err != nil {
				t.Fatalf("expected nil error for dev build notice on --check, got %v", err)
			}
			expectedNotice := updater.SelfUpdateDisabledNotice(tc.version)
			if !strings.Contains(out, expectedNotice) {
				t.Errorf("expected output to contain notice %q, got:\n%s", expectedNotice, out)
			}
		})
	}
}

func TestRunUpdateManualBypassesRecentCheckTTL(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"manual update apply", nil},
		{"manual update check-only", []string{"--check"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CORTEX_IA_HOME", home)
			withVersion(t, "v0.4.9")

			// Seed state indicating a recent check within 24 hours with no update available.
			state := updater.UpdateState{
				SchemaVersion: updater.UpdateStateSchemaVersion,
				LastCheckedAt: time.Now().UTC(),
				Available:     "",
			}
			if err := updater.SaveUpdateStateAtomic(home, state); err != nil {
				t.Fatalf("failed to seed update state: %v", err)
			}

			transport := stubUpdateCheck(t, "v0.4.9")

			out, err := captureStdout(func() error {
				return runUpdate(tc.args)
			})
			if err != nil {
				t.Fatalf("runUpdate failed: %v", err)
			}

			if transport.calls != 1 {
				t.Fatalf("expected network check to be performed (calls=1), got calls=%d", transport.calls)
			}

			if !strings.Contains(out, "Checking for cortex-ia updates") {
				t.Errorf("expected check message in output, got:\n%s", out)
			}
			if !strings.Contains(out, "already up to date") {
				t.Errorf("expected up to date message in output, got:\n%s", out)
			}
		})
	}
}

func TestPrintDualInstallWarning(t *testing.T) {
	t.Run("from candidates slice", func(t *testing.T) {
		out, err := captureStdout(func() error {
			printDualInstallWarning([]string{"/usr/local/bin/cortex-ia", "/opt/homebrew/bin/cortex-ia"})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "multiple cortex-ia installations detected") {
			t.Errorf("expected dual install warning, got: %q", out)
		}
	})

	t.Run("single candidate produces no warning", func(t *testing.T) {
		out, err := captureStdout(func() error {
			printDualInstallWarning([]string{"/usr/local/bin/cortex-ia"})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected no output for single candidate, got: %q", out)
		}
	})
}
