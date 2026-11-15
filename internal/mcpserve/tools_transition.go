package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

func workTransition(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	target := optString(args, "to")
	if target == "" {
		return "", errors.New("'to' is required: one of in_review, in_progress, blocked")
	}
	revision, err := reqRevision(args)
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, false)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "transition", task, "--claim-token", "@stdin", "--to", target, "--revision", fmt.Sprint(revision)}
	if summary, ok := presentString(args, "summary"); ok {
		argv = append(argv, "--summary", summary)
	}
	if verdict, ok := presentString(args, "verdict"); ok {
		argv = append(argv, "--verdict", verdict)
	}
	if reason, ok := presentString(args, "blocked_reason"); ok {
		argv = append(argv, "--blocked-reason", reason)
	}
	refs, err := optStringList(args, "evidence_refs")
	if err != nil {
		return "", err
	}
	if target == "in_review" {
		if err := s.assertGotchaEvidence(ctx, task, refs); err != nil {
			return "", err
		}
	}
	for _, ref := range refs {
		argv = append(argv, "--evidence-ref", ref)
	}
	changed, err := optStringList(args, "changed_files")
	if err != nil {
		return "", err
	}
	for _, file := range changed {
		argv = append(argv, "--changed-file", file)
	}
	stdout, err := s.runner.Run(ctx, argv, a.claimToken)
	if err != nil {
		return "", err
	}
	if err := confirmTransition(stdout, task, target); err != nil {
		return "", err
	}
	if target != "in_progress" {
		s.vault.stop(task, "delivery")
	}
	switch target {
	case "in_review":
		s.vault.clearLeases(a)
	case "blocked":
		s.vault.clearLeases(a)
		s.vault.remove(task)
	}
	return cleanOutput(stdout), nil
}

func confirmTransition(stdout, task, target string) error {
	const unconfirmed = "WORK_TRANSITION_UNCONFIRMED: durable transition acknowledgement is missing or inconsistent; query task status before retrying"
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	delivered, ok := decoded.(map[string]any)
	if err != nil || !ok || delivered["task_id"] != task || delivered["status"] != target {
		return errors.New(unconfirmed)
	}
	leases, present := delivered["leases"]
	if present && leases != nil {
		list, isList := leases.([]any)
		if !isList || (target != "in_progress" && len(list) > 0) {
			return errors.New(unconfirmed)
		}
	}
	return nil
}

// assertGotchaEvidence requires gotchas/<task> evidence when the approval
// history already holds a FAIL verdict. An unreadable history fails closed.
func (s *session) assertGotchaEvidence(ctx context.Context, task string, refs []string) error {
	required := gotchaEvidencePrefix + task
	stdout, err := s.runner.Run(ctx, []string{"work", "approvals", task}, "")
	if err != nil {
		return gotchaFailure("GOTCHA_EVIDENCE_UNVERIFIED", task, "approval history could not be read: "+err.Error())
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	if err != nil {
		return gotchaFailure("GOTCHA_EVIDENCE_UNVERIFIED", task, "approval history output was not valid JSON")
	}
	approvals, ok := decoded.([]any)
	if !ok {
		return nil
	}
	hasFail := false
	for _, raw := range approvals {
		if entry, ok := raw.(map[string]any); ok && strings.EqualFold(fmt.Sprint(entry["verdict"]), "FAIL") {
			hasFail = true
		}
	}
	if !hasFail || contains(refs, required) {
		return nil
	}
	return gotchaFailure("GOTCHA_EVIDENCE_REQUIRED", task, fmt.Sprintf("a FAIL verdict in the approval history requires evidence_refs to include %q", required))
}

func gotchaFailure(code, task, detail string) error {
	body, _ := json.Marshal(map[string]any{
		"code": code, "task_id": task, "detail": detail,
		"required_evidence_ref": gotchaEvidencePrefix + task,
		"action":                "SAVE_GOTCHA_EVIDENCE_THEN_RETRANSITION",
	})
	return errors.New(string(body))
}

func workApprove(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	if a := s.vault.get(task); a != nil && a.sessionKey == key {
		return "", errors.New("implementation session cannot approve its own task")
	}
	verdict := optString(args, "verdict")
	if verdict == "" {
		return "", errors.New("verdict is required")
	}
	verdict = strings.ToUpper(verdict)
	if !contains([]string{"PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"}, verdict) {
		return "", errors.New("verdict must be one of PASS, FAIL, BLOCKED, INCONCLUSIVE")
	}
	evidence := optString(args, "evidence")
	if verdict == "PASS" && evidence == "" {
		return "", errors.New("EVIDENCE_REQUIRED: a PASS approval needs `evidence` (a pointer to test output, an observation id, or a commit). Pass it as the `evidence` argument; `summary` and `findings` are optional and are appended to it")
	}
	findings, err := optStringList(args, "findings")
	if err != nil {
		return "", err
	}
	var notes []string
	if summary := optString(args, "summary"); summary != "" {
		notes = append(notes, summary)
	}
	if len(findings) > 0 {
		notes = append(notes, "Findings: "+strings.Join(findings, "; "))
	}
	if note := strings.Join(notes, " "); note != "" {
		if evidence != "" {
			evidence += " | " + note
		} else {
			evidence = note
		}
	}
	revision, err := reqRevision(args)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "approve", task, "--reviewer", hostbridge.Owner(key), "--verdict", verdict}
	if evidence != "" {
		argv = append(argv, "--evidence", evidence)
	}
	argv = append(argv, "--revision", fmt.Sprint(revision))
	stdout, err := s.runner.Run(ctx, argv, "")
	if err != nil {
		return "", err
	}
	s.vault.remove(task)
	return cleanOutput(stdout), nil
}

func recoverOwn(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a := s.vault.get(task)
	if a == nil || a.sessionKey != key {
		return "", errors.New("scoped recover denied: session does not own the in-memory authority for this task")
	}
	stdout, err := s.runner.Run(ctx, []string{"work", "recover", "--task", task, "--owner", hostbridge.Owner(key)}, "")
	if err != nil {
		return "", err
	}
	s.vault.remove(task)
	return cleanOutput(stdout), nil
}
