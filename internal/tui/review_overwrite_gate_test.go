package tui

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// TestReviewOverwriteGateRefusesNonClearableConflict proves that when the plan
// holds only conflicts an overwrite can never clear (an MCP ownership
// conflict), the 'o' key no longer toggles a dead overwrite flag: it keeps the
// authorization off, omits the footer hint, and explains the refusal.
func TestReviewOverwriteGateRefusesNonClearableConflict(t *testing.T) {
	fake := &fakeService{}
	plan := &pipeline.Plan{
		Digest: "digest-mcp1",
		Conflicts: []pipeline.Conflict{{
			Target: "cortex",
			Kind:   pipeline.ConflictMCP,
			Reason: "server is not owned by cortex-ia",
		}},
	}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) { return plan, nil }

	m := openReview(t, fake)
	view := m.View()
	if !strings.Contains(view, "manual resolution required") || !strings.Contains(view, "mcp-conflict") {
		t.Fatalf("expected manual-resolution hint naming mcp-conflict:\n%s", view)
	}
	if strings.Contains(view, "b back to wizard · o overwrite") {
		t.Fatalf("footer must omit the overwrite hint when nothing is overwritable:\n%s", view)
	}

	m = pressDrive(t, m, "o")
	if m.overwrite {
		t.Fatal("'o' must not authorize overwrite when no conflict is clearable")
	}
	if m.hadConflict {
		t.Fatal("a non-clearable conflict must not set the sticky conflict hint")
	}
	if m.reviewStatus == "" || !strings.Contains(m.reviewStatus, "overwrite cannot clear mcp-conflict") {
		t.Fatalf("expected explanatory refusal status, got %q", m.reviewStatus)
	}
	if view := m.View(); strings.Contains(view, "b back to wizard · o overwrite") {
		t.Fatalf("footer must stay hint-free after the refused toggle:\n%s", view)
	}

	m = press(m, "enter")
	if m.screen != screenReview || m.confirm.kind != confirmNone {
		t.Fatalf("mcp conflict must keep blocking, got screen=%v confirm=%v", m.screen, m.confirm.kind)
	}
	if len(fake.installCalls) != 0 {
		t.Fatal("no install may run while an mcp conflict blocks")
	}
}

// TestReviewOverwriteGateAuthorizesClearableConflict proves the overwrite path
// stays intact: a file-ownership conflict lets 'o' authorize the overwrite,
// replan with the authorization, and advertise the resulting state.
func TestReviewOverwriteGateAuthorizesClearableConflict(t *testing.T) {
	fake := &fakeService{}
	withConflict := &pipeline.Plan{
		Digest: "digest-file1",
		Conflicts: []pipeline.Conflict{{
			Target:              ".config/opencode/agents/review.md",
			Kind:                pipeline.ConflictUnmanagedExisting,
			Reason:              "existing file is not owned by cortex-ia",
			OverwriteAuthorized: true,
		}},
	}
	cleared := &pipeline.Plan{Digest: "digest-file2", Effects: []pipeline.Effect{
		{Kind: pipeline.EffectOverwrite, Dest: ".config/opencode/agents/review.md"},
	}}
	fake.planFn = func(opts install.Options) (*pipeline.Plan, error) {
		if opts.Overwrite {
			return cleared, nil
		}
		return withConflict, nil
	}

	m := openReview(t, fake)
	view := m.View()
	for _, want := range []string{"b back to wizard · o overwrite", "[o] authorize overwrite"} {
		if !strings.Contains(view, want) {
			t.Fatalf("clearable conflict must offer overwrite %q:\n%s", want, view)
		}
	}

	m = pressDrive(t, m, "o")
	if !m.overwrite {
		t.Fatal("'o' must authorize overwrite for a clearable conflict")
	}
	if calls := fake.planCalls; len(calls) < 2 || !calls[len(calls)-1].Overwrite {
		t.Fatalf("expected a replan with Overwrite=true, got %+v", calls)
	}
	if view := m.View(); !strings.Contains(view, "overwrite authorized") {
		t.Fatalf("authorized overwrite must be advertised:\n%s", view)
	}
}
