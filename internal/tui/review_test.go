package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// openReview presses Home → Install/Sync and drains the returned plan
// command, so the model sits on a loaded Review screen.
func openReview(t *testing.T, svc ServiceAPI) model {
	t.Helper()
	m := sized(newModel(svc, "/home/test", "vtest"))
	m = pressDrive(t, m, "enter")
	if m.screen != screenReview {
		t.Fatalf("expected review screen, got %v", m.screen)
	}
	return m
}

// TestReviewDefaultsProvesCortexAndForgeSpecSelected verifies the default
// selection reaches the service and renders: Cortex+ForgeSpec on, Context7
// optional and off.
func TestReviewDefaultsProvesCortexAndForgeSpecSelected(t *testing.T) {
	fake := &fakeService{}
	m := openReview(t, fake)

	if len(fake.planCalls) != 1 {
		t.Fatalf("expected one plan call, got %d", len(fake.planCalls))
	}
	opts := fake.planCalls[0]
	if !opts.Cortex || !opts.ForgeSpec || opts.Context7 {
		t.Fatalf("expected default cortex+forgespec without context7, got %+v", opts)
	}
	if !opts.DryRun {
		t.Fatal("review planning must be read-only (DryRun)")
	}

	view := m.View()
	for _, want := range []string{"[x] cortex", "[x] forgespec", "[ ] context7"} {
		if !strings.Contains(view, want) {
			t.Fatalf("review view missing toggle %q:\n%s", want, view)
		}
	}
}

// TestReviewToggleReplansAndUpdatesSelection proves the space toggle changes
// the selection and immediately replans with it.
func TestReviewToggleReplansAndUpdatesSelection(t *testing.T) {
	fake := &fakeService{}
	m := openReview(t, fake)

	m = press(m, "down")
	m = press(m, "down") // cursor on context7
	m = pressDrive(t, m, "space")

	if len(fake.planCalls) != 2 {
		t.Fatalf("expected a second plan call after toggle, got %d", len(fake.planCalls))
	}
	if !fake.planCalls[1].Context7 {
		t.Fatalf("expected context7 selected after toggle, got %+v", fake.planCalls[1])
	}
	view := m.View()
	if !strings.Contains(view, "[x] context7") {
		t.Fatalf("expected context7 rendered selected:\n%s", view)
	}
}

// TestReviewShowsEffectsAndConflicts proves the plan's effects and conflicts
// are visible and that unresolved conflicts block execution.
func TestReviewShowsEffectsAndConflicts(t *testing.T) {
	fake := &fakeService{}
	plan := &pipeline.Plan{
		Digest: "digest0001",
		Effects: []pipeline.Effect{
			{Kind: pipeline.EffectCreate, Dest: ".config/opencode/AGENTS.md"},
			{Kind: pipeline.EffectNoop, Dest: ".config/opencode/commands/plan.md"},
		},
		Conflicts: []pipeline.Conflict{{
			Target: ".config/opencode/opencode.json",
			Kind:   pipeline.ConflictUnmanagedExisting,
			Reason: "existing file is not owned by cortex-ia",
		}},
	}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) { return plan, nil }

	m := openReview(t, fake)
	view := m.View()
	for _, want := range []string{
		"create .config/opencode/AGENTS.md",
		"plus 1 already converged",
		".config/opencode/opencode.json",
		"unmanaged-existing",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("review view missing %q:\n%s", want, view)
		}
	}

	// Conflicts block: enter does not start anything.
	m = press(m, "enter")
	if m.screen != screenReview || m.confirm.kind != confirmNone {
		t.Fatalf("conflicts must block execution, got screen=%v confirm=%v", m.screen, m.confirm.kind)
	}
	if len(fake.installCalls) != 0 {
		t.Fatal("no install may run while conflicts block the plan")
	}
}

