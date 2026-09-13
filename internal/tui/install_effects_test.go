package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

func TestInstallEffectsTUI(t *testing.T) {
	applyInstall := func(fake *fakeService, msg installMsg) model {
		m := sized(newModel(fake, "/home/test", "vtest"))
		res, _ := m.onInstallDone(msg)
		return res.(model)
	}

	t.Run("PartialSuccess", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest:     "sha256:plan01",
			Configured:     []string{"cortex"},
			Qualified:      []string{"cortex"},
			Changed:        []string{"managed .config/opencode/tui.jsonc"},
			PartialSuccess: true,
			BackupID:       "bkp-001",
			PostPipelineEffects: []install.PostPipelineEffect{
				{Kind: "tui_plugin", Status: install.EffectStatusChanged},
				{Kind: "delegation_config", Status: install.EffectStatusFailed, Error: "write denied"},
			},
		}
		m := applyInstall(&fakeService{}, installMsg{receipt: rec})
		if m.result.pass {
			t.Fatal("partial success must never yield false PASS")
		}
		if m.result.canRollback {
			t.Fatal("partial success must not offer universal rollback")
		}
		view := m.View()
		if strings.Contains(view, "PASS") {
			t.Fatalf("partial success must not render PASS:\n%s", view)
		}
		if strings.Contains(view, "Rollback:") {
			t.Fatalf("partial success must not render rollback command:\n%s", view)
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "Partial success: separate effect failed; surviving changes remain on disk.") {
			t.Errorf("expected partial success disclaimer, got: %s", joined)
		}
		if !strings.Contains(joined, "Guidance: run fresh reconciliation to retry failed separate effects.") {
			t.Errorf("expected reconciliation guidance, got: %s", joined)
		}
		if strings.Contains(joined, "nothing was written") {
			t.Errorf("surviving changes cannot claim nothing was written")
		}
	})

	t.Run("DryRun", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:plan02",
			DryRun:     true,
			BackupID:   "bkp-dry",
		}
		m := applyInstall(&fakeService{}, installMsg{receipt: rec})
		if m.result.pass {
			t.Fatal("dry-run must never produce PASS")
		}
		if m.result.canRollback {
			t.Fatal("dry-run must not offer rollback")
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "Dry-run: preview only; nothing was written.") {
			t.Errorf("expected dry-run note, got: %s", joined)
		}
	})

	t.Run("NoOp", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:plan03",
			Configured: []string{"cortex"},
			Qualified:  []string{"cortex"},
			Converged:  true,
		}
		m := applyInstall(&fakeService{}, installMsg{receipt: rec})
		if !m.result.pass {
			t.Fatal("converged run with qualified MCPs should pass")
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "Already converged: zero writes needed.") {
			t.Errorf("expected converged note, got: %s", joined)
		}
		if strings.Contains(joined, "nothing was written") {
			t.Errorf("converged run should not say nothing was written")
		}
	})

	t.Run("NilReceipt", func(t *testing.T) {
		errPipeline := errors.New("hard pipeline crash")
		m := applyInstall(&fakeService{}, installMsg{receipt: nil, err: errPipeline})
		if m.result.pass {
			t.Fatal("nil receipt must never pass")
		}
		if m.result.canRollback {
			t.Fatal("nil receipt cannot rollback")
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "error: hard pipeline crash") {
			t.Errorf("expected error message, got: %s", joined)
		}
	})

	t.Run("NonNilClean", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest:     "sha256:plan04",
			Configured:     []string{"cortex"},
			Qualified:      []string{"cortex"},
			Changed:        []string{"created .config/opencode/AGENTS.md"},
			BackupID:       "bkp-clean-1",
			BackupVerified: true,
		}
		m := applyInstall(&fakeService{}, installMsg{receipt: rec})
		if !m.result.pass {
			t.Fatal("clean receipt must pass")
		}
		if !m.result.canRollback {
			t.Fatal("clean receipt with backup must allow rollback")
		}
		view := m.View()
		if !strings.Contains(view, "PASS") {
			t.Fatalf("expected PASS in view:\n%s", view)
		}
		if !strings.Contains(view, "Rollback: cortex-ia rollback bkp-clean-1") {
			t.Fatalf("expected rollback command in view:\n%s", view)
		}
	})

	t.Run("UnqualifiedMCPs", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest: "sha256:plan05",
			Configured: []string{"cortex", "unqualified-tool"},
			Qualified:  []string{"cortex"},
			Changed:    []string{"config updated"},
		}
		m := applyInstall(&fakeService{}, installMsg{receipt: rec})
		if m.result.pass {
			t.Fatal("unqualified MCP must fail")
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "FAIL: configured without valid qualification evidence: unqualified-tool") {
			t.Errorf("expected unqualified failure note, got: %s", joined)
		}
	})

	t.Run("RequiredEffectErrors", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest:     "sha256:plan06",
			Configured:     []string{"cortex"},
			Qualified:      []string{"cortex"},
			Changed:        []string{"file written"},
			PartialSuccess: true,
			PostPipelineEffects: []install.PostPipelineEffect{
				{Kind: "tui_plugin", Status: install.EffectStatusFailed, Error: "plugin lock timeout"},
			},
		}
		m := applyInstall(&fakeService{}, installMsg{
			receipt: rec,
			err:     errors.New("required effect failed: tui_plugin"),
		})
		if m.result.pass {
			t.Fatal("required effect error must not pass")
		}
		if m.result.canRollback {
			t.Fatal("required effect failure must not offer rollback")
		}
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "Effect tui_plugin: failed (error: plugin lock timeout)") {
			t.Errorf("expected effect error detail, got: %s", joined)
		}
		if !strings.Contains(joined, "error: required effect failed: tui_plugin") {
			t.Errorf("expected error message in detail, got: %s", joined)
		}
		if strings.Contains(joined, "nothing was written") {
			t.Errorf("surviving file written must not claim nothing was written")
		}
	})

	t.Run("StaleConfirmationDriftWithChanges", func(t *testing.T) {
		rec := &install.InstallReceipt{
			PlanDigest:     "sha256:plan07",
			PartialSuccess: true,
			Changed:        []string{"surviving .env"},
		}
		m := applyInstall(&fakeService{}, installMsg{
			receipt: rec,
			err:     pipeline.ErrPlanDrift,
		})
		joined := strings.Join(m.result.detail, "\n")
		if !strings.Contains(joined, "surviving changes remain on disk.") {
			t.Errorf("expected surviving changes disclaimer on drift, got: %s", joined)
		}
		if strings.Contains(joined, "nothing was written") {
			t.Errorf("drift with surviving changes must not claim nothing was written")
		}
	})
}
