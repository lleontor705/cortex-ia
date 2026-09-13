package delegation

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigWriteResult(t *testing.T) {
	t.Run("creation on clean target", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := NormalConfig()

		res, err := SaveWithResult(tempDir, cfg, SaveOptions{AllowOverwrite: false})
		if err != nil {
			t.Fatalf("unexpected error on creation: %v", err)
		}
		if res.Outcome != ConfigOutcomeCreated {
			t.Errorf("expected outcome %s, got %s", ConfigOutcomeCreated, res.Outcome)
		}
		if res.BytesWritten <= 0 {
			t.Errorf("expected positive BytesWritten, got %d", res.BytesWritten)
		}
		if _, err := os.Stat(ResolveConfigPath(tempDir)); err != nil {
			t.Errorf("expected file to exist: %v", err)
		}
	})

	t.Run("identical bytes produces noop", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := NormalConfig()

		if err := Save(tempDir, cfg); err != nil {
			t.Fatalf("initial save failed: %v", err)
		}

		res, err := SaveWithResult(tempDir, cfg, SaveOptions{AllowOverwrite: true})
		if err != nil {
			t.Fatalf("unexpected error on identical save: %v", err)
		}
		if res.Outcome != ConfigOutcomeNoOp {
			t.Errorf("expected outcome %s, got %s", ConfigOutcomeNoOp, res.Outcome)
		}
		if res.BytesWritten != 0 {
			t.Errorf("expected 0 BytesWritten for noop, got %d", res.BytesWritten)
		}
	})

	t.Run("authorized change updates content", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg1 := NormalConfig()
		if err := Save(tempDir, cfg1); err != nil {
			t.Fatalf("initial save failed: %v", err)
		}

		cfg2 := DefaultDelegationConfig(true)
		res, err := SaveWithResult(tempDir, cfg2, SaveOptions{AllowOverwrite: true})
		if err != nil {
			t.Fatalf("unexpected error on authorized change: %v", err)
		}
		if res.Outcome != ConfigOutcomeChanged {
			t.Errorf("expected outcome %s, got %s", ConfigOutcomeChanged, res.Outcome)
		}
		if res.BytesWritten <= 0 {
			t.Errorf("expected positive BytesWritten, got %d", res.BytesWritten)
		}
	})

	t.Run("unauthorized overwrite rejected", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg1 := NormalConfig()
		if err := Save(tempDir, cfg1); err != nil {
			t.Fatalf("initial save failed: %v", err)
		}

		cfg2 := DefaultDelegationConfig(true)
		_, err := SaveWithResult(tempDir, cfg2, SaveOptions{AllowOverwrite: false})
		if !errors.Is(err, ErrOverwriteDenied) {
			t.Fatalf("expected ErrOverwriteDenied, got: %v", err)
		}
	})

	t.Run("malformed request fails before write and preserves caller struct", func(t *testing.T) {
		tempDir := t.TempDir()
		badCfg := NormalConfig()
		badCfg.Version = "invalid-version-99"

		copied := badCfg
		_, err := SaveWithResult(tempDir, badCfg, SaveOptions{AllowOverwrite: true})
		if err == nil {
			t.Fatal("expected error for malformed request, got nil")
		}

		if !reflect.DeepEqual(badCfg, copied) {
			t.Error("caller-owned struct was mutated during validation")
		}
		if _, err := os.Stat(ResolveConfigPath(tempDir)); !os.IsNotExist(err) {
			t.Error("file was created despite malformed request")
		}
	})

	t.Run("malformed existing content detected", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := ResolveConfigPath(tempDir)
		if err := os.WriteFile(configPath, []byte("{invalid-json-content"), 0600); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}

		cfg := NormalConfig()
		_, err := SaveWithResult(tempDir, cfg, SaveOptions{AllowOverwrite: true})
		if !errors.Is(err, ErrMalformedConfig) {
			t.Fatalf("expected ErrMalformedConfig, got: %v", err)
		}
	})

	t.Run("stale or unmanaged preimage rejected", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg1 := NormalConfig()
		if err := Save(tempDir, cfg1); err != nil {
			t.Fatalf("initial save failed: %v", err)
		}

		cfg2 := DefaultDelegationConfig(true)
		wrongPreimage := []byte("different-preimage-bytes")
		_, err := SaveWithResult(tempDir, cfg2, SaveOptions{
			RequirePreimage:  true,
			ExpectedPreimage: wrongPreimage,
			AllowOverwrite:   true,
		})
		if !errors.Is(err, ErrPreimageMismatch) {
			t.Fatalf("expected ErrPreimageMismatch, got: %v", err)
		}
	})

	t.Run("matching preimage accepted", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg1 := NormalConfig()
		if err := Save(tempDir, cfg1); err != nil {
			t.Fatalf("initial save failed: %v", err)
		}
		currentBytes, _ := os.ReadFile(ResolveConfigPath(tempDir))

		cfg2 := DefaultDelegationConfig(true)
		res, err := SaveWithResult(tempDir, cfg2, SaveOptions{
			RequirePreimage:  true,
			ExpectedPreimage: currentBytes,
			AllowOverwrite:   true,
		})
		if err != nil {
			t.Fatalf("unexpected error with matching preimage: %v", err)
		}
		if res.Outcome != ConfigOutcomeChanged {
			t.Errorf("expected changed outcome, got %s", res.Outcome)
		}
	})

	t.Run("writer failure surfaces error", func(t *testing.T) {
		tempDir := t.TempDir()
		// Create a file where directory should be to force MkdirAll/WriteFile failure
		blockingFile := filepath.Join(tempDir, "blocking")
		if err := os.WriteFile(blockingFile, []byte("x"), 0600); err != nil {
			t.Fatalf("failed setup: %v", err)
		}
		badDir := filepath.Join(blockingFile, "sub")

		cfg := NormalConfig()
		_, err := SaveWithResult(badDir, cfg, SaveOptions{AllowOverwrite: true})
		if err == nil {
			t.Fatal("expected error on unwritable path, got nil")
		}
	})
}