// TestReviewOverwriteRequiresExplicitConfirmation proves the destructive
// overwrite path: authorize, then confirm; declining cancels without calls.
func TestReviewOverwriteRequiresExplicitConfirmation(t *testing.T) {
	fake := &fakeService{}
	withConflicts := &pipeline.Plan{
		Digest: "digest0001",
		Conflicts: []pipeline.Conflict{{
			Target: ".config/opencode/agents/review.md",
			Kind:   pipeline.ConflictUnmanagedExisting,
			Reason: "existing file is not owned by cortex-ia",
		}},
	}
	cleared := &pipeline.Plan{Digest: "digest0002", Effects: []pipeline.Effect{
		{Kind: pipeline.EffectOverwrite, Dest: ".config/opencode/agents/review.md"},
	}}
	fake.planFn = func(opts install.Options) (*pipeline.Plan, error) {
		if opts.Overwrite {
			return cleared, nil
		}
		return withConflicts, nil
	}
	fake.installFn = func(opts install.Options) (*install.InstallReceipt, error) {
		if !opts.Overwrite {
			t.Fatalf("install must only run with confirmed overwrite, got %+v", opts)
		}
		return &install.InstallReceipt{PlanDigest: "digest0002", BackupID: "bkp-ow"}, nil
	}

	m := openReview(t, fake)

	// Without authorization, enter is inert.
	m = press(m, "enter")
	if m.screen != screenReview || m.confirm.kind != confirmNone {
		t.Fatal("enter must be inert while unauthorized conflicts block")
	}

	// Authorize overwrite: replans with Overwrite=true.
	m = pressDrive(t, m, "o")
	if len(fake.planCalls) < 2 || !fake.planCalls[len(fake.planCalls)-1].Overwrite {
		t.Fatalf("expected replan with overwrite, got %+v", fake.planCalls)
	}

	// Enter now demands an explicit confirmation.
	m = press(m, "enter")
	if m.confirm.kind != confirmOverwrite {
		t.Fatalf("expected overwrite confirmation, got %v", m.confirm.kind)
	}
	view := m.View()
	if !strings.Contains(view, "Confirm overwrite") {
		t.Fatalf("confirmation overlay not visible:\n%s", view)
	}

	// Decline: nothing runs.
	m = press(m, "n")
	if m.confirm.kind != confirmNone || len(fake.installCalls) != 0 {
		t.Fatal("declining must cancel without calling install")
	}

	// Re-enter and confirm: install runs with overwrite and reaches running.
	m = press(m, "enter")
	m = pressDrive(t, m, "y")
	if len(fake.installCalls) != 1 {
		t.Fatalf("expected exactly one confirmed install, got %d", len(fake.installCalls))
	}
	if !fake.installCalls[0].Overwrite {
		t.Fatal("confirmed install must carry overwrite authorization")
	}
	if m.screen != screenResult {
		t.Fatalf("expected result screen after confirmed install, got %v", m.screen)
	}
	if !m.result.pass || m.result.backupID != "bkp-ow" {
		t.Fatalf("expected PASS with backup bkp-ow, got %+v", m.result)
	}
}

// TestReviewUnclearableConflictsStayBlocking proves conflicts that overwrite
// cannot clear (malformed config) never become runnable.
func TestReviewUnclearableConflictsStayBlocking(t *testing.T) {
	fake := &fakeService{}
	plan := &pipeline.Plan{
		Digest: "digest0001",
		Conflicts: []pipeline.Conflict{{
			Target: ".config/opencode/opencode.jsonc",
			Kind:   pipeline.ConflictMalformedConfig,
			Reason: "not a JSON object",
		}},
	}
	fake.planFn = func(opts install.Options) (*pipeline.Plan, error) { return plan, nil }

	m := openReview(t, fake)
	view := m.View()
	if !strings.Contains(view, "overwrite cannot clear this") {
		t.Fatalf("expected unclearable-conflict hint:\n%s", view)
	}
	m = pressDrive(t, m, "o") // toggling is allowed…
	m = press(m, "enter")     // …but conflicts still block
	if m.screen != screenReview || m.confirm.kind != confirmNone {
		t.Fatalf("unclearable conflicts must keep blocking, got screen=%v confirm=%v", m.screen, m.confirm.kind)
	}
}

// TestReviewModeFollowsInstallationPresence proves Review labels and runs
// sync for an installed (v2) home and install otherwise.
func TestReviewModeFollowsInstallationPresence(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "d", MetadataPresence: state.PresenceV2}, nil
	}
	fake.syncFn = func(install.Options) (*install.InstallReceipt, error) {
		return &install.InstallReceipt{PlanDigest: "d"}, nil
	}
	m := openReview(t, fake)
	if m.installMode != "sync" || !strings.Contains(m.View(), "Review — Sync") {
		t.Fatalf("expected sync mode for installed home, got %q", m.installMode)
	}
	m = pressDrive(t, m, "enter")
	if len(fake.syncCalls) != 1 || len(fake.installCalls) != 0 {
		t.Fatalf("installed home must sync: sync=%d install=%d", len(fake.syncCalls), len(fake.installCalls))
	}

	fake2 := &fakeService{}
	fake2.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "d", MetadataPresence: state.PresenceAbsent}, nil
	}
	m2 := openReview(t, fake2)
	if m2.installMode != "install" {
		t.Fatalf("expected install mode for fresh home, got %q", m2.installMode)
	}
}

