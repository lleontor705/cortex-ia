package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ControllerAuthority is live native controller authority, never worker authority.
// Tokens are accepted transiently and are not included in the acknowledgement.
type ControllerAuthority struct {
	ClaimToken string            `json:"claim_token"`
	Leases     map[string]string `json:"leases"`
}

type ControllerRenewal struct {
	TaskID     string `json:"task_id"`
	Owner      string `json:"owner"`
	ExpiresAt  string `json:"expires_at"`
	LeaseCount int    `json:"lease_count"`
}

// RenewControllerAuthority renews exactly the complete set of owned live leases
// and its live claim in one transaction. It cannot revive expired authority.
func (s *Store) RenewControllerAuthority(ctx context.Context, id, owner string, auth ControllerAuthority, ttl time.Duration) (ControllerRenewal, error) {
	result := ControllerRenewal{}
	if id == "" || owner == "" || len(owner) > 1024 || auth.ClaimToken == "" || len(auth.ClaimToken) > 128 || len(auth.Leases) > 1024 || ttl < time.Second || ttl > 15*time.Minute {
		return result, errors.New("controller renewal requires bounded live authority and ttl between 1s and 15m")
	}
	leases := make(map[string]string, len(auth.Leases))
	for path, token := range auth.Leases {
		canonical, err := canonicalLeasePath(path)
		if err != nil || token == "" || len(token) > 128 || leases[canonical] != "" {
			return result, errors.New("controller renewal contains invalid or duplicate lease authority")
		}
		leases[canonical] = token
	}
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		now := s.now().UTC()
		var status, storedOwner, hash, expiry string
		if err := conn.QueryRowContext(ctx, `SELECT w.status,c.owner,c.token_hash,c.expires_at FROM work_items w JOIN work_claims c ON c.item_id=w.id WHERE w.id=?`, id).Scan(&status, &storedOwner, &hash, &expiry); err != nil {
			return fmt.Errorf("%w: controller claim unavailable", ErrWorkConflict)
		}
		expires, err := time.Parse(time.RFC3339Nano, expiry)
		if err != nil || !expires.After(now) || status != string(WorkInProgress) || storedOwner != owner || hash != tokenHash(auth.ClaimToken) {
			return fmt.Errorf("%w: controller claim is not live or owned", ErrWorkConflict)
		}
		rows, err := conn.QueryContext(ctx, `SELECT path,token_hash,expires_at FROM work_leases WHERE item_id=?`, id)
		if err != nil {
			return err
		}
		count := 0
		valid := true
		for rows.Next() {
			var path, leaseHash, leaseExpiry string
			if err := rows.Scan(&path, &leaseHash, &leaseExpiry); err != nil {
				_ = rows.Close()
				return err
			}
			deadline, parseErr := time.Parse(time.RFC3339Nano, leaseExpiry)
			if parseErr != nil || !deadline.After(now) || leases[path] == "" || leaseHash != tokenHash(leases[path]) {
				valid = false
			}
			count++
		}
		rowErr := rows.Err()
		_ = rows.Close()
		if rowErr != nil {
			return rowErr
		}
		if !valid || count != len(leases) {
			return fmt.Errorf("%w: controller lease set is incomplete or expired", ErrWorkConflict)
		}
		until := now.Add(ttl).Format(time.RFC3339Nano)
		stamp := now.Format(time.RFC3339Nano)
		if _, err := conn.ExecContext(ctx, `UPDATE work_claims SET expires_at=?,updated_at=? WHERE item_id=?`, until, stamp, id); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `UPDATE work_leases SET expires_at=?,updated_at=? WHERE item_id=?`, until, stamp, id); err != nil {
			return err
		}
		result = ControllerRenewal{TaskID: id, Owner: owner, ExpiresAt: until, LeaseCount: count}
		return s.addWorkEvent(ctx, conn, id, "controller_renewed", "", "", owner)
	})
	if err != nil {
		return ControllerRenewal{}, err
	}
	return result, nil
}
