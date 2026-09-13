package delegation

import (
	"strings"
	"time"
)

const (
	ActionDisclaimer = "Action requires live SQLite authority revalidation and explicit claims/leases before execution."

	BlockerUnsatisfiedDeps = "UNSATISFIED_DEPENDENCIES"
	BlockerClaimExpired    = "CLAIM_EXPIRED"
	BlockerClaimHeldOther  = "CLAIM_HELD_BY_ANOTHER"
	BlockerSelfReview      = "SELF_REVIEW_PROHIBITED"
	BlockerTaskBlocked     = "TASK_BLOCKED"
	BlockerTaskSuperseded  = "TASK_SUPERSEDED"
)

type WorkProjectionNote struct {
	Source  string `json:"source"`
	Content string `json:"content"`
	Stale   bool   `json:"stale,omitempty"`
}

type WorkProjectionBlocker struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type WorkCandidateAction struct {
	Action     string   `json:"action"`
	Role       string   `json:"role"`
	Conditions []string `json:"conditions,omitempty"`
	Disclaimer string   `json:"disclaimer"`
}

type WorkProjection struct {
	TaskID              string                  `json:"task_id"`
	Revision            int64                   `json:"revision"`
	AsOf                string                  `json:"as_of"`
	Status              WorkStatus              `json:"status"`
	PhaseStatus         string                  `json:"phase_status,omitempty"`
	VerificationVerdict string                  `json:"verification_verdict,omitempty"`
	AuthorityAvailable  bool                    `json:"authority_available"`
	Notes               []WorkProjectionNote    `json:"notes,omitempty"`
	Blockers            []WorkProjectionBlocker `json:"blockers,omitempty"`
	CandidateActions    []WorkCandidateAction   `json:"candidate_actions,omitempty"`
}

func (p WorkProjection) Clone() WorkProjection {
	cp := p
	if p.Notes != nil {
		cp.Notes = make([]WorkProjectionNote, len(p.Notes))
		copy(cp.Notes, p.Notes)
	}
	if p.Blockers != nil {
		cp.Blockers = make([]WorkProjectionBlocker, len(p.Blockers))
		copy(cp.Blockers, p.Blockers)
	}
	if p.CandidateActions != nil {
		cp.CandidateActions = make([]WorkCandidateAction, len(p.CandidateActions))
		for i, a := range p.CandidateActions {
			actionCopy := a
			if a.Conditions != nil {
				actionCopy.Conditions = make([]string, len(a.Conditions))
				copy(actionCopy.Conditions, a.Conditions)
			}
			cp.CandidateActions[i] = actionCopy
		}
	}
	return cp
}

type WorkProjectionInput struct {
	TaskID              string
	Revision            int64
	AsOf                string
	Status              WorkStatus
	PhaseStatus         string
	VerificationVerdict string
	Dependencies        []string
	UnsatisfiedDeps     []string
	ClaimOwner          string
	ClaimExpiresAt      string
	ClaimExpired        bool
	ReviewID            string
	ImplementationOwner string
	ReviewRevision      int64
	HasActiveReview     bool
	AuthorityAvailable  bool
	PresentationRole    string
	Actor               string
	Notes               []WorkProjectionNote
	Now                 time.Time
}

func isRecognizedRole(role string) bool {
	switch role {
	case "orchestrator", "implement", "reviewer", "planner", "investigate", "discovery":
		return true
	default:
		return false
	}
}