// TestReviewPlanErrorShown proves planning failures surface on the review
// screen and block execution.
func TestReviewPlanErrorShown(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return nil, errors.New("home root is not writable")
	}
	m := openReview(t, fake)
	if !strings.Contains(m.View(), "home root is not writable") {
		t.Fatalf("expected plan error rendered:\n%s", m.View())
	}
	m = press(m, "enter")
	if m.screen != screenReview || len(fake.installCalls) != 0 {
		t.Fatal("plan errors must block execution")
	}
}

// TestReviewConfirmedRunBindsPlanDigest proves every mutating install/sync
// call carries the digest of the plan the user reviewed: the service
// re-plans and rejects drift before any write.
func TestReviewConfirmedRunBindsPlanDigest(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001", MetadataPresence: state.PresenceAbsent}, nil
	}
	fake.installFn = func(opts install.Options) (*install.InstallReceipt, error) {
		if opts.ExpectedPlanDigest != "digest0001" {
			t.Fatalf("install must be digest-bound, got %q", opts.ExpectedPlanDigest)
		}
		if opts.DryRun {
			t.Fatal("confirmed run must not be a dry run")
		}
		return &install.InstallReceipt{PlanDigest: "digest0001"}, nil
	}
	m := openReview(t, fake)
	pressDrive(t, m, "enter")
	if len(fake.installCalls) != 1 {
		t.Fatalf("expected one digest-bound install, got %d", len(fake.installCalls))
	}

	fakeSync := &fakeService{}
	fakeSync.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "syncdigest01", MetadataPresence: state.PresenceV2}, nil
	}
	fakeSync.syncFn = func(opts install.Options) (*install.InstallReceipt, error) {
		if opts.ExpectedPlanDigest != "syncdigest01" {
			t.Fatalf("sync must be digest-bound, got %q", opts.ExpectedPlanDigest)
		}
		return &install.InstallReceipt{PlanDigest: "syncdigest01"}, nil
	}
	mSync := openReview(t, fakeSync)
	pressDrive(t, mSync, "enter")
	if len(fakeSync.syncCalls) != 1 {
		t.Fatalf("expected one digest-bound sync, got %d", len(fakeSync.syncCalls))
	}
}

// TestReviewOverwriteConfirmBindsReplannedDigest proves new overwrite
// effects never run bound to a stale plan: after authorizing overwrite the
// review replans, and only the replanned digest — the one whose overwrite
// effects were displayed — reaches the service.
func TestReviewOverwriteConfirmBindsReplannedDigest(t *testing.T) {
	fake := &fakeService{}
	withConflicts := &pipeline.Plan{
		Digest: "staledigest0",
		Conflicts: []pipeline.Conflict{{
			Target:              ".config/opencode/agents/review.md",
			Kind:                pipeline.ConflictUnmanagedExisting,
			Reason:              "existing file is not owned by cortex-ia",
			OverwriteAuthorized: true,
		}},
	}
	cleared := &pipeline.Plan{Digest: "freshdigest1", Effects: []pipeline.Effect{
		{Kind: pipeline.EffectOverwrite, Dest: ".config/opencode/agents/review.md"},
	}}
	fake.planFn = func(opts install.Options) (*pipeline.Plan, error) {
		if opts.Overwrite {
			return cleared, nil
		}
		return withConflicts, nil
	}
	fake.installFn = func(opts install.Options) (*install.InstallReceipt, error) {
		if opts.ExpectedPlanDigest != "freshdigest1" {
			t.Fatalf("confirmed overwrite must bind the replanned digest, got %q", opts.ExpectedPlanDigest)
		}
		return &install.InstallReceipt{PlanDigest: "freshdigest1", BackupID: "bkp-digest"}, nil
	}

	m := openReview(t, fake)
	m = pressDrive(t, m, "o") // authorize → replan → fresh digest displayed
	m = press(m, "enter")
	if m.confirm.kind != confirmOverwrite {
		t.Fatalf("expected overwrite confirmation, got %v", m.confirm.kind)
	}
	if !strings.Contains(m.View(), "freshdigest1"[:12]) {
		t.Fatalf("confirmation should show the bound plan digest:\n%s", m.View())
	}
	m = pressDrive(t, m, "y")
	if len(fake.installCalls) != 1 {
		t.Fatalf("expected exactly one digest-bound overwrite install, got %d", len(fake.installCalls))
	}
}

