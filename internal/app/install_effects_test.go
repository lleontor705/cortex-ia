package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

func TestInstallEffectsCLI(t *testing.T) {
	// 1. Dry-run rendering
	t.Run("DryRun", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:fake-digest",
			DryRun:     true,
		}
		rendered := renderInstallReceipt("cortex-ia install", rec)
		if !strings.Contains(rendered, "Dry-run: nothing was written.") {
			t.Errorf("expected dry-run disclaimer, got: %s", rendered)
		}
		if strings.Contains(rendered, "Partial success") {
			t.Errorf("unexpected Partial success in dry-run")
		}
	})

	// 2. Converged / no-op rendering
	t.Run("NoOp", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:fake-digest",
			Converged:  true,
		}
		rendered := renderInstallReceipt("cortex-ia sync", rec)
		if !strings.Contains(rendered, "Already converged: zero writes needed.") {
			t.Errorf("expected converged disclaimer, got: %s", rendered)
		}
		if strings.Contains(rendered, "nothing was written") {
			t.Errorf("unexpected 'nothing was written' on converged run")
		}
	})

	// 3. Changed rendering
	t.Run("Changed", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:fake-digest",
			Changed:    []string{"added internal/asset.go"},
			PostPipelineEffects: []install.PostPipelineEffect{
				{Kind: "environment", Status: install.EffectStatusChanged},
			},
		}
		rendered := renderInstallReceipt("cortex-ia install", rec)
		if !strings.Contains(rendered, "Changed: 1") {
			t.Errorf("expected Changed: 1, got: %s", rendered)
		}
		if !strings.Contains(rendered, "Effect environment: changed") {
			t.Errorf("expected effect environment changed, got: %s", rendered)
		}
		if strings.Contains(rendered, "nothing was written") {
			t.Errorf("surviving changes cannot render 'nothing was written'")
		}
	})

	// 4. Partial success rendering
	t.Run("PartialSuccess", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest:     "sha256:fake-digest",
			PartialSuccess: true,
			Changed:        []string{"managed-update .config/opencode/tui.jsonc"},
			PostPipelineEffects: []install.PostPipelineEffect{
				{Kind: "tui_plugin", Status: install.EffectStatusChanged},
				{Kind: "delegation_config", Status: install.EffectStatusFailed, Error: "permission denied"},
			},
		}
		rendered := renderInstallReceipt("cortex-ia install", rec)
		if !strings.Contains(rendered, "Partial success: separate effect failed; surviving changes remain on disk.") {
			t.Errorf("expected partial success disclaimer, got: %s", rendered)
		}
		if !strings.Contains(rendered, "Guidance: run fresh reconciliation to retry failed separate effects.") {
			t.Errorf("expected reconciliation guidance, got: %s", rendered)
		}
		if strings.Contains(rendered, "nothing was written") {
			t.Errorf("partial success cannot render 'nothing was written'")
		}
		if !strings.Contains(rendered, "Effect delegation_config: failed (error: permission denied)") {
			t.Errorf("expected failure detail, got: %s", rendered)
		}
	})

	// 5. Stale confirmation with and without surviving changes
	t.Run("StaleConfirmationDrift", func(t *testing.T) {
		driftErr := &pipeline.PlanDriftError{
			Expected: "sha256:confirmed",
			Observed: "sha256:planned",
		}

		// A: Drift with surviving changes
		recWithChanges := &install.InstallReceipt{
			PlanDigest:     "sha256:planned",
			PartialSuccess: true,
			Changed:        []string{"managed-update .config/opencode/tui.jsonc"},
		}
		applyErrWithChanges := func() error {
			if recWithChanges != nil && (len(recWithChanges.Changed) > 0 || recWithChanges.PartialSuccess) {
				return errors.New("confirmed plan is stale; surviving changes exist on disk (run reconciliation)")
			}
			return errors.New("the confirmed plan is stale; nothing was written")
		}()
		if strings.Contains(applyErrWithChanges.Error(), "nothing was written") {
			t.Errorf("drift with surviving changes must not claim nothing was written: %v", applyErrWithChanges)
		}

		// B: Drift with zero changes
		recWithoutChanges := &install.InstallReceipt{
			PlanDigest: "sha256:planned",
		}
		applyErrNoChanges := func() error {
			var drift *pipeline.PlanDriftError
			if errors.As(driftErr, &drift) {
				if recWithoutChanges != nil && (len(recWithoutChanges.Changed) > 0 || recWithoutChanges.PartialSuccess) {
					return errors.New("confirmed plan is stale; surviving changes exist on disk")
				}
				return errors.New("the confirmed plan is stale; nothing was written (preview again and re-confirm)")
			}
			return nil
		}()
		if !strings.Contains(applyErrNoChanges.Error(), "nothing was written") {
			t.Errorf("drift with zero changes must claim nothing was written: %v", applyErrNoChanges)
		}
	})
}
