package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

func TestUpdateCLIAuth(t *testing.T) {
	expectedMsg := "Authenticated update unavailable: no trusted release key is packaged"

	t.Run("default update CLI fails closed with exact error", func(t *testing.T) {
		err := runUpdate([]string{})
		if err == nil {
			t.Fatal("expected runUpdate to fail closed when trusted roots are absent, got nil")
		}
		if err.Error() != expectedMsg {
			t.Errorf("expected exact error %q, got %q", expectedMsg, err.Error())
		}
		if !errors.Is(err, updater.ErrNoTrustedKey) {
			t.Errorf("expected errors.Is(err, updater.ErrNoTrustedKey) to be true")
		}
	})

	t.Run("check flag fails closed with exact error", func(t *testing.T) {
		for _, flag := range []string{"--check", "-c"} {
			err := runUpdate([]string{flag})
			if err == nil {
				t.Fatalf("expected runUpdate(%q) to fail closed, got nil", flag)
			}
			if err.Error() != expectedMsg {
				t.Errorf("expected exact error %q, got %q", expectedMsg, err.Error())
			}
		}
	})

	t.Run("help flag remains usable without failure", func(t *testing.T) {
		for _, flag := range []string{"--help", "-h"} {
			if err := runUpdate([]string{flag}); err != nil {
				t.Errorf("expected help flag %q to succeed, got %v", flag, err)
			}
		}
	})

	t.Run("unknown flag rejected", func(t *testing.T) {
		err := runUpdate([]string{"--nonexistent-flag"})
		if err == nil || !strings.Contains(err.Error(), "unknown flag for update") {
			t.Errorf("expected unknown flag error, got: %v", err)
		}
	})
}
