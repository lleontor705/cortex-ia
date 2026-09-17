package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// WorkSubmissionInput is an implementer's assertion, never a review approval.
type WorkSubmissionInput struct {
	Summary      string   `json:"summary"`
	Verdict      *string  `json:"verdict,omitempty"`
	EvidenceRefs []string `json:"evidence_refs"`
	ChangedFiles []string `json:"changed_files"`
}

type WorkSubmission struct {
	WorkSubmissionInput
	ID                  string     `json:"submission_id"`
	ItemID              string     `json:"task_id"`
	Attempt             int64      `json:"attempt"`
	ImplementationOwner string     `json:"implementation_owner"`
	TransitionRevision  int64      `json:"transition_revision"`
	From                WorkStatus `json:"from_status"`
	To                  WorkStatus `json:"to_status"`
	ReviewID            string     `json:"review_id,omitempty"`
	CreatedAt           string     `json:"created_at"`
}

func (s *Store) migrateWorkSubmissions(ctx context.Context, conn *sql.Conn) error {
	for _, statement := range []string{
		`CREATE TABLE work_submissions (
			id TEXT PRIMARY KEY CHECK(length(id)=32),
			item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE RESTRICT,
			attempt INTEGER NOT NULL CHECK(attempt>0),
			implementation_owner TEXT NOT NULL CHECK(length(CAST(implementation_owner AS BLOB)) BETWEEN 1 AND 1024),
			transition_revision INTEGER NOT NULL CHECK(transition_revision>0),
			from_status TEXT NOT NULL CHECK(from_status IN('in_progress','in_review')),
			to_status TEXT NOT NULL CHECK(to_status IN('in_progress','in_review','blocked')),
			review_id TEXT CHECK(review_id IS NULL OR length(review_id)=32),
			verification_verdict TEXT CHECK(verification_verdict IS NULL OR verification_verdict IN('PASS','FAIL','BLOCKED','INCONCLUSIVE')),
			summary TEXT NOT NULL CHECK(length(CAST(summary AS BLOB))<=8192),
			evidence_refs_json TEXT NOT NULL CHECK(length(CAST(evidence_refs_json AS BLOB))<=65536 AND json_valid(evidence_refs_json) AND json_type(evidence_refs_json)='array' AND json_array_length(evidence_refs_json)<=64),
			changed_files_json TEXT NOT NULL CHECK(length(CAST(changed_files_json AS BLOB))<=131072 AND json_valid(changed_files_json) AND json_type(changed_files_json)='array' AND json_array_length(changed_files_json)<=128),
			created_at TEXT NOT NULL,
			UNIQUE(item_id,transition_revision),
			CHECK((to_status='in_review' AND review_id IS NOT NULL) OR (to_status<>'in_review' AND review_id IS NULL))
		) STRICT`,
		`CREATE INDEX work_submissions_attempt_idx ON work_submissions(item_id,attempt,transition_revision DESC)`,
		`CREATE UNIQUE INDEX work_submissions_review_idx ON work_submissions(review_id) WHERE review_id IS NOT NULL`,
		`CREATE TRIGGER work_submissions_no_update BEFORE UPDATE ON work_submissions BEGIN SELECT RAISE(ABORT,'immutable work submission'); END`,
		`CREATE TRIGGER work_submissions_no_delete BEFORE DELETE ON work_submissions BEGIN SELECT RAISE(ABORT,'immutable work submission'); END`,
		`ALTER TABLE work_approvals ADD COLUMN submission_id TEXT REFERENCES work_submissions(id) ON DELETE RESTRICT`,
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("work submission migration: %w", err)
		}
	}
	_, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(14,?)`, s.timestamp())
	return err
}

func submissionArrays(input WorkSubmissionInput, allowed []string) (WorkSubmissionInput, string, string, error) {
	if len(input.Summary) > 8192 || strings.ContainsRune(input.Summary, 0) {
		return input, "", "", errors.New("submission summary must be at most 8192 bytes without NUL")
	}
	if input.Verdict != nil {
		verdict := strings.ToUpper(*input.Verdict)
		switch verdict {
		case "PASS", "FAIL", "BLOCKED", "INCONCLUSIVE":
			input.Verdict = &verdict
		default:
			return input, "", "", errors.New("invalid submission verification verdict")
		}
	}
	if len(input.EvidenceRefs) > 64 || len(input.ChangedFiles) > 128 {
		return input, "", "", errors.New("submission exceeds reference or changed-file count")
	}
	refs := append([]string{}, input.EvidenceRefs...)
	for _, ref := range refs {
		if len(ref) == 0 || len(ref) > 1024 || strings.ContainsRune(ref, 0) {
			return input, "", "", errors.New("submission references require 1..1024 bytes without NUL")
		}
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		canonical, err := canonicalLeasePath(name)
		if err != nil {
			return input, "", "", err
		}
		allowedSet[canonical] = true
	}
	files := make([]string, 0, len(input.ChangedFiles))
	seen := make(map[string]bool)
	for _, name := range input.ChangedFiles {
		canonical, err := canonicalLeasePath(name)
		if err != nil || !allowedSet[canonical] {
			return input, "", "", errors.New("submission changed file is invalid or outside allowed_files")
		}
		if !seen[canonical] {
			files = append(files, canonical)
			seen[canonical] = true
		}
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return input, "", "", err
	}
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return input, "", "", err
	}
	if len(refsJSON) > 65536 || len(filesJSON) > 131072 {
		return input, "", "", errors.New("submission encoded arrays exceed byte limit")
	}
	input.EvidenceRefs, input.ChangedFiles = refs, files
	return input, string(refsJSON), string(filesJSON), nil
}

func (s *Store) insertWorkSubmission(ctx context.Context, conn *sql.Conn, submission WorkSubmission) (string, error) {
	var allowedJSON string
	if err := conn.QueryRowContext(ctx, `SELECT allowed_files_json FROM work_definitions WHERE item_id=?`, submission.ItemID).Scan(&allowedJSON); err != nil {
		return "", err
	}
	var allowed []string
	if err := json.Unmarshal([]byte(allowedJSON), &allowed); err != nil {
		return "", err
	}
	input, refs, files, err := submissionArrays(submission.WorkSubmissionInput, allowed)
	if err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	var reviewID any
	if submission.ReviewID != "" {
		reviewID = submission.ReviewID
	}
	_, err = conn.ExecContext(ctx, `INSERT INTO work_submissions(id,item_id,attempt,implementation_owner,transition_revision,from_status,to_status,review_id,verification_verdict,summary,evidence_refs_json,changed_files_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, submission.ItemID, submission.Attempt, submission.ImplementationOwner, submission.TransitionRevision, submission.From, submission.To, reviewID, input.Verdict, input.Summary, refs, files, submission.CreatedAt)
	return id, err
}