// TestReviewLowHeightClipsConflictsKeepsSafeAction proves a 20-row
// terminal with many conflicts still renders the header, the safe-action
// hints, and the help line, folds overflow behind a "+N more" summary,
// and lets pgup/pgdn reveal folded conflicts without exceeding the budget.
func TestReviewLowHeightClipsConflictsKeepsSafeAction(t *testing.T) {
	fake := &fakeService{}
	plan := &pipeline.Plan{Digest: "digest0001"}
	for i := 0; i < 15; i++ {
		plan.Conflicts = append(plan.Conflicts, pipeline.Conflict{
			Target:              fmt.Sprintf(".config/opencode/conflict-%d.md", i),
			Kind:                pipeline.ConflictUnmanagedExisting,
			Reason:              "existing file is not owned by cortex-ia",
			OverwriteAuthorized: true,
		})
	}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) { return plan, nil }

	m := openReview(t, fake)
	m.height = 20
	view := m.View()
	if got := viewLines(m); got > 20 {
		t.Fatalf("review must fit a 20-row terminal, got %d rows:\n%s", got, view)
	}
	for _, want := range []string{
		"Conflicts (15)",
		"+", "more",
		"[o] authorize overwrite",
		"esc home",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("low-height review missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "conflict-14") {
		t.Fatalf("folded conflict must not render unscrolled:\n%s", view)
	}

	// Scrolling reveals folded conflicts while staying within budget.
	for i := 0; i < 10; i++ {
		m = press(m, "pgdown")
	}
	view = m.View()
	if !strings.Contains(view, "conflict-14") {
		t.Fatalf("scrolled review must reveal the last conflict:\n%s", view)
	}
	if got := viewLines(m); got > 20 {
		t.Fatalf("scrolled review must still fit 20 rows, got %d:\n%s", got, view)
	}
	if !strings.Contains(view, "esc home") {
		t.Fatalf("help line must survive scrolling:\n%s", view)
	}
}

// TestReviewConfirmOverlayFitsLowHeight proves the destructive-action
// confirmation stays fully visible on a 20-row terminal with many
// conflicts: the safe choice and the bound digest remain on screen.
func TestReviewConfirmOverlayFitsLowHeight(t *testing.T) {
	fake := &fakeService{}
	plan := &pipeline.Plan{Digest: "lowheightdg1"}
	for i := 0; i < 15; i++ {
		plan.Conflicts = append(plan.Conflicts, pipeline.Conflict{
			Target:              fmt.Sprintf(".config/opencode/conflict-%d.md", i),
			Kind:                pipeline.ConflictUnmanagedExisting,
			Reason:              "existing file is not owned by cortex-ia",
			OverwriteAuthorized: true,
		})
	}
	cleared := &pipeline.Plan{Digest: "lowheightdg1", Effects: []pipeline.Effect{
		{Kind: pipeline.EffectOverwrite, Dest: ".config/opencode/conflict-0.md"},
	}}
	fake.planFn = func(opts install.Options) (*pipeline.Plan, error) {
		if opts.Overwrite {
			return cleared, nil
		}
		return plan, nil
	}

	m := openReview(t, fake)
	m.height = 20
	m = pressDrive(t, m, "o")
	m = press(m, "enter")
	if m.confirm.kind != confirmOverwrite {
		t.Fatalf("expected overwrite confirmation, got %v", m.confirm.kind)
	}
	view := m.View()
	if got := viewLines(m); got > 20 {
		t.Fatalf("confirmation must fit a 20-row terminal, got %d rows:\n%s", got, view)
	}
	for _, want := range []string{"Confirm overwrite", "[y] yes, proceed", "[n]/esc no, cancel", "lowheightdg1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("low-height confirmation missing %q:\n%s", want, view)
		}
	}
}
