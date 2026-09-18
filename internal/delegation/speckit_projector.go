package delegation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HighRiskKeywords are paths or keywords that trigger mandatory security reviews.
var HighRiskKeywords = []string{
	"auth", "oauth", "jwt", "session", "password", "credential",
	"crypto", "cipher", "tls", "ssl", "cert",
	"sql", "migration", "schema", "secrets", "vault",
}

// DetectHighRiskPaths evaluates whether any assigned file paths touch security-sensitive surfaces.
func DetectHighRiskPaths(paths []string) bool {
	for _, p := range paths {
		lower := strings.ToLower(filepath.ToSlash(p))
		for _, kw := range HighRiskKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

// ProjectSpecKitState writes the projected SpecKit state to .specify/ in the given workspace.
func (s *Store) ProjectSpecKitState(ctx context.Context, workspace string, changeID string, boardID string) error {
	if workspace == "" || changeID == "" || boardID == "" {
		return nil
	}

	canonical, err := CanonicalWorkspace(workspace)
	if err != nil {
		return err
	}

	items, err := s.listWork(ctx, boardID, false)
	if err != nil {
		return err
	}

	// Filter items belonging to this changeID (if contract is set)
	var changeItems []WorkItem
	for _, item := range items {
		if item.Contract != nil && item.Contract.ChangeID == changeID {
			changeItems = append(changeItems, item)
		} else if item.Contract == nil {
			changeItems = append(changeItems, item)
		}
	}

	if len(changeItems) == 0 {
		return nil
	}

	specDir := filepath.Join(canonical, ".specify", "specs", changeID)
	runtimeDir := filepath.Join(canonical, ".specify", "runtime")

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return fmt.Errorf("create speckit spec dir: %w", err)
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return fmt.Errorf("create speckit runtime dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	total := len(changeItems)
	doneCount := 0
	inProgressCount := 0
	blockedCount := 0
	var activeItem *WorkItem

	for i := range changeItems {
		item := &changeItems[i]
		switch item.Status {
		case WorkDone:
			doneCount++
		case WorkInProgress:
			inProgressCount++
			if activeItem == nil {
				activeItem = item
			}
		case WorkBlocked:
			blockedCount++
		}
	}

	phase := "TASKS"
	status := "ACTIVE"
	if doneCount == total && total > 0 {
		phase = "DONE"
		status = "DONE"
	} else if inProgressCount > 0 {
		phase = "IMPLEMENT"
	}

	// 1. Render state.yaml
	var sbState strings.Builder
	fmt.Fprintf(&sbState, "feature_id: %q\n", changeID)
	fmt.Fprintf(&sbState, "title: %q\n", changeID)
	fmt.Fprintf(&sbState, "phase: %q\n", phase)
	fmt.Fprintf(&sbState, "status: %q\n\n", status)
	sbState.WriteString("gates:\n")
	sbState.WriteString("  spec:\n    status: \"APPROVED\"\n")
	sbState.WriteString("  plan:\n    status: \"APPROVED\"\n")
	if status == "DONE" {
		sbState.WriteString("  final:\n    status: \"APPROVED\"\n\n")
	} else {
		sbState.WriteString("  final:\n    status: \"PENDING\"\n\n")
	}

	if activeItem != nil {
		fmt.Fprintf(&sbState, "active_task: %q\n\n", activeItem.ID)
	} else {
		sbState.WriteString("active_task: \"\"\n\n")
	}

	sbState.WriteString("counters:\n")
	fmt.Fprintf(&sbState, "  tasks_total: %d\n", total)
	fmt.Fprintf(&sbState, "  tasks_done: %d\n", doneCount)
	fmt.Fprintf(&sbState, "  tasks_in_progress: %d\n", inProgressCount)
	fmt.Fprintf(&sbState, "  tasks_blocked: %d\n\n", blockedCount)

	sbState.WriteString("artifacts:\n")
	sbState.WriteString("  spec: \"spec.md\"\n")
	sbState.WriteString("  acceptance: \"acceptance.md\"\n")
	sbState.WriteString("  plan: \"plan.md\"\n")
	sbState.WriteString("  tasks: \"tasks.md\"\n")
	sbState.WriteString("  evidence: \"evidence.md\"\n\n")
	fmt.Fprintf(&sbState, "updated_at: %q\n", now)

	if err := os.WriteFile(filepath.Join(specDir, "state.yaml"), []byte(sbState.String()), 0o644); err != nil {
		return fmt.Errorf("write state.yaml: %w", err)
	}

	// 2. Render tasks.md
	var sbTasks strings.Builder
	sbTasks.WriteString("# Tasks\n\n")
	sbTasks.WriteString("| ID | Estado | Rol | Descripción | Dependencias | Criterios |\n")
	sbTasks.WriteString("|---|---|---|---|---|---|\n")
	for _, it := range changeItems {
		desc := strings.ReplaceAll(it.Title, "|", "\\|")
		deps := "—"
		if len(it.Dependencies) > 0 {
			deps = strings.Join(it.Dependencies, ", ")
		}
		criteria := "—"
		if it.Acceptance != "" {
			criteria = strings.ReplaceAll(it.Acceptance, "|", "\\|")
		}
		fmt.Fprintf(&sbTasks, "| %s | %s | %s | %s | %s | %s |\n",
			it.ID, strings.ToUpper(string(it.Status)), "implementer", desc, deps, criteria)
	}
	sbTasks.WriteString("\n")

	for _, it := range changeItems {
		fmt.Fprintf(&sbTasks, "## %s\n\n", it.ID)
		sbTasks.WriteString("### Objetivo\n\n")
		if it.Objective != "" {
			sbTasks.WriteString(it.Objective + "\n\n")
		} else {
			sbTasks.WriteString(it.Title + "\n\n")
		}
		sbTasks.WriteString("### Rutas permitidas\n\n")
		if len(it.AllowedFiles) > 0 {
			for _, f := range it.AllowedFiles {
				fmt.Fprintf(&sbTasks, "- `%s`\n", f)
			}
		} else {
			sbTasks.WriteString("- *(ninguna asignada)*\n")
		}
		sbTasks.WriteString("\n### Validación\n\n")
		if it.Verification != "" {
			fmt.Fprintf(&sbTasks, "- `%s`\n\n", it.Verification)
		} else {
			sbTasks.WriteString("- *(sin comando de validación)*\n\n")
		}
	}

	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(sbTasks.String()), 0o644); err != nil {
		return fmt.Errorf("write tasks.md: %w", err)
	}

	// 3. Render evidence.md
	var sbEvidence strings.Builder
	sbEvidence.WriteString("# Evidence\n\n")
	for _, it := range changeItems {
		fmt.Fprintf(&sbEvidence, "## %s\n\n", it.ID)
		fmt.Fprintf(&sbEvidence, "- **Estado:** %s\n", it.Status)
		if it.Verification != "" {
			fmt.Fprintf(&sbEvidence, "- **Comando:** `%s`\n", it.Verification)
		}
		if it.LatestApproval != nil {
			fmt.Fprintf(&sbEvidence, "- **Aprobador:** %s\n", it.LatestApproval.Reviewer)
			fmt.Fprintf(&sbEvidence, "- **Veredicto:** %s\n", it.LatestApproval.Verdict)
			if it.LatestApproval.Evidence != "" {
				fmt.Fprintf(&sbEvidence, "- **Evidencia:** %s\n", it.LatestApproval.Evidence)
			}
		}
		sbEvidence.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(specDir, "evidence.md"), []byte(sbEvidence.String()), 0o644); err != nil {
		return fmt.Errorf("write evidence.md: %w", err)
	}

	// 4. Render active-task.json
	activeTaskPath := filepath.Join(runtimeDir, "active-task.json")
	if activeItem != nil {
		allowed := activeItem.AllowedFiles
		if allowed == nil {
			allowed = []string{}
		}
		expires := ""
		if activeItem.Claim != nil {
			expires = activeItem.Claim.ExpiresAt
		}
		payload := map[string]any{
			"featureId":      changeID,
			"taskId":         activeItem.ID,
			"agent":          "sdd-implementer",
			"allowedRoots":   allowed,
			"forbiddenRoots": []string{".specify", ".opencode", "infra", "deploy"},
			"expiresAt":      expires,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			_ = os.WriteFile(activeTaskPath, data, 0o644)
		}
	} else {
		payload := map[string]any{
			"active": false,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		_ = os.WriteFile(activeTaskPath, data, 0o644)
	}

	return nil
}
