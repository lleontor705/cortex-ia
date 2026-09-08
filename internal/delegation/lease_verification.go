package delegation

import (
	"context"
	"database/sql"
	"errors"
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
	err = s.db.QueryRowContext(ctx, `
		SELECT l.item_id, l.expires_at, c.owner
		FROM work_leases l
		JOIN work_claims c ON c.item_id = l.item_id
		JOIN work_items w ON w.id = l.item_id
		WHERE l.path = ? AND l.expires_at > ? AND c.expires_at > ?
		  AND w.status = 'in_progress' AND w.workspace = ? AND c.owner = ?
	`, clean, now, now, workspaceKey, "opencode-session:"+sessionID).Scan(&result.TaskID, &result.ExpiresAt, &result.Owner)
	if errors.Is(err, sql.ErrNoRows) {
		result.Reason = "no current task with a live claim and lease owned by this session in this workspace"
		return result, nil
	}
	if err != nil {
		result.Reason = "lease authority lookup failed"
		return result, err
	}
	result.Valid = true
	return result, nil
}
