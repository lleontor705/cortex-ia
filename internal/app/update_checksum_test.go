package app

import (
	"strings"
	"testing"
)

func TestUpdateChecksumsCli(t *testing.T) {
	t.Run("update_help_reflects_checksums_standard", func(t *testing.T) {
		out, _ := captureStdout(func() error {
			return runUpdate([]string{"--help"})
		})
		if !strings.Contains(out, "checksums.txt") {
			t.Fatalf("expected help output to mention checksums.txt, got:\n%s", out)
		}
		if !strings.Contains(out, "Deprecated") {
			t.Fatalf("expected help output to mark --allow-checksum-updates as deprecated, got:\n%s", out)
		}
	})

	t.Run("deprecated_flag_emits_notice", func(t *testing.T) {
		out, _ := captureStdout(func() error {
			return runUpdate([]string{"--check", "--allow-checksum-updates"})
		})
		if !strings.Contains(out, "Note: --allow-checksum-updates is deprecated") {
			t.Fatalf("expected deprecation notice, got:\n%s", out)
		}
	})

	t.Run("check_does_not_fail_closed_without_trust_bundle", func(t *testing.T) {
		_, err := captureStdout(func() error {
			return runUpdate([]string{"--check"})
		})
		if err != nil && strings.Contains(err.Error(), "no trusted release key is packaged") {
			t.Fatalf("update check failed closed on missing trust bundle: %v", err)
		}
	})
}
