package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestPostPipelineEffects(t *testing.T) {
	cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	})
	defer cleanup()

	// 1. Dry-run calls no writer and reports no applied effects
	t.Run("DryRun", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		opts := Options{DryRun: true}
		rec, err := s.Install(opts)
		if err != nil {
			t.Fatalf("Install dry-run failed: %v", err)
		}
		if len(rec.PostPipelineEffects) > 0 {
			t.Errorf("expected no post-pipeline effects on dry-run, got: %v", rec.PostPipelineEffects)
		}
		if len(rec.Qualified) > 0 {
			t.Errorf("expected no qualified entries on dry-run, got: %v", rec.Qualified)
		}
	})

	// 2. Happy path: all effects requested and changed
	t.Run("AllChanged", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cfg := delegation.NormalConfig()
		opts := Options{DelegationConfig: &cfg}
		rec, err := s.Install(opts)
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		if len(rec.PostPipelineEffects) != 3 {
			t.Fatalf("expected 3 post-pipeline effects, got: %d", len(rec.PostPipelineEffects))
		}
		for _, eff := range rec.PostPipelineEffects {
			if eff.Status != EffectStatusChanged {
				t.Errorf("expected effect %s to be changed, got: %s", eff.Kind, eff.Status)
			}
			if eff.TransactionCovered {
				t.Errorf("expected TransactionCovered=false for effect %s", eff.Kind)
			}
		}
		if rec.PartialSuccess {
			t.Errorf("expected PartialSuccess=false on full success")
		}
	})

	// 3. Converged path: all effects unchanged
	t.Run("ConvergedUnchanged", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cfg := delegation.NormalConfig()
		opts := Options{DelegationConfig: &cfg}
		if _, err := s.Install(opts); err != nil {
			t.Fatalf("first Install failed: %v", err)
		}
		// Second install with mock returning unchanged
		runnerCleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
			return []byte("unchanged"), nil
		})
		defer runnerCleanup()

		rec, err := s.Install(opts)
		if err != nil {
			t.Fatalf("second Install failed: %v", err)
		}
		if !rec.Converged {
			t.Errorf("expected Converged=true on unchanged install")
		}
		for _, eff := range rec.PostPipelineEffects {
			if eff.Status != EffectStatusUnchanged {
				t.Errorf("expected effect %s to be unchanged, got: %s", eff.Kind, eff.Status)
			}
		}
	})

	// 4. Not requested effects
	t.Run("NotRequested", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		opts := Options{SkipEnvironment: true, SkipTUIPlugin: true, DelegationConfig: nil}
		rec, err := s.Install(opts)
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		for _, eff := range rec.PostPipelineEffects {
			if eff.Status != EffectStatusNotRequested {
				t.Errorf("expected effect %s to be not_requested, got: %s", eff.Kind, eff.Status)
			}
		}
	})

	// 5. Stop on failure: environment fails and remaining are not_attempted
	t.Run("StopOnFailureEnv", func(t *testing.T) {
		home := t.TempDir()
		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		runnerCleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
			return nil, errors.New("simulated environment failure")
		})
		defer runnerCleanup()

		cfg := delegation.NormalConfig()
		opts := Options{DelegationConfig: &cfg}
		rec, err := s.Install(opts)
		if err == nil {
			t.Fatalf("expected error on environment failure, got nil")
		}
		if len(rec.PostPipelineEffects) != 3 {
			t.Fatalf("expected 3 post-pipeline effects, got: %d", len(rec.PostPipelineEffects))
		}
		if rec.PostPipelineEffects[0].Status != EffectStatusFailed {
			t.Errorf("expected env failed, got: %s", rec.PostPipelineEffects[0].Status)
		}
		if rec.PostPipelineEffects[1].Status != EffectStatusNotAttempted {
			t.Errorf("expected tui not_attempted, got: %s", rec.PostPipelineEffects[1].Status)
		}
		if rec.PostPipelineEffects[2].Status != EffectStatusNotAttempted {
			t.Errorf("expected delegation not_attempted, got: %s", rec.PostPipelineEffects[2].Status)
		}
	})

	// 6. Surviving earlier effects: env succeeds, tui fails due to malformed file
	t.Run("SurvivingEarlierTUIFailure", func(t *testing.T) {
		home := t.TempDir()
		configDir := filepath.Join(home, ".config", "opencode")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		// Write malformed JSON to tui.jsonc
		if err := os.WriteFile(filepath.Join(configDir, "tui.jsonc"), []byte("{malformed"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		s, err := New(home)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cfg := delegation.NormalConfig()
		opts := Options{DelegationConfig: &cfg}
		rec, err := s.Install(opts)
		if err == nil {
			t.Fatalf("expected error on malformed tui.jsonc, got nil")
		}
		if !rec.PartialSuccess {
			t.Errorf("expected PartialSuccess=true when env changed before tui failure")
		}
		if rec.PostPipelineEffects[0].Status != EffectStatusChanged {
			t.Errorf("expected env changed, got: %s", rec.PostPipelineEffects[0].Status)
		}
		if rec.PostPipelineEffects[1].Status != EffectStatusFailed {
			t.Errorf("expected tui failed, got: %s", rec.PostPipelineEffects[1].Status)
		}
		if rec.PostPipelineEffects[2].Status != EffectStatusNotAttempted {
			t.Errorf("expected delegation not_attempted, got: %s", rec.PostPipelineEffects[2].Status)
		}
	})
}
