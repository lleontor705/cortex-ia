package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type WorkRevisionPlan struct {
	Version           int                    `json:"version"`
	TaskID            string                 `json:"task_id"`
	BoardID           string                 `json:"board_id"`
	Project           string                 `json:"project"`
	ExpectedRevision  int64                  `json:"expected_revision"`
	ExpectedStatus    WorkStatus             `json:"expected_status"`
	ExpectedWorkflow  string                 `json:"expected_workflow,omitempty"`
	ExpectedChangeID  string                 `json:"expected_change_id,omitempty"`
	ExpectedSpecPlane string                 `json:"expected_spec_plane,omitempty"`
	Definition        WorkRevisionDefinition `json:"definition"`
}

type WorkRevisionDefinition struct {
	Title        string       `json:"title"`
	Objective    string       `json:"objective"`
	Acceptance   string       `json:"acceptance_criteria"`
	Verification string       `json:"verification"`
	AllowedFiles []string     `json:"allowed_files"`
	Contract     *SDDContract `json:"sdd_contract"`
}

func DecodeWorkRevisionPlan(r io.Reader) (WorkRevisionPlan, error) {
	var plan WorkRevisionPlan
	decoder := json.NewDecoder(io.LimitReader(r, 128*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return WorkRevisionPlan{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return WorkRevisionPlan{}, errors.New("trailing revise plan JSON")
	}
	return plan, nil
}

// ReviseWorkDefinition atomically replaces the bounded definition of an unclaimed
// ready/backlog task after read-only validation of the replacement contract pins.
func (s *Store) ReviseWorkDefinition(ctx context.Context, plan WorkRevisionPlan) (WorkItem, error) {
	normalized, allowedJSON, contractJSON, workspace, err := normalizeWorkRevisionPlan(ctx, plan)
	if err != nil {
		return WorkItem{}, err
	}
	if normalized.Definition.Contract != nil {
		if err := verifyWorkspacePins(workspace, normalized.Definition.Contract, "", ""); err != nil {
			return WorkItem{}, err
		}
		if err := verifyLocalCortexPins(ctx, normalized.Definition.Contract); err != nil {
			return WorkItem{}, err
		}
	}

	now := s.timestamp()
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var live struct {
			BoardID, Workspace, Title, Status string
			Revision                          int64
		}
		if err := conn.QueryRowContext(ctx, `SELECT board_id,workspace,title,status,revision FROM work_items WHERE id=?`, normalized.TaskID).Scan(&live.BoardID, &live.Workspace, &live.Title, &live.Status, &live.Revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		liveWorkspace, err := CanonicalWorkspace(live.Workspace)
		if err != nil {
			return err
		}
		if live.BoardID != normalized.BoardID || liveWorkspace != workspace {
			return fmt.Errorf("%w: task identity mismatch", ErrWorkConflict)
		}
		if live.Revision != normalized.ExpectedRevision {
			return fmt.Errorf("%w: task is at revision %d, not %d", ErrWorkConflict, live.Revision, normalized.ExpectedRevision)
		}
		if WorkStatus(live.Status) != normalized.ExpectedStatus || (live.Status != string(WorkReady) && live.Status != string(WorkBacklog)) {
			return fmt.Errorf("%w: task %s is %s, not revisable", ErrWorkConflict, normalized.TaskID, live.Status)
		}

		var activeClaims, activeLeases, activeReviews, replacements int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_claims WHERE item_id=? AND expires_at>?`, normalized.TaskID, now).Scan(&activeClaims); err != nil {
			return err
		}
		if activeClaims != 0 {
			return fmt.Errorf("%w: task has an active claim", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_leases WHERE item_id=? AND expires_at>?`, normalized.TaskID, now).Scan(&activeLeases); err != nil {
			return err
		}
		if activeLeases != 0 {
			return fmt.Errorf("%w: task has active file leases", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_reviews WHERE item_id=?`, normalized.TaskID).Scan(&activeReviews); err != nil {
			return err
		}
		if activeReviews != 0 {
			return fmt.Errorf("%w: task has an active review", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, normalized.TaskID).Scan(&replacements); err != nil {
			return err
		}
		if replacements != 0 {
			return fmt.Errorf("%w: task was superseded by decomposition", ErrWorkConflict)
		}

		var objective, acceptance, verification, filesJSON, oldContractJSON string
		if err := conn.QueryRowContext(ctx, `SELECT objective,acceptance_criteria,verification,allowed_files_json,contract_json FROM work_definitions WHERE item_id=?`, normalized.TaskID).Scan(&objective, &acceptance, &verification, &filesJSON, &oldContractJSON); err != nil {
			return err
		}
		oldContract, err := decodeContract(oldContractJSON)
		if err != nil {
			return err
		}
		if err := validateRevisionContractIdentity(normalized, oldContract); err != nil {
			return err
		}
		if err := requireOpenSDDChange(ctx, conn, normalized.BoardID, normalized.Definition.Contract); err != nil {
			return err
		}

		oldHash := hashJSON([]any{live.Title, objective, acceptance, verification, filesJSON, oldContractJSON})
		newHash := hashJSON([]any{normalized.Definition.Title, normalized.Definition.Objective, normalized.Definition.Acceptance, normalized.Definition.Verification, allowedJSON, contractJSON})
		if _, err := conn.ExecContext(ctx, `UPDATE work_definitions SET objective=?,acceptance_criteria=?,verification=?,allowed_files_json=?,contract_json=? WHERE item_id=?`, normalized.Definition.Objective, normalized.Definition.Acceptance, normalized.Definition.Verification, allowedJSON, contractJSON, normalized.TaskID); err != nil {
			return err
		}
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET title=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND status IN('ready','backlog')`, normalized.Definition.Title, now, normalized.TaskID, normalized.ExpectedRevision)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale revision", ErrWorkConflict)
		}
		detail, _ := json.Marshal(map[string]string{"old": oldHash, "new": newHash})
		return s.addWorkEvent(ctx, conn, normalized.TaskID, "definition_revised", live.Status, live.Status, string(detail))
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, normalized.TaskID)
}

func normalizeWorkRevisionPlan(ctx context.Context, plan WorkRevisionPlan) (WorkRevisionPlan, string, string, string, error) {
	_ = ctx
	plan.TaskID = strings.TrimSpace(plan.TaskID)
	plan.BoardID = strings.TrimSpace(plan.BoardID)
	plan.Project = strings.TrimSpace(plan.Project)
	plan.ExpectedWorkflow = strings.TrimSpace(plan.ExpectedWorkflow)
	plan.ExpectedChangeID = strings.TrimSpace(plan.ExpectedChangeID)
	plan.ExpectedSpecPlane = strings.TrimSpace(plan.ExpectedSpecPlane)
	plan.Definition.Title = strings.TrimSpace(plan.Definition.Title)
	plan.Definition.Objective = strings.TrimSpace(plan.Definition.Objective)
	plan.Definition.Acceptance = strings.TrimSpace(plan.Definition.Acceptance)
	plan.Definition.Verification = strings.TrimSpace(plan.Definition.Verification)
	if plan.Version != 1 || plan.TaskID == "" || plan.BoardID == "" || plan.Project == "" || plan.ExpectedRevision <= 0 {
		return WorkRevisionPlan{}, "", "", "", errors.New("revision plan requires version, identity, project and positive revision")
	}
	if plan.ExpectedStatus != WorkReady && plan.ExpectedStatus != WorkBacklog {
		return WorkRevisionPlan{}, "", "", "", errors.New("revision plan expected status must be ready or backlog")
	}
	if plan.Definition.Title == "" || len(plan.Definition.Title) > 512 || plan.Definition.Objective == "" || plan.Definition.Acceptance == "" || plan.Definition.Verification == "" {
		return WorkRevisionPlan{}, "", "", "", errors.New("revision definition requires bounded title, objective, acceptance and verification")
	}
	if len(plan.Definition.Objective) > 4096 || len(plan.Definition.Acceptance) > 4096 || len(plan.Definition.Verification) > 2048 {
		return WorkRevisionPlan{}, "", "", "", errors.New("revision definition exceeds size limit")
	}
	if len(plan.Definition.AllowedFiles) > 128 {
		return WorkRevisionPlan{}, "", "", "", errors.New("revision allowed file count exceeds limit")
	}
	workspace, err := ResolveProjectRoot(plan.Project)
	if err != nil {
		return WorkRevisionPlan{}, "", "", "", fmt.Errorf("resolve revision project: %w", err)
	}
	allowed := make([]string, 0, len(plan.Definition.AllowedFiles))
	seen := map[string]struct{}{}
	for _, value := range plan.Definition.AllowedFiles {
		file, err := canonicalLeasePath(value)
		if err != nil {
			return WorkRevisionPlan{}, "", "", "", fmt.Errorf("invalid allowed file %q: %w", value, err)
		}
		if _, duplicate := seen[file]; duplicate {
			continue
		}
		seen[file] = struct{}{}
		allowed = append(allowed, file)
	}
	plan.Definition.AllowedFiles = allowed
	allowedJSON, err := json.Marshal(allowed)
	if err != nil {
		return WorkRevisionPlan{}, "", "", "", err
	}
	contractJSON, err := encodeContract(plan.Definition.Contract)
	if err != nil {
		return WorkRevisionPlan{}, "", "", "", err
	}
	return plan, string(allowedJSON), contractJSON, workspace, nil
}

func validateRevisionContractIdentity(plan WorkRevisionPlan, old *SDDContract) error {
	newContract := plan.Definition.Contract
	if old != nil && newContract == nil {
		return errors.New("SDD binding cannot be removed by revision")
	}
	if old == nil {
		return nil
	}
	if plan.ExpectedWorkflow == "" || plan.ExpectedChangeID == "" || plan.ExpectedSpecPlane == "" {
		return errors.New("SDD revision requires expected workflow, change, and spec plane")
	}
	if old.Workflow != plan.ExpectedWorkflow || old.ChangeID != plan.ExpectedChangeID || old.SpecPlane != plan.ExpectedSpecPlane {
		return fmt.Errorf("%w: live SDD identity mismatch", ErrWorkConflict)
	}
	if newContract.Workflow != old.Workflow || newContract.ChangeID != old.ChangeID || newContract.SpecPlane != old.SpecPlane {
		return errors.New("SDD workflow, change, and spec plane are immutable for revision")
	}
	if !sameStringSet(old.RequirementIDs, newContract.RequirementIDs) {
		return errors.New("SDD requirement identity changed; fresh planning required")
	}
	return nil
}

func sameStringSet(a, b []string) bool {
	left, right := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
