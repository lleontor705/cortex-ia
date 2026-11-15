package delegation

import "context"

// HasAnyActiveAuthority reports whether the store holds live work authority anywhere: an unexpired
// claim, an unexpired file lease, or a task still in_progress. It is global on purpose — a binary
// replacement is unsafe while any session owns work, and the workspace-scoped
// HasActiveWorkspaceWork cannot see authority whose workspace does not match the caller.
//
// It is read-only and never claims, leases, or renews anything, so it is safe to call from a
// read-only store handle. A nil store means "no database, no authority".
func HasAnyActiveAuthority(ctx context.Context, s *Store) (bool, error) {
	if s == nil {
		return false, nil
	}
	now := s.timestamp()
	var active int
	err := s.db.QueryRowContext(ctx, `
		SELECT
			EXISTS(SELECT 1 FROM work_claims WHERE expires_at > ?)
			OR EXISTS(SELECT 1 FROM work_leases WHERE expires_at > ?)
			OR EXISTS(SELECT 1 FROM work_items WHERE status = 'in_progress')`,
		now, now).Scan(&active)
	if err != nil {
		return false, err
	}
	return active != 0, nil
}