func ProjectWork(input WorkProjectionInput) WorkProjection {
	proj := WorkProjection{
		TaskID:              input.TaskID,
		Revision:            input.Revision,
		AsOf:                input.AsOf,
		Status:              input.Status,
		PhaseStatus:         input.PhaseStatus,
		VerificationVerdict: input.VerificationVerdict,
		AuthorityAvailable:  input.AuthorityAvailable,
	}

	if len(input.Notes) > 0 {
		proj.Notes = make([]WorkProjectionNote, len(input.Notes))
		copy(proj.Notes, input.Notes)
	}

	if len(input.UnsatisfiedDeps) > 0 {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerUnsatisfiedDeps,
			Description: "Unsatisfied dependencies: " + strings.Join(input.UnsatisfiedDeps, ", "),
		})
	}
	if input.Status == WorkInProgress && input.ClaimExpired {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerClaimExpired,
			Description: "Claim expired at " + input.ClaimExpiresAt,
		})
	}
	if input.Status == WorkInProgress && !input.ClaimExpired && input.Actor != "" && input.ClaimOwner != "" && input.Actor != input.ClaimOwner {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerClaimHeldOther,
			Description: "Claim is held by " + input.ClaimOwner,
		})
	}
	isSelfReview := input.Status == WorkInReview && input.Actor != "" && input.ImplementationOwner != "" && strings.EqualFold(input.Actor, input.ImplementationOwner)
	if isSelfReview {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerSelfReview,
			Description: "Reviewer matches implementation owner (" + input.ImplementationOwner + "); self-review prohibited",
		})
	}
	if input.Status == WorkBlocked {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerTaskBlocked,
			Description: "Task is marked blocked",
		})
	}
	if input.Status == WorkSuperseded {
		proj.Blockers = append(proj.Blockers, WorkProjectionBlocker{
			Code:        BlockerTaskSuperseded,
			Description: "Task is superseded by decomposed subtasks",
		})
	}

	normRole := strings.ToLower(strings.TrimSpace(input.PresentationRole))
	if !isRecognizedRole(normRole) || !input.AuthorityAvailable {
		return proj.Clone()
	}

	switch normRole {
	case "implement":
		if input.Status == WorkReady && len(input.UnsatisfiedDeps) == 0 {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "request_claim",
				Role:       "implement",
				Conditions: []string{"dependencies_satisfied", "no_active_claim"},
				Disclaimer: ActionDisclaimer,
			})
		} else if input.Status == WorkInProgress && !input.ClaimExpired {
			if input.Actor == "" || input.ClaimOwner == "" || input.Actor == input.ClaimOwner {
				proj.CandidateActions = append(proj.CandidateActions,
					WorkCandidateAction{
						Action:     "transition_in_review",
						Role:       "implement",
						Conditions: []string{"active_claim_held", "verification_passed"},
						Disclaimer: ActionDisclaimer,
					},
					WorkCandidateAction{
						Action:     "renew_claim",
						Role:       "implement",
						Conditions: []string{"active_claim_held"},
						Disclaimer: ActionDisclaimer,
					},
				)
			}
		}
	case "reviewer":
		if input.Status == WorkInReview && !isSelfReview {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "independent_review",
				Role:       "reviewer",
				Conditions: []string{"independent_reviewer", "review_binding_valid"},
				Disclaimer: ActionDisclaimer,
			})
		}
	case "orchestrator":
		if input.Status == WorkReady && len(input.UnsatisfiedDeps) == 0 {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "dispatch_implement",
				Role:       "orchestrator",
				Conditions: []string{"task_ready"},
				Disclaimer: ActionDisclaimer,
			})
		} else if input.Status == WorkInProgress && input.ClaimExpired {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "reconcile_expired_claim",
				Role:       "orchestrator",
				Conditions: []string{"claim_expired"},
				Disclaimer: ActionDisclaimer,
			})
		} else if input.Status == WorkInReview {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "dispatch_reviewer",
				Role:       "orchestrator",
				Conditions: []string{"in_review_ready"},
				Disclaimer: ActionDisclaimer,
			})
		} else if input.Status == WorkBlocked {
			proj.CandidateActions = append(proj.CandidateActions, WorkCandidateAction{
				Action:     "reconcile_blocked_task",
				Role:       "orchestrator",
				Conditions: []string{"task_blocked"},
				Disclaimer: ActionDisclaimer,
			})
		}
	}

	return proj.Clone()
}
