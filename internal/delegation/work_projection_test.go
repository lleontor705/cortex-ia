package delegation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorkProjection(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	t.Run("missing_and_unknown_role", func(t *testing.T) {
		input := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkReady,
			AuthorityAvailable: true,
			Now:                now,
		}
		for _, role := range []string{"", "unknown", "guest", "model"} {
			input.PresentationRole = role
			proj := ProjectWork(input)
			if len(proj.CandidateActions) != 0 {
				t.Fatalf("expected 0 candidate actions for role %q, got %d", role, len(proj.CandidateActions))
			}
		}
	})

	t.Run("dependencies_ready_and_blocked", func(t *testing.T) {
		inputBlocked := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkReady,
			UnsatisfiedDeps:    []string{"dep-1"},
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Now:                now,
		}
		projBlocked := ProjectWork(inputBlocked)
		if len(projBlocked.Blockers) == 0 || projBlocked.Blockers[0].Code != BlockerUnsatisfiedDeps {
			t.Fatalf("expected BlockerUnsatisfiedDeps, got %+v", projBlocked.Blockers)
		}
		if len(projBlocked.CandidateActions) != 0 {
			t.Fatalf("expected 0 candidate actions when deps unsatisfied, got %d", len(projBlocked.CandidateActions))
		}

		inputReady := inputBlocked
		inputReady.UnsatisfiedDeps = nil
		projReady := ProjectWork(inputReady)
		if len(projReady.Blockers) != 0 {
			t.Fatalf("expected 0 blockers, got %+v", projReady.Blockers)
		}
		if len(projReady.CandidateActions) != 1 || projReady.CandidateActions[0].Action != "request_claim" {
			t.Fatalf("expected request_claim action, got %+v", projReady.CandidateActions)
		}

		inputReady.PresentationRole = "orchestrator"
		projOrch := ProjectWork(inputReady)
		if len(projOrch.CandidateActions) != 1 || projOrch.CandidateActions[0].Action != "dispatch_implement" {
			t.Fatalf("expected dispatch_implement, got %+v", projOrch.CandidateActions)
		}
	})

	t.Run("owner_and_expiry", func(t *testing.T) {
		expired := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkInProgress,
			ClaimOwner:         "agent-1",
			ClaimExpiresAt:     now.Add(-time.Hour).Format(time.RFC3339),
			ClaimExpired:       true,
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Actor:              "agent-1",
			Now:                now,
		}
		projExpired := ProjectWork(expired)
		if len(projExpired.Blockers) == 0 || projExpired.Blockers[0].Code != BlockerClaimExpired {
			t.Fatalf("expected BlockerClaimExpired, got %+v", projExpired.Blockers)
		}
		if len(projExpired.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for expired implementer, got %d", len(projExpired.CandidateActions))
		}

		expired.PresentationRole = "orchestrator"
		projReconcile := ProjectWork(expired)
		if len(projReconcile.CandidateActions) != 1 || projReconcile.CandidateActions[0].Action != "reconcile_expired_claim" {
			t.Fatalf("expected reconcile_expired_claim, got %+v", projReconcile.CandidateActions)
		}

		active := expired
		active.ClaimExpiresAt = now.Add(time.Hour).Format(time.RFC3339)
		active.ClaimExpired = false
		active.PresentationRole = "implement"
		active.Actor = "agent-2"
		projHeldOther := ProjectWork(active)
		if len(projHeldOther.Blockers) == 0 || projHeldOther.Blockers[0].Code != BlockerClaimHeldOther {
			t.Fatalf("expected BlockerClaimHeldOther, got %+v", projHeldOther.Blockers)
		}
		if len(projHeldOther.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for different actor, got %d", len(projHeldOther.CandidateActions))
		}

		active.Actor = "agent-1"
		projOwner := ProjectWork(active)
		if len(projOwner.CandidateActions) != 2 {
			t.Fatalf("expected transition_in_review and renew_claim, got %+v", projOwner.CandidateActions)
		}
	})

	t.Run("independent_review", func(t *testing.T) {
		input := WorkProjectionInput{
			TaskID:              "task-1",
			Status:              WorkInReview,
			ImplementationOwner: "dev-alice",
			AuthorityAvailable:  true,
			PresentationRole:    "reviewer",
			Actor:               "dev-alice",
			Now:                 now,
		}
		projSelf := ProjectWork(input)
		if len(projSelf.Blockers) == 0 || projSelf.Blockers[0].Code != BlockerSelfReview {
			t.Fatalf("expected BlockerSelfReview, got %+v", projSelf.Blockers)
		}
		if len(projSelf.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for self-approval attempt, got %d", len(projSelf.CandidateActions))
		}

		input.Actor = "dev-bob"
		projIndep := ProjectWork(input)
		if len(projIndep.CandidateActions) != 1 || projIndep.CandidateActions[0].Action != "independent_review" {
			t.Fatalf("expected independent_review, got %+v", projIndep.CandidateActions)
		}
	})

	t.Run("blocked_done_superseded", func(t *testing.T) {
		inputBlocked := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkBlocked,
			AuthorityAvailable: true,
			PresentationRole:   "orchestrator",
			Now:                now,
		}
		projBlocked := ProjectWork(inputBlocked)
		if len(projBlocked.CandidateActions) != 1 || projBlocked.CandidateActions[0].Action != "reconcile_blocked_task" {
			t.Fatalf("expected reconcile_blocked_task, got %+v", projBlocked.CandidateActions)
		}

		inputDone := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkDone,
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Now:                now,
		}
		projDone := ProjectWork(inputDone)
		if len(projDone.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for done task, got %+v", projDone.CandidateActions)
		}

		inputSuperseded := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkSuperseded,
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Now:                now,
		}
		projSuperseded := ProjectWork(inputSuperseded)
		if len(projSuperseded.Blockers) == 0 || projSuperseded.Blockers[0].Code != BlockerTaskSuperseded {
			t.Fatalf("expected BlockerTaskSuperseded, got %+v", projSuperseded.Blockers)
		}
	})

	t.Run("stale_notes_and_disclaimer", func(t *testing.T) {
		notes := []WorkProjectionNote{
			{Source: "memory", Content: "previous attempt failed", Stale: true},
		}
		input := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkReady,
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Notes:              notes,
			Now:                now,
		}
		proj := ProjectWork(input)
		if len(proj.Notes) != 1 || !proj.Notes[0].Stale {
			t.Fatalf("expected stale note preserved, got %+v", proj.Notes)
		}
		if len(proj.Blockers) != 0 {
			t.Fatalf("stale notes must not create blockers, got %+v", proj.Blockers)
		}
		for _, a := range proj.CandidateActions {
			if a.Disclaimer != ActionDisclaimer {
				t.Fatalf("expected ActionDisclaimer in %+v", a)
			}
		}
	})

	t.Run("nested_output_mutation_and_no_tokens", func(t *testing.T) {
		input := WorkProjectionInput{
			TaskID:             "task-1",
			Status:             WorkReady,
			AuthorityAvailable: true,
			PresentationRole:   "implement",
			Notes:              []WorkProjectionNote{{Source: "test", Content: "orig"}},
			Now:                now,
		}
		proj := ProjectWork(input)
		proj.Notes[0].Content = "mutated"
		proj.CandidateActions[0].Conditions[0] = "mutated_condition"

		proj2 := ProjectWork(input)
		if proj2.Notes[0].Content != "orig" {
			t.Fatalf("input was mutated: got %q", proj2.Notes[0].Content)
		}
		if proj2.CandidateActions[0].Conditions[0] != "dependencies_satisfied" {
			t.Fatalf("subsequent output was mutated: got %q", proj2.CandidateActions[0].Conditions[0])
		}

		raw, err := json.Marshal(proj2)
		if err != nil {
			t.Fatalf("json marshal failed: %v", err)
		}
		if strings.Contains(string(raw), "token") {
			t.Fatalf("token field escaped in json: %s", string(raw))
		}
	})
}
