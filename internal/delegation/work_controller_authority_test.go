package delegation

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func controllerFixture(t *testing.T) (*Store, WorkClaimReservation, ControllerAuthority) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.CreateWorkInBoardWithDefinition(context.Background(), DefaultBoardID, "controller", "controller", nil, WorkDefinition{Project: t.TempDir(), AllowedFiles: []string{"a.go", "b.go"}})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWorkWithLeases(context.Background(), "controller", "opencode-session:controller", []string{"a.go", "b.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	auth := ControllerAuthority{ClaimToken: claim.Token, Leases: map[string]string{}}
	for _, lease := range claim.ReservedFiles {
		auth.Leases[lease.Path] = lease.Token
	}
	return s, claim, auth
}

func TestControllerRenewalAtomicLiveAuthority(t *testing.T) {
	s, claim, auth := controllerFixture(t)
	ctx := context.Background()
	receipt, err := s.RenewControllerAuthority(ctx, "controller", claim.Owner, auth, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.GetWork(ctx, "controller")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TaskID != item.ID || receipt.Owner != claim.Owner || receipt.LeaseCount != 2 || receipt.ExpiresAt != item.Claim.ExpiresAt {
		t.Fatalf("bad receipt: %+v", receipt)
	}
	for _, lease := range item.Leases {
		if lease.ExpiresAt != receipt.ExpiresAt {
			t.Fatal("non-atomic deadlines")
		}
	}
	if len(auth.Leases) != 2 || auth.ClaimToken != claim.Token {
		t.Fatal("caller authority mutated")
	}
}

func TestControllerRenewalRejectsWithoutPartialWrites(t *testing.T) {
	for _, scenario := range []string{"claim-token", "lease-token", "missing", "extra", "owner", "claim-expired", "lease-expired", "state", "sql-failure"} {
		t.Run(scenario, func(t *testing.T) {
			s, claim, auth := controllerFixture(t)
			ctx := context.Background()
			owner := claim.Owner
			switch scenario {
			case "claim-token":
				auth.ClaimToken = "wrong"
			case "lease-token":
				auth.Leases["b.go"] = "wrong"
			case "missing":
				delete(auth.Leases, "b.go")
			case "extra":
				auth.Leases["c.go"] = "wrong"
			case "owner":
				owner = "other"
			case "claim-expired":
				if _, err := s.db.Exec(`UPDATE work_claims SET expires_at='2000-01-01T00:00:00Z'`); err != nil {
					t.Fatal(err)
				}
			case "lease-expired":
				if _, err := s.db.Exec(`UPDATE work_leases SET expires_at='2000-01-01T00:00:00Z' WHERE path='b.go'`); err != nil {
					t.Fatal(err)
				}
			case "state":
				if _, err := s.TransitionWork(ctx, "controller", claim.Token, claim.Revision, WorkInReview); err != nil {
					t.Fatal(err)
				}
			case "sql-failure":
				if _, err := s.db.Exec(`CREATE TRIGGER reject_controller_lease BEFORE UPDATE ON work_leases BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			before, err := s.GetWork(ctx, "controller")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.RenewControllerAuthority(ctx, "controller", owner, auth, 5*time.Minute); err == nil {
				t.Fatal("invalid authority accepted")
			}
			after, err := s.GetWork(ctx, "controller")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.Claim, after.Claim) || !reflect.DeepEqual(before.Leases, after.Leases) {
				t.Fatal("partial renewal escaped rollback")
			}
		})
	}
}
