package delegation

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func submissionFixture(t *testing.T) (*Store, WorkClaimReservation) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.CreateWorkInBoardWithDefinition(context.Background(), DefaultBoardID, "delivery", "delivery", nil, WorkDefinition{Project: t.TempDir(), AllowedFiles: []string{"src/a.go"}})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWorkWithLeases(context.Background(), "delivery", "implement-owner", []string{"src/a.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return s, claim
}

func TestWorkSubmissionDeliveryAndIndependentApproval(t *testing.T) {
	s, claim := submissionFixture(t)
	ctx := context.Background()
	verdict := "pass"
	input := WorkSubmissionInput{Summary: "verified synthetic change", Verdict: &verdict, EvidenceRefs: []string{"go test synthetic"}, ChangedFiles: []string{"src/./a.go"}}
	item, err := s.TransitionWork(ctx, "delivery", claim.Token, claim.Revision, WorkInReview, input)
	if err != nil {
		t.Fatal(err)
	}
	receipt := item.Submission
	if item.Status != WorkInReview || len(item.Leases) != 0 || receipt == nil || receipt.Attempt != claim.Attempt || receipt.ImplementationOwner != claim.Owner || receipt.ReviewID != item.Review.ReviewID || receipt.TransitionRevision != item.Revision {
		t.Fatalf("missing attributable atomic delivery: %+v", item)
	}
	if receipt.Summary != input.Summary || *receipt.Verdict != "PASS" || !reflect.DeepEqual(receipt.EvidenceRefs, input.EvidenceRefs) || !reflect.DeepEqual(receipt.ChangedFiles, []string{"src/a.go"}) || input.ChangedFiles[0] != "src/./a.go" || verdict != "pass" {
		t.Fatal("receipt lost content or mutated caller input")
	}
	if _, err := s.ApproveWork(ctx, "delivery", claim.Owner, "PASS", "test", item.Revision); err == nil {
		t.Fatal("self approval accepted")
	}
	for _, statement := range []string{"UPDATE work_submissions SET summary='changed'", "DELETE FROM work_submissions"} {
		if _, err := s.db.ExecContext(ctx, statement); err == nil {
			t.Fatal("submission was mutable")
		}
	}
	approval, err := s.ApproveWork(ctx, "delivery", "independent-reviewer", "PASS", "independent check", item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if approval.SubmissionID != receipt.ID {
		t.Fatal("approval lost submission attribution")
	}
	again, err := s.GetWork(ctx, "delivery")
	if err != nil || again.Status != WorkDone || again.Submission.ID != receipt.ID || again.LatestApproval.SubmissionID != receipt.ID {
		t.Fatalf("persisted result: %v", err)
	}
	approvals, err := s.ListWorkApprovals(ctx, "delivery")
	if err != nil || len(approvals) != 1 || approvals[0].SubmissionID != receipt.ID {
		t.Fatal("history lost linkage")
	}
}

func TestWorkSubmissionRejectsWithoutPartialEffects(t *testing.T) {
	for _, scenario := range []string{"token", "revision", "expiry", "path", "summary", "references", "release"} {
		t.Run(scenario, func(t *testing.T) {
			s, claim := submissionFixture(t)
			ctx := context.Background()
			token, revision := claim.Token, claim.Revision
			input := WorkSubmissionInput{Summary: "test", ChangedFiles: []string{"src/a.go"}}
			switch scenario {
			case "token":
				token = "not-authority"
			case "revision":
				revision++
			case "expiry":
				s.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
			case "path":
				input.ChangedFiles = []string{"unleased.go"}
			case "summary":
				input.Summary = strings.Repeat("a", 8193)
			case "references":
				input.EvidenceRefs = []string{strings.Repeat("a", 1025)}
			case "release":
				if _, err := s.db.Exec(`CREATE TRIGGER reject_release BEFORE DELETE ON work_leases BEGIN SELECT RAISE(ABORT,'synthetic release failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.TransitionWork(ctx, "delivery", token, revision, WorkInReview, input); err == nil {
				t.Fatal("invalid submission accepted")
			}
			item, err := s.GetWork(ctx, "delivery")
			if err != nil {
				t.Fatal(err)
			}
			if item.Status != WorkInProgress || item.Revision != claim.Revision || len(item.Leases) != 1 || item.Submission != nil || item.Review != nil {
				t.Fatal("failed delivery partially committed")
			}
			var count int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM work_submissions`).Scan(&count); err != nil || count != 0 {
				t.Fatal("failed delivery retained receipt")
			}
		})
	}
}

func TestWorkSubmissionRetryAndLegacyTransition(t *testing.T) {
	s, claim := submissionFixture(t)
	ctx := context.Background()
	blocked, err := s.TransitionWork(ctx, "delivery", claim.Token, claim.Revision, WorkBlocked, WorkSubmissionInput{Summary: "first attempt blocked"})
	if err != nil || blocked.Submission == nil || blocked.Submission.Verdict != nil || blocked.Claim != nil || len(blocked.Leases) != 0 {
		t.Fatalf("blocked submission: %v", err)
	}
	ready, err := s.RetryWork(ctx, "delivery", blocked.Revision)
	if err != nil || ready.Submission != nil {
		t.Fatal("retry projected previous receipt")
	}
	next, err := s.ClaimWorkWithLeases(ctx, "delivery", "second-owner", []string{"src/a.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.GetWork(ctx, "delivery")
	if err != nil || item.Submission != nil {
		t.Fatal("new attempt projected stale receipt")
	}
	item, err = s.TransitionWork(ctx, "delivery", next.Token, next.Revision, WorkInReview)
	if err != nil || item.Submission != nil || len(item.Leases) != 0 {
		t.Fatalf("legacy transition: %v", err)
	}
	approval, err := s.ApproveWork(ctx, "delivery", "reviewer", "PASS", "legacy evidence", item.Revision)
	if err != nil || approval.SubmissionID != "" {
		t.Fatal("fabricated legacy submission")
	}
}
