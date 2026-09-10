package delegation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// VerifySessionWorkLease is the native mutation admission check. Diagnostic
// lease lookup without session ownership is not sufficient authority to write.
func (s *Store) VerifySessionWorkLease(ctx context.Context, filePath, workspace, sessionID string) (LeaseVerification, error) {
	clean, err := canonicalLeasePath(filePath)
	if err != nil {
		return LeaseVerification{Valid: false, Path: filePath, Reason: "invalid workspace-relative target"}, nil
	}
	result := LeaseVerification{Path: clean}
	if sessionID == "" || (ConversationOwnership{OpenCodeSessionID: sessionID, OpenCodeRootSessionID: sessionID}).Validate() != nil {
		result.Reason = "valid host session is required"
		return result, nil
	}
	if !filepath.IsAbs(workspace) {
		result.Reason = "absolute workspace is required"
		return result, nil
	}
	if info, statErr := os.Stat(workspace); statErr != nil || !info.IsDir() {
		result.Reason = "workspace must be an accessible directory"
		return result, nil
	}
	workspaceKey, err := CanonicalWorkspace(workspace)
	if err != nil || workspaceKey == "" {
		result.Reason = "workspace cannot be resolved"
		return result, err
	}
	now := s.timestamp()
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.item_id, l.expires_at, c.owner, w.workspace, l.path
		FROM work_leases l
		JOIN work_claims c ON c.item_id = l.item_id
		JOIN work_items w ON w.id = l.item_id
		WHERE l.expires_at > ? AND c.expires_at > ?
		  AND w.status = 'in_progress' AND c.owner = ?
	`, now, now, "opencode-session:"+sessionID)
	if err != nil {
		result.Reason = "lease authority lookup failed"
		return result, err
	}
	defer func() { _ = rows.Close() }()

	foundAnyForSession := false
	foundCompatibleWorkspace := false

	for rows.Next() {
		var itemID, expiresAt, owner, taskWorkspace, leasePath string
		if scanErr := rows.Scan(&itemID, &expiresAt, &owner, &taskWorkspace, &leasePath); scanErr != nil {
			result.Reason = "lease authority scan failed"
			return result, scanErr
		}
		foundAnyForSession = true

		if !WorkspacesCompatible(taskWorkspace, workspaceKey) {
			continue
		}
		foundCompatibleWorkspace = true

		// 1. Direct path match
		if leasePath == clean {
			result.TaskID = itemID
			result.ExpiresAt = expiresAt
			result.Owner = owner
			result.Valid = true
			return result, nil
		}

		// 2. taskWorkspace is a child/sub-repo of workspaceKey
		// e.g. workspaceKey = "d:/fuentes/provi", taskWorkspace = "d:/fuentes/provi/paasproviperu-backend"
		// If lease was registered relative to sub-repo (e.g. "src/test.cs")
		// and clean is relative to umbrella workspace ("paasproviperu-backend/src/test.cs")
		if subRel, ok := WorkspaceRelativePrefix(workspaceKey, taskWorkspace); ok && subRel != "" {
			if clean == subRel+"/"+leasePath {
				result.TaskID = itemID
				result.ExpiresAt = expiresAt
				result.Owner = owner
				result.Valid = true
				return result, nil
			}
		}

		// 3. workspaceKey is a child/sub-repo of taskWorkspace
		// e.g. taskWorkspace = "d:/fuentes/provi", workspaceKey = "d:/fuentes/provi/paasproviperu-backend"
		// If lease was registered relative to umbrella workspace ("paasproviperu-backend/src/test.cs")
		// and clean is relative to sub-repo ("src/test.cs")
		if subRel, ok := WorkspaceRelativePrefix(taskWorkspace, workspaceKey); ok && subRel != "" {
			if leasePath == subRel+"/"+clean {
				result.TaskID = itemID
				result.ExpiresAt = expiresAt
				result.Owner = owner
				result.Valid = true
				return result, nil
			}
		}

		// 4. Either leasePath or clean includes the sub-repository directory name
		// e.g. taskWorkspace is ".../backend", leasePath is "backend/src/handler.cs" and clean is "src/handler.cs" (or vice versa)
		baseName := filepath.Base(taskWorkspace)
		if baseName != "" && baseName != "." && baseName != "/" {
			if leasePath == baseName+"/"+clean || clean == baseName+"/"+leasePath {
				result.TaskID = itemID
				result.ExpiresAt = expiresAt
				result.Owner = owner
				result.Valid = true
				return result, nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		result.Reason = "lease iteration failed"
		return result, err
	}

	if !foundAnyForSession {
		result.Reason = "no current task with a live claim and lease owned by this session"
		return result, nil
	}
	if !foundCompatibleWorkspace {
		result.Reason = "active claim belongs to an incompatible workspace"
		return result, nil
	}
	result.Reason = fmt.Sprintf("target %q is not leased under active task in this workspace", clean)
	return result, nil
}