func (s *Store) currentWorkSubmission(ctx context.Context, item WorkItem) (*WorkSubmission, error) {
	// A retry is a new unclaimed attempt, even before its next claimed event exists.
	if item.Status == WorkReady || item.Status == WorkBacklog {
		return nil, nil
	}
	var submission WorkSubmission
	var reviewID, verdict sql.NullString
	var refs, files string
	err := s.db.QueryRowContext(ctx, `SELECT id,item_id,attempt,implementation_owner,transition_revision,from_status,to_status,review_id,verification_verdict,summary,evidence_refs_json,changed_files_json,created_at FROM work_submissions WHERE item_id=? AND attempt=(SELECT COUNT(*) FROM work_events WHERE item_id=? AND kind='claimed') ORDER BY transition_revision DESC LIMIT 1`, item.ID, item.ID).Scan(&submission.ID, &submission.ItemID, &submission.Attempt, &submission.ImplementationOwner, &submission.TransitionRevision, &submission.From, &submission.To, &reviewID, &verdict, &submission.Summary, &refs, &files, &submission.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	submission.ReviewID = reviewID.String
	if verdict.Valid {
		submission.Verdict = &verdict.String
	}
	if err := json.Unmarshal([]byte(refs), &submission.EvidenceRefs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(files), &submission.ChangedFiles); err != nil {
		return nil, err
	}
	return &submission, nil
}
