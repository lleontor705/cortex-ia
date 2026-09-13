package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestEffectRecovery(t *testing.T) {
	cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	})
	defer cleanup()

	// 1. Partial failure and retry with fresh explicit authorization
	t.Run("PartialFailureAndRetry", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		// Install with environment only
		opts := Options{SkipTUIPlugin: true}
		rec, err := s.Install(opts)
		if err != nil {
			t.Fatalf("initial install failed: %v", err)
		}
		initialTx := rec.TransactionID

		// Recover with delegation config added
		cfg := delegation.NormalConfig()
		cfg.UseHerdr = true
		recovOpts := EffectRecoveryOptions{
			DelegationConfig: &cfg,
		}
		recovRec, err := s.RecoverEffect(recovOpts)
		if err != nil {
			t.Fatalf("RecoverEffect failed: %v", err)
		}
		if recovRec.TransactionID != initialTx {
			t.Errorf("expected transaction ID preserved: got %s, want %s", recovRec.TransactionID, initialTx)
		}
		if len(recovRec.PostPipelineEffects) != 1 || recovRec.PostPipelineEffects[0].Status != EffectStatusChanged {
			t.Errorf("expected 1 changed delegation effect, got: %v", recovRec.PostPipelineEffects)
		}
	})

	// 2. Fresh and stale preimages
	t.Run("PreimageValidation", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cfg1 := delegation.NormalConfig()
		if _, err := s.Install(Options{DelegationConfig: &cfg1, SkipEnvironment: true, SkipTUIPlugin: true}); err != nil {
			t.Fatalf("setup install failed: %v", err)
		}

		configPath := filepath.Join(home, ".config", "opencode", "cortex-delegation.json")
		currentBytes, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read current config: %v", err)
		}

		// A: Stale preimage fails truthfully
		cfg2 := cfg1
		cfg2.UseHerdr = true
		staleOpts := EffectRecoveryOptions{
			DelegationConfig: &cfg2,
			RequirePreimage:  true,
			ExpectedPreimage: []byte(`{"version":"stale"}`),
		}
		_, err = s.RecoverEffect(staleOpts)
		if err == nil {
			t.Fatal("expected error with stale preimage, got nil")
		}
		if !errors.Is(err, delegation.ErrPreimageMismatch) && !bytes.Contains([]byte(err.Error()), []byte("preimage")) {
			t.Errorf("expected preimage mismatch error, got: %v", err)
		}

		// B: Fresh preimage succeeds
		freshOpts := EffectRecoveryOptions{
			DelegationConfig: &cfg2,
			RequirePreimage:  true,
			ExpectedPreimage: currentBytes,
		}
		rec, err := s.RecoverEffect(freshOpts)
		if err != nil {
			t.Fatalf("RecoverEffect with fresh preimage failed: %v", err)
		}
		if rec.PostPipelineEffects[0].Status != EffectStatusChanged {
			t.Errorf("expected changed effect on fresh preimage, got: %s", rec.PostPipelineEffects[0].Status)
		}
	})

	// 3. No-op retry
	t.Run("NoOpRetry", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cfg := delegation.NormalConfig()
		if _, err := s.Install(Options{DelegationConfig: &cfg, SkipEnvironment: true, SkipTUIPlugin: true}); err != nil {
			t.Fatalf("setup install failed: %v", err)
		}

		// Retry with identical config
		rec, err := s.RecoverEffect(EffectRecoveryOptions{
			DelegationConfig: &cfg,
		})
		if err != nil {
			t.Fatalf("no-op retry failed: %v", err)
		}
		if rec.PostPipelineEffects[0].Status != EffectStatusUnchanged {
			t.Errorf("expected unchanged effect on no-op retry, got: %s", rec.PostPipelineEffects[0].Status)
		}
		if !rec.Converged {
			t.Errorf("expected Converged=true on no-op recovery")
		}
	})

	// 4. Pipeline journal, digest, and backup remain unchanged
	t.Run("PipelinePreservation", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		rec, err := s.Install(Options{SkipTUIPlugin: true})
		if err != nil {
			t.Fatalf("install failed: %v", err)
		}

		metaBefore := state.LoadMetadataV2(home)
		backupsBefore, err := s.ListBackups()
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		// Run recovery
		cfg := delegation.NormalConfig()
		_, err = s.RecoverEffect(EffectRecoveryOptions{DelegationConfig: &cfg})
		if err != nil {
			t.Fatalf("RecoverEffect failed: %v", err)
		}

		metaAfter := state.LoadMetadataV2(home)
		backupsAfter, err := s.ListBackups()
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		if metaBefore.Metadata.TransactionID != metaAfter.Metadata.TransactionID {
			t.Errorf("transaction ID changed after recovery: before %s, after %s", metaBefore.Metadata.TransactionID, metaAfter.Metadata.TransactionID)
		}
		if metaBefore.Metadata.BackupID != metaAfter.Metadata.BackupID {
			t.Errorf("backup ID changed after recovery: before %s, after %s", metaBefore.Metadata.BackupID, metaAfter.Metadata.BackupID)
		}
		if len(backupsBefore) != len(backupsAfter) {
			t.Errorf("backup list count changed: before %d, after %d", len(backupsBefore), len(backupsAfter))
		}
		if rec.TransactionID != metaAfter.Metadata.TransactionID {
			t.Errorf("receipt transaction ID mismatch")
		}
	})
}
