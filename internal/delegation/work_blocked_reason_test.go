package delegation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func blockedReasonFixture(t *testing.T) (*Store, WorkClaimReservation) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	if _, err := s.CreateWorkInBoardWithDefinition(ctx, DefaultBoardID, "blocked-reason", "blocked reason", nil, WorkDefinition{Project: t.TempDir(), AllowedFiles: []string{"src/a.go"}}); err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWorkWithLeases(ctx, "blocked-reason", "implement-owner", []string{"src/a.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return s, claim
}

func TestBlockedReasonPersistsValidTaxonomy(t *testing.T) {
	ctx := context.Background()
	for _, reason := range []string{
		WorkBlockedReasonAuthorityExpired,
		WorkBlockedReasonUpstream,
		WorkBlockedReasonNeedsUser,
		WorkBlockedReasonEnv,
		WorkBlockedReasonScopeDrift,
		WorkBlockedReasonUnclassified,
	} {
		t.Run(reason, func(t *testing.T) {
			s, claim := blockedReasonFixture(t)
			item, err := s.TransitionWork(ctx, "blocked-reason", claim.Token, claim.Revision, WorkBlocked,
				WorkSubmissionInput{Summary: "blocked", BlockedReason: reason})
			if err != nil {
				t.Fatalf("blocked transition: %v", err)
			}
			if item.Status != WorkBlocked || item.BlockedReason != reason {
				t.Fatalf("work_items blocked_reason = %q, want %q", item.BlockedReason, reason)
			}
			var stored sql.NullString
			if err := s.db.QueryRow(`SELECT blocked_reason FROM work_submissions WHERE item_id='blocked-reason'`).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if !stored.Valid || stored.String != reason {
				t.Fatalf("submission blocked_reason = %v, want %q", stored, reason)
			}
			var detail string
			if err := s.db.QueryRow(`SELECT detail FROM work_events WHERE item_id='blocked-reason' AND kind='transition' ORDER BY id DESC LIMIT 1`).Scan(&detail); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(detail, "blocked_reason="+reason) {
				t.Fatalf("event detail %q missing blocked_reason", detail)
			}
		})
	}
}

func TestBlockedReasonRejectsUnknownValueAtomically(t *testing.T) {
	s, claim := blockedReasonFixture(t)
	ctx := context.Background()
	_, err := s.TransitionWork(ctx, "blocked-reason", claim.Token, claim.Revision, WorkBlocked,
		WorkSubmissionInput{Summary: "blocked", BlockedReason: "not_a_class"})
	if !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("error = %v, want ErrWorkConflict", err)
	}
	item, err := s.GetWork(ctx, "blocked-reason")
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != WorkInProgress || item.Revision != claim.Revision || len(item.Leases) != 1 || item.Submission != nil || item.BlockedReason != "" {
		t.Fatalf("invalid reason partially committed: %+v", item)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM work_submissions WHERE item_id='blocked-reason'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("submission rows = %d, err = %v", count, err)
	}
}

func TestBlockedReasonClearsWhenLeavingBlocked(t *testing.T) {
	s, claim := blockedReasonFixture(t)
	ctx := context.Background()
	blocked, err := s.TransitionWork(ctx, "blocked-reason", claim.Token, claim.Revision, WorkBlocked,
		WorkSubmissionInput{Summary: "blocked", BlockedReason: WorkBlockedReasonUpstream})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := s.RetryWork(ctx, "blocked-reason", blocked.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != WorkReady || ready.BlockedReason != "" {
		t.Fatalf("retry left blocked_reason = %q", ready.BlockedReason)
	}
	var stored sql.NullString
	if err := s.db.QueryRow(`SELECT blocked_reason FROM work_items WHERE id='blocked-reason'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Fatalf("work_items.blocked_reason = %q, want NULL", stored.String)
	}
}

func TestBlockedReasonLegacyNullIsUnclassified(t *testing.T) {
	s, claim := blockedReasonFixture(t)
	ctx := context.Background()
	blocked, err := s.TransitionWork(ctx, "blocked-reason", claim.Token, claim.Revision, WorkBlocked, WorkSubmissionInput{Summary: "legacy block"})
	if err != nil {
		t.Fatalf("legacy blocked transition: %v", err)
	}
	if blocked.Status != WorkBlocked || blocked.BlockedReason != "" {
		t.Fatalf("legacy blocked reason = %q, want empty", blocked.BlockedReason)
	}
	var stored sql.NullString
	if err := s.db.QueryRow(`SELECT blocked_reason FROM work_items WHERE id='blocked-reason'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Fatalf("legacy reason stored = %q, want NULL", stored.String)
	}
	if got := WorkBlockedReasonClass(""); got != WorkBlockedReasonUnclassified {
		t.Fatalf("class of NULL = %q, want %q", got, WorkBlockedReasonUnclassified)
	}
	if got := WorkBlockedReasonClass(WorkBlockedReasonNeedsUser); got != WorkBlockedReasonNeedsUser {
		t.Fatalf("class of needs_user = %q", got)
	}
	if _, err := s.db.Exec(`UPDATE work_items SET blocked_reason=NULL WHERE id='blocked-reason'`); err != nil {
		t.Fatal(err)
	}
	legacy, err := s.GetWork(ctx, "blocked-reason")
	if err != nil || legacy.BlockedReason != "" {
		t.Fatalf("legacy row = %q, err = %v", legacy.BlockedReason, err)
	}
}
