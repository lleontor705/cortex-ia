package delegation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWorkProjectionRead(t *testing.T) {
	ctx := context.Background()
	store, _ := newReviseTestStore(t)

	_, err := store.CreateBoard(ctx, "board-proj", "Test Board", "Description")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}

	t.Run("read_not_found", func(t *testing.T) {
		proj, err := store.ReadWorkProjection(ctx, "nonexistent-task", "implement", "actor", nil)
		if !errors.Is(err, ErrWorkNotFound) {
			t.Fatalf("expected ErrWorkNotFound, got %v", err)
		}
		if proj.AuthorityAvailable {
			t.Fatalf("expected AuthorityAvailable to be false for missing task")
		}
	})

	t.Run("read_dependencies_and_lifecycle", func(t *testing.T) {
		_, err := store.CreateWorkInBoard(ctx, "board-proj", "dep-task", "Dependency Task", nil)
		if err != nil {
			t.Fatalf("create dep task: %v", err)
		}
		mainItem, err := store.CreateWorkInBoard(ctx, "board-proj", "main-task", "Main Task", []string{"dep-task"})
		if err != nil {
			t.Fatalf("create main task: %v", err)
		}

		// 1. Dependency unsatisfied (dep is ready/backlog, not done)
		proj1, err := store.ReadWorkProjection(ctx, mainItem.ID, "implement", "agent-1", nil)
		if err != nil {
			t.Fatalf("read projection 1: %v", err)
		}
		if len(proj1.Blockers) == 0 || proj1.Blockers[0].Code != BlockerUnsatisfiedDeps {
			t.Fatalf("expected BlockerUnsatisfiedDeps, got %+v", proj1.Blockers)
		}
		if len(proj1.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions when deps unsatisfied, got %+v", proj1.CandidateActions)
		}

		// 2. Mark dep as done and main-task as ready
		if _, err := store.db.ExecContext(ctx, `UPDATE work_items SET status='done' WHERE id='dep-task'`); err != nil {
			t.Fatalf("mark dep done: %v", err)
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE work_items SET status='ready' WHERE id='main-task'`); err != nil {
			t.Fatalf("mark main ready: %v", err)
		}

		proj2, err := store.ReadWorkProjection(ctx, mainItem.ID, "implement", "agent-1", nil)
		if err != nil {
			t.Fatalf("read projection 2: %v", err)
		}
		if len(proj2.Blockers) != 0 {
			t.Fatalf("expected 0 blockers after dep done, got %+v", proj2.Blockers)
		}
		if len(proj2.CandidateActions) != 1 || proj2.CandidateActions[0].Action != "request_claim" {
			t.Fatalf("expected request_claim, got %+v", proj2.CandidateActions)
		}
	})

	t.Run("read_claim_active_and_expired", func(t *testing.T) {
		item, err := store.CreateWorkInBoard(ctx, "board-proj", "claim-task", "Claim Task", nil)
		if err != nil {
			t.Fatalf("create claim task: %v", err)
		}

		claim, err := store.ClaimWork(ctx, item.ID, "worker-alpha", 1*time.Hour)
		if err != nil {
			t.Fatalf("claim work: %v", err)
		}

		now := time.Now().UTC()

		// Active claim
		projActive, err := store.ReadWorkProjectionAt(ctx, item.ID, "implement", "worker-alpha", nil, now)
		if err != nil {
			t.Fatalf("read active projection: %v", err)
		}
		if projActive.Status != WorkInProgress {
			t.Fatalf("expected in_progress status, got %s", projActive.Status)
		}
		if len(projActive.CandidateActions) != 2 {
			t.Fatalf("expected 2 actions (transition_in_review, renew_claim), got %+v", projActive.CandidateActions)
		}

		// Expired claim
		future := now.Add(2 * time.Hour)
		projExpired, err := store.ReadWorkProjectionAt(ctx, item.ID, "implement", "worker-alpha", nil, future)
		if err != nil {
			t.Fatalf("read expired projection: %v", err)
		}
		if len(projExpired.Blockers) == 0 || projExpired.Blockers[0].Code != BlockerClaimExpired {
			t.Fatalf("expected BlockerClaimExpired, got %+v", projExpired.Blockers)
		}
		if len(projExpired.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for expired implementer, got %+v", projExpired.CandidateActions)
		}

		_ = claim
	})

	t.Run("read_in_review_self_approval_and_superseded", func(t *testing.T) {
		item, err := store.CreateWorkInBoard(ctx, "board-proj", "review-task", "Review Task", nil)
		if err != nil {
			t.Fatalf("create review task: %v", err)
		}

		_, err = store.ClaimWork(ctx, item.ID, "dev-carol", 10*time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		// Transition to in_review
		if _, err := store.db.ExecContext(ctx, `UPDATE work_items SET status='in_review' WHERE id=?`, item.ID); err != nil {
			t.Fatalf("update in_review: %v", err)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO work_reviews(item_id,review_id,attempt,implementation_owner,review_revision,created_at,binding_json) VALUES(?,?,?,?,?,?,?)`,
			item.ID, "rev-123", 1, "dev-carol", 2, time.Now().UTC().Format(time.RFC3339), "{}"); err != nil {
			t.Fatalf("insert review: %v", err)
		}

		// Self-approval check
		projSelf, err := store.ReadWorkProjection(ctx, item.ID, "reviewer", "dev-carol", nil)
		if err != nil {
			t.Fatalf("read self projection: %v", err)
		}
		if len(projSelf.Blockers) == 0 || projSelf.Blockers[0].Code != BlockerSelfReview {
			t.Fatalf("expected BlockerSelfReview, got %+v", projSelf.Blockers)
		}
		if len(projSelf.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for self review, got %+v", projSelf.CandidateActions)
		}

		// Independent review
		projIndep, err := store.ReadWorkProjection(ctx, item.ID, "reviewer", "dev-dave", nil)
		if err != nil {
			t.Fatalf("read indep projection: %v", err)
		}
		if len(projIndep.CandidateActions) != 1 || projIndep.CandidateActions[0].Action != "independent_review" {
			t.Fatalf("expected independent_review action, got %+v", projIndep.CandidateActions)
		}

		// Superseded
		_, err = store.CreateWorkInBoard(ctx, "board-proj", "child-1", "Child 1", nil)
		if err != nil {
			t.Fatalf("create child-1: %v", err)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO work_decomposition_steps(parent_id,child_id,position) VALUES(?,?,?)`, item.ID, "child-1", 1); err != nil {
			t.Fatalf("insert decomp step: %v", err)
		}
		projSuper, err := store.ReadWorkProjection(ctx, item.ID, "reviewer", "dev-dave", nil)
		if err != nil {
			t.Fatalf("read super projection: %v", err)
		}
		if projSuper.Status != WorkSuperseded {
			t.Fatalf("expected WorkSuperseded, got %s", projSuper.Status)
		}
	})

	t.Run("read_concurrent_coherent", func(t *testing.T) {
		item, err := store.CreateWorkInBoard(ctx, "board-proj", "conc-task", "Concurrent Task", nil)
		if err != nil {
			t.Fatalf("create conc task: %v", err)
		}

		var wg sync.WaitGroup
		errCh := make(chan error, 20)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				proj, err := store.ReadWorkProjection(ctx, item.ID, "implement", "worker", nil)
				if err != nil {
					errCh <- err
					return
				}
				if !proj.AuthorityAvailable || proj.TaskID != item.ID {
					errCh <- errors.New("incoherent projection read")
					return
				}
			}()
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatalf("concurrent read error: %v", err)
		}
	})
}
