package delegation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type activeSessionLease struct {
	itemID        string
	expiresAt     string
	owner         string
	taskWorkspace string
	leasePath     string
}

func matchLease(filePath, workspaceKey string, leases []activeSessionLease, foundAnyForSession, foundCompatibleWorkspace bool) LeaseVerification {
	clean, err := canonicalLeasePath(filePath)
	if err != nil {
		return LeaseVerification{Valid: false, Path: filePath, Reason: "invalid workspace-relative target"}
	}
	result := LeaseVerification{Path: clean}
	for _, l := range leases {
		if !WorkspacesCompatible(l.taskWorkspace, workspaceKey) {
			continue
		}

		// 1. Direct path match
		if l.leasePath == clean {
			result.TaskID = l.itemID
			result.ExpiresAt = l.expiresAt
			result.Owner = l.owner
			result.Valid = true
			return result
		}

		// 2. taskWorkspace is a child/sub-repo of workspaceKey
		if subRel, ok := WorkspaceRelativePrefix(workspaceKey, l.taskWorkspace); ok && subRel != "" {
			if clean == subRel+"/"+l.leasePath {
				result.TaskID = l.itemID
				result.ExpiresAt = l.expiresAt
				result.Owner = l.owner
				result.Valid = true
				return result
			}
		}

		// 3. workspaceKey is a child/sub-repo of taskWorkspace
		if subRel, ok := WorkspaceRelativePrefix(l.taskWorkspace, workspaceKey); ok && subRel != "" {
			if l.leasePath == subRel+"/"+clean {
				result.TaskID = l.itemID
				result.ExpiresAt = l.expiresAt
				result.Owner = l.owner
				result.Valid = true
				return result
			}
		}

		// 4. Either leasePath or clean includes the sub-repository directory name
		baseName := filepath.Base(l.taskWorkspace)
		if baseName != "" && baseName != "." && baseName != "/" {
			if l.leasePath == baseName+"/"+clean || clean == baseName+"/"+l.leasePath {
				result.TaskID = l.itemID
				result.ExpiresAt = l.expiresAt
				result.Owner = l.owner
				result.Valid = true
				return result
			}
		}
	}

	if !foundAnyForSession {
		result.Reason = "no current task with a live claim and lease owned by this session"
		return result
	}
	if !foundCompatibleWorkspace {
		result.Reason = "active claim belongs to an incompatible workspace"
		return result
	}
	result.Reason = fmt.Sprintf("target %q is not leased under active task in this workspace", clean)
	return result
}

// VerifySessionWorkLeases validates multiple paths atomically against active session leases.
func (s *Store) VerifySessionWorkLeases(ctx context.Context, filePaths []string, workspace, sessionID string) ([]LeaseVerification, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}
	if sessionID == "" || (ConversationOwnership{OpenCodeSessionID: sessionID, OpenCodeRootSessionID: sessionID}).Validate() != nil {
		results := make([]LeaseVerification, len(filePaths))
		for i, fp := range filePaths {
			results[i] = LeaseVerification{Path: fp, Reason: "valid host session is required"}
		}
		return results, nil
	}
	if !filepath.IsAbs(workspace) {
		results := make([]LeaseVerification, len(filePaths))
		for i, fp := range filePaths {
			results[i] = LeaseVerification{Path: fp, Reason: "absolute workspace is required"}
		}
		return results, nil
	}
	if info, statErr := os.Stat(workspace); statErr != nil || !info.IsDir() {
		results := make([]LeaseVerification, len(filePaths))
		for i, fp := range filePaths {
			results[i] = LeaseVerification{Path: fp, Reason: "workspace must be an accessible directory"}
		}
		return results, nil
	}
	workspaceKey, err := CanonicalWorkspace(workspace)
	if err != nil || workspaceKey == "" {
		results := make([]LeaseVerification, len(filePaths))
		for i, fp := range filePaths {
			results[i] = LeaseVerification{Path: fp, Reason: "workspace cannot be resolved"}
		}
		return results, err
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
		results := make([]LeaseVerification, len(filePaths))
		for i, fp := range filePaths {
			results[i] = LeaseVerification{Path: fp, Reason: "lease authority lookup failed"}
		}
		return results, err
	}
	defer func() { _ = rows.Close() }()

	var leases []activeSessionLease
	foundAnyForSession := false
	foundCompatibleWorkspace := false

	for rows.Next() {
		var l activeSessionLease
		if scanErr := rows.Scan(&l.itemID, &l.expiresAt, &l.owner, &l.taskWorkspace, &l.leasePath); scanErr != nil {
			return nil, scanErr
		}
		foundAnyForSession = true
		if WorkspacesCompatible(l.taskWorkspace, workspaceKey) {
			foundCompatibleWorkspace = true
		}
		leases = append(leases, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]LeaseVerification, len(filePaths))
	for i, fp := range filePaths {
		results[i] = matchLease(fp, workspaceKey, leases, foundAnyForSession, foundCompatibleWorkspace)
	}
	return results, nil
}

// VerifySessionWorkLease is the native mutation admission check. Diagnostic
// lease lookup without session ownership is not sufficient authority to write.
func (s *Store) VerifySessionWorkLease(ctx context.Context, filePath, workspace, sessionID string) (LeaseVerification, error) {
	res, err := s.VerifySessionWorkLeases(ctx, []string{filePath}, workspace, sessionID)
	if err != nil {
		return LeaseVerification{Path: filePath, Reason: "lease authority lookup failed"}, err
	}
	if len(res) == 0 {
		return LeaseVerification{Path: filePath, Reason: "no verification returned"}, nil
	}
	return res[0], nil
}
