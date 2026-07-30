package install

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	sddmerge "github.com/lleontor705/cortex-ia/internal/components/sdd/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestInstallationSafetyMatrix(t *testing.T) {
	t.Run("outside-region customization survives", func(t *testing.T) {
		root := t.TempDir()
		current := []byte("user: keep\nmanaged: old\nuser: tail\n")
		writeTarget(t, root, "AGENTS.md", current, 0o600)

		result, err := sddmerge.Merge(current, []sddmerge.ManagedRegion{{
			SemanticID:   "region/agents/workflow",
			Start:        len("user: keep\n"),
			End:          len("user: keep\nmanaged: old\n"),
			RecordedBase: []byte("managed: old\n"),
			Generated:    []byte("managed: new\n"),
		}})
		if err != nil {
			t.Fatalf("Merge() error = %v", err)
		}
		if len(result.Conflicts) != 0 || !bytes.Equal(result.Content, []byte("user: keep\nmanaged: new\nuser: tail\n")) {
			t.Fatalf("merge result = %#v", result)
		}
	})

	t.Run("managed conflict retains three versions and blocks mutation", func(t *testing.T) {
		_ = t.TempDir()
		current := []byte("outside\nmanaged: user\ntail\n")
		result, err := sddmerge.Merge(current, []sddmerge.ManagedRegion{{
			SemanticID:   "region/agents/workflow",
			Start:        len("outside\n"),
			End:          len("outside\nmanaged: user\n"),
			RecordedBase: []byte("managed: old\n"),
			Generated:    []byte("managed: new\n"),
		}})
		if err != nil {
			t.Fatalf("Merge() error = %v", err)
		}
		if !bytes.Equal(result.Content, current) || len(result.Conflicts) != 1 {
			t.Fatalf("conflicting merge = %#v", result)
		}
		conflict := result.Conflicts[0]
		if string(conflict.RecordedBase) != "managed: old\n" || string(conflict.Current) != "managed: user\n" || string(conflict.Generated) != "managed: new\n" ||
			conflict.RecordedBaseRef == "" || conflict.CurrentRef == "" || conflict.GeneratedRef == "" {
			t.Fatalf("conflict evidence = %#v", conflict)
		}
	})

	t.Run("unknown ownership requires disclosed takeover", func(t *testing.T) {
		root := t.TempDir()
		writeTarget(t, root, "agents/implement.md", []byte("user-owned\n"), 0o600)
		plan, err := NewPlanner(root).Plan(PlanRequest{
			Bundle:  bundleWithAsset("agents/implement.md", "asset/agent/implement", []byte("generated\n")),
			Profile: "portable-sequential",
		})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if len(plan.Conflicts) != 1 || plan.Conflicts[0].State != OwnershipUnknown {
			t.Fatalf("unknown-ownership plan = %#v", plan)
		}
		inspection := Inspection{State: plan.Conflicts[0].State, Reason: plan.Conflicts[0].Reason}
		if err := AuthorizeReplacement(inspection, ReplacementRequest{Destructive: true}); !errors.Is(err, ErrUnknownOwnership) {
			t.Fatalf("undisclosed replacement error = %v", err)
		}
		if err := AuthorizeReplacement(inspection, ReplacementRequest{Destructive: true, Takeover: true}); !errors.Is(err, ErrTakeoverNotDisclosed) {
			t.Fatalf("takeover without disclosure error = %v", err)
		}
		if err := AuthorizeReplacement(inspection, ReplacementRequest{Destructive: true, Takeover: true, TakeoverDisclosed: true}); err != nil {
			t.Fatalf("disclosed takeover error = %v", err)
		}
	})

	t.Run("partial failure stops writes and restores the verified snapshot", func(t *testing.T) {
		root := t.TempDir()
		backupRoot := filepath.Join(t.TempDir(), "backups")
		writeTarget(t, root, "agents/a.md", []byte("old-a\n"), 0o600)
		writeTarget(t, root, "agents/b.md", []byte("old-b\n"), 0o600)
		plan := Plan{
			Updates: []Effect{
				{Path: "agents/a.md", SemanticID: "asset/agent/a", Content: []byte("new-a\n"), AfterMode: 0o600},
				{Path: "agents/b.md", SemanticID: "asset/agent/b", Content: []byte("new-b\n"), AfterMode: 0o600},
			},
			Backup: BackupScope{Required: true, Paths: []string{"agents/a.md", "agents/b.md"}},
		}
		applier := NewApplier(root, backupRoot)
		applier.beforeMutation = func(receipt Receipt, effect Effect) error {
			if effect.Path == "agents/b.md" {
				return errors.New("deterministic injected failure")
			}
			return nil
		}
		receipt, err := applier.Apply(plan)
		if err == nil || receipt.FailedPath != "agents/b.md" || !receipt.BackupVerified || !receipt.RestoreAvailable {
			t.Fatalf("partial apply receipt = %#v, error = %v", receipt, err)
		}
		assertTarget(t, root, "agents/a.md", "new-a\n")
		assertTarget(t, root, "agents/b.md", "old-b\n")

		result, err := Rollback(receipt, func() error {
			a, readErr := os.ReadFile(filepath.Join(root, "agents", "a.md"))
			if readErr != nil || !bytes.Equal(a, []byte("old-a\n")) {
				return errors.New("restored asset failed deterministic doctor")
			}
			return nil
		})
		if err != nil || !result.DoctorPassed || len(result.Conflicts) != 0 {
			t.Fatalf("Rollback() = %#v, %v", result, err)
		}
		assertTarget(t, root, "agents/a.md", "old-a\n")
		assertTarget(t, root, "agents/b.md", "old-b\n")
	})

	t.Run("unchanged reinstall creates no effects or backup", func(t *testing.T) {
		root := t.TempDir()
		backupRoot := filepath.Join(t.TempDir(), "backups")
		content := []byte("unchanged\n")
		writeTarget(t, root, "agents/validate.md", content, 0o600)
		plan, err := NewPlanner(root).Plan(PlanRequest{
			Bundle:  bundleWithAsset("agents/validate.md", "asset/agent/validate", content),
			Managed: []ManagedAsset{managedAsset(t, "agents/validate.md", "asset/agent/validate", content, 0o600)},
			Profile: "portable-sequential",
		})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if len(plan.Creates)+len(plan.Updates)+len(plan.Deletes) != 0 || plan.Backup.Required {
			t.Fatalf("no-op plan = %#v", plan)
		}
		receipt, err := NewApplier(root, backupRoot).Apply(plan)
		if err != nil || receipt.Backup.ID != "" {
			t.Fatalf("no-op Apply() = %#v, %v", receipt, err)
		}
		if _, err := os.Stat(backupRoot); !os.IsNotExist(err) {
			t.Fatalf("no-op reinstall created backup root: %v", err)
		}
	})

	t.Run("budget exhaustion remains inconclusive with partial evidence", func(t *testing.T) {
		_ = t.TempDir()
		outcome := quality.EvaluateActivity(
			quality.ActivityBudget{WallTime: time.Second, Cases: 10},
			quality.ActivityUsage{WallTime: time.Second + 1, Cases: 11},
			true,
			[]string{"install-safety-matrix.partial.json"},
		)
		if outcome.Status == quality.OutcomePass || outcome.Status != quality.OutcomeInconclusive || len(outcome.PartialEvidence) == 0 {
			t.Fatalf("exhausted matrix outcome = %#v", outcome)
		}
	})
}

func TestInstallationSafetyMatrixPreservesArbitraryOutsideBytes(t *testing.T) {
	_ = t.TempDir()
	rng := rand.New(rand.NewSource(0x70557))
	for caseIndex := 0; caseIndex < 256; caseIndex++ {
		prefix := make([]byte, rng.Intn(96))
		suffix := make([]byte, rng.Intn(96))
		_, _ = rng.Read(prefix)
		_, _ = rng.Read(suffix)
		base := []byte("managed-base")
		generated := []byte("managed-generated")
		current := append(append(append([]byte(nil), prefix...), base...), suffix...)

		result, err := sddmerge.Merge(current, []sddmerge.ManagedRegion{{
			SemanticID: "region/property", Start: len(prefix), End: len(prefix) + len(base), RecordedBase: base, Generated: generated,
		}})
		if err != nil {
			t.Fatalf("case %d: Merge() error = %v", caseIndex, err)
		}
		if len(result.Conflicts) != 0 || !bytes.Equal(result.Content[:len(prefix)], prefix) || !bytes.Equal(result.Content[len(result.Content)-len(suffix):], suffix) {
			t.Fatalf("case %d: outside-region bytes changed", caseIndex)
		}
	}
}
