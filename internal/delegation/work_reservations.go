package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// WorkClaimReservation returns tokens only after the entire transaction commits.
type WorkClaimReservation struct {
	WorkClaim
	ReservedFiles []WorkLease `json:"reserved_files,omitempty"`
}

func reservationPaths(paths []string) ([]string, error) {
	if len(paths) > 128 {
		return nil, errors.New("a reservation supports at most 128 paths")
	}
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, raw := range paths {
		p, err := canonicalLeasePath(raw)
		if err != nil {
			return nil, err
		}
		if seen[p] {
			return nil, fmt.Errorf("duplicate reservation path %q", p)
		}
		seen[p] = true
		result = append(result, p)
	}
	sort.Strings(result)
	return result, nil
}

// ClaimWorkWithLeases rolls back claim, attempt, events and leases together.
func (s *Store) ClaimWorkWithLeases(ctx context.Context, id, owner string, paths []string, ttl time.Duration) (WorkClaimReservation, error) {
	paths, err := reservationPaths(paths)
	if err != nil {
		return WorkClaimReservation{}, err
	}
	var result WorkClaimReservation
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		claim, err := s.claimWork(ctx, conn, id, owner, ttl)
		if err != nil {
			return err
		}
		result.WorkClaim = claim
		for _, p := range paths {
			lease, err := s.reserveWorkLease(ctx, conn, id, claim.Token, p, ttl)
			if err != nil {
				return err
			}
			result.ReservedFiles = append(result.ReservedFiles, lease)
		}
		return nil
	})
	if err != nil {
		return WorkClaimReservation{}, err
	}
	if item, getErr := s.GetWork(ctx, id); getErr == nil && item.Contract != nil && item.Contract.SpecPlane == "speckit" {
		_ = s.ProjectSpecKitState(ctx, item.Workspace, item.Contract.ChangeID, item.BoardID)
	}
	return result, nil
}

// ReserveWorkLeases acquires a deterministic batch or leaves state unchanged.
func (s *Store) ReserveWorkLeases(ctx context.Context, id, claimToken string, paths []string, ttl time.Duration) ([]WorkLease, error) {
	paths, err := reservationPaths(paths)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("at least one reservation path is required")
	}
	leases := make([]WorkLease, 0, len(paths))
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		for _, p := range paths {
			lease, err := s.reserveWorkLease(ctx, conn, id, claimToken, p, ttl)
			if err != nil {
				return err
			}
			leases = append(leases, lease)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return leases, nil
}
