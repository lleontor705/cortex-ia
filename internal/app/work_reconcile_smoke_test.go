package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

// TestWorkReconcileIncidentSmoke replays the obs-176/obs-184 incident through the
// real CLI dispatcher against a CORTEX_IA_HOME temporary state root: a synthetic
// owner claims the task with file leases, dies without renewing, the orchestrator
// proves the fail-closed refusals, force-releases the aged orphan, retries, and a
// fresh owner claims the task while the dead owner's token stays dead.
const (
	reconcileIncidentOwner  = "opencode-session:dead-owner"
	reconcileIncidentHost   = "orch-sess"
	reconcileIncidentReason = "orphaned token: controller died post-quoting failure"
	reconcileStaleProbe     = 16 * time.Minute
)

type reconcileLedger struct {
	Status         string
	Revision       int64
	Claims         int
	Leases         int
	Events         int
	ReconciledRows int
}

func TestWorkReconcileIncidentSmoke(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	db := openReconcileLedger(t, home)
	taskID := "recon-incident"
	seedReconcileIncidentTask(t, home, taskID)

	claim := workJSON[delegation.WorkClaimReservation](t, []string{"claim", taskID, "--owner", reconcileIncidentOwner, "--path", "src/a.go", "--path", "src/b.go", "--ttl", "1h"})
	if claim.Token == "" || len(claim.ReservedFiles) != 2 {
		t.Fatalf("incident claim did not retain a live token with two leases: %+v", claim)
	}

	t.Run("a_fresh_foreign_claim_resists_reconcile", func(t *testing.T) {
		before := readReconcileLedger(t, db, taskID)
		err := workError([]string{"reconcile", taskID, "--reason", reconcileIncidentReason, "--session", reconcileIncidentHost, "--revision", strconv.FormatInt(claim.Revision, 10)})
		assertRefusalCode(t, err, "claim_fresh_no_inactivity_evidence")
		assertLedgerUnchanged(t, before, readReconcileLedger(t, db, taskID))
	})

	t.Run("an_aged_orphan_is_force_released_with_audit", func(t *testing.T) {
		ageReconcileClaim(t, db, taskID)
		out, err := captureStdout(func() error {
			return runWork([]string{"reconcile", taskID, "--reason", reconcileIncidentReason, "--session", reconcileIncidentHost, "--revision", strconv.FormatInt(claim.Revision, 10)})
		})
		if err != nil {
			t.Fatalf("reconcile release failed: %v", err)
		}
		receipt := firstJSON[delegation.ReconcileResult](t, out)
		if receipt.Decision != "released" || receipt.From != delegation.WorkInProgress || receipt.To != delegation.WorkBlocked || receipt.RevisionAfter != claim.Revision+1 {
			t.Fatalf("unexpected decision: %+v", receipt)
		}
		if strings.Join(receipt.ReleasedLeases, ",") != "src/a.go,src/b.go" {
			t.Fatalf("released leases = %v", receipt.ReleasedLeases)
		}
		if receipt.Inputs.ActorSession != reconcileIncidentHost || receipt.Inputs.ClaimOwner != reconcileIncidentOwner || receipt.Inputs.Reason != reconcileIncidentReason || receipt.Inputs.OwnerSessionInactive {
			t.Fatalf("decision inputs incomplete: %+v", receipt.Inputs)
		}
		if !strings.Contains(out, "reconciled "+taskID+":") {
			t.Fatalf("missing human summary line:\n%s", out)
		}
		if ledger := readReconcileLedger(t, db, taskID); ledger.Status != string(delegation.WorkBlocked) || ledger.Revision != claim.Revision+1 || ledger.Claims != 0 || ledger.Leases != 0 || ledger.ReconciledRows != 1 {
			t.Fatalf("unexpected post-release ledger: %+v", ledger)
		}
		detail := readReconcileDetail(t, db, taskID)
		var snapshot delegation.ReconcileDecisionSnapshot
		if err := json.Unmarshal([]byte(detail), &snapshot); err != nil {
			t.Fatalf("decode audit snapshot: %v", err)
		}
		if snapshot.ActorSession != reconcileIncidentHost || snapshot.ClaimOwner != reconcileIncidentOwner || snapshot.RevisionBefore != claim.Revision || snapshot.RevisionAfter != claim.Revision+1 {
			t.Fatalf("audit snapshot mismatch: %+v", snapshot)
		}
		assertNoReconcileTokenMaterial(t, claim, out, detail)
	})

	t.Run("retry_and_fresh_claim_succeed_while_old_token_stays_dead", func(t *testing.T) {
		item := workJSON[delegation.WorkItem](t, []string{"retry", taskID, "--revision", strconv.FormatInt(claim.Revision+1, 10)})
		if item.Status != delegation.WorkReady || item.Revision != claim.Revision+2 {
			t.Fatalf("unexpected retry item: %+v", item)
		}
		fresh := workJSON[delegation.WorkClaimReservation](t, []string{"claim", taskID, "--owner", "opencode-session:new-owner", "--path", "src/a.go", "--ttl", "1h"})
		if fresh.Token == "" || fresh.Revision <= item.Revision {
			t.Fatalf("fresh claim did not acquire the retried task: %+v", fresh)
		}
		if err := workError([]string{"renew", taskID, "--claim-token", claim.Token}); err == nil {
			t.Fatal("dead owner token renewed the fresh claim")
		}
		if err := workError([]string{"transition", taskID, "--claim-token", claim.Token, "--revision", strconv.FormatInt(fresh.Revision, 10), "--to", "in_review"}); err == nil {
			t.Fatal("dead owner token transitioned the fresh claim")
		}
	})

	t.Run("expired_claim_routes_to_recover", func(t *testing.T) {
		expiredID := "recon-expired"
		seedReconcileIncidentTask(t, home, expiredID)
		expired := workJSON[delegation.WorkClaimReservation](t, []string{"claim", expiredID, "--owner", reconcileIncidentOwner, "--ttl", "1s"})
		time.Sleep(1100 * time.Millisecond)
		before := readReconcileLedger(t, db, expiredID)
		err := workError([]string{"reconcile", expiredID, "--reason", reconcileIncidentReason, "--session", reconcileIncidentHost, "--revision", strconv.FormatInt(expired.Revision, 10)})
		assertRefusalCode(t, err, "claim_not_live")
		assertLedgerUnchanged(t, before, readReconcileLedger(t, db, expiredID))
	})

	t.Run("the_owners_own_session_is_protected", func(t *testing.T) {
		selfID := "recon-self"
		seedReconcileIncidentTask(t, home, selfID)
		self := workJSON[delegation.WorkClaimReservation](t, []string{"claim", selfID, "--owner", "opencode-session:" + reconcileIncidentHost, "--ttl", "1h"})
		before := readReconcileLedger(t, db, selfID)
		err := workError([]string{"reconcile", selfID, "--reason", reconcileIncidentReason, "--session", reconcileIncidentHost, "--revision", strconv.FormatInt(self.Revision, 10), "--owner-session-inactive", "true"})
		assertRefusalCode(t, err, "current_session_owner")
		assertLedgerUnchanged(t, before, readReconcileLedger(t, db, selfID))
	})
}

func seedReconcileIncidentTask(t *testing.T, home, id string) {
	t.Helper()
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if _, err := store.CreateWorkInBoardWithDefinition(context.Background(), delegation.DefaultBoardID, id, id, nil, delegation.WorkDefinition{
		Project: t.TempDir(), Objective: "replay the reconcile incident", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/a.go", "src/b.go"},
	}); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func workJSON[T any](t *testing.T, args []string) T {
	t.Helper()
	out, err := captureStdout(func() error { return runWork(args) })
	if err != nil {
		t.Fatalf("work %v failed: %v", args, err)
	}
	return firstJSON[T](t, out)
}

func firstJSON[T any](t *testing.T, out string) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&value); err != nil {
		t.Fatalf("decode work output %q: %v", out, err)
	}
	return value
}

func workError(args []string) error {
	_, err := captureStdout(func() error { return runWork(args) })
	return err
}

func assertRefusalCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), code) {
		t.Fatalf("expected refusal %q, got %v", code, err)
	}
}

func openReconcileLedger(t *testing.T, home string) *sql.DB {
	t.Helper()
	dbPath := delegation.DefaultDBPath(home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func ageReconcileClaim(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	stamp := time.Now().UTC().Add(-reconcileStaleProbe).Format(time.RFC3339Nano)
	if _, err := db.Exec(`UPDATE work_claims SET updated_at=? WHERE item_id=?`, stamp, id); err != nil {
		t.Fatalf("age claim %s: %v", id, err)
	}
}

func readReconcileLedger(t *testing.T, db *sql.DB, id string) reconcileLedger {
	t.Helper()
	var ledger reconcileLedger
	if err := db.QueryRow(`SELECT status,revision,(SELECT COUNT(*) FROM work_claims WHERE item_id=?), (SELECT COUNT(*) FROM work_leases WHERE item_id=?), (SELECT COUNT(*) FROM work_events WHERE item_id=?), (SELECT COUNT(*) FROM work_events WHERE item_id=? AND kind='reconciled') FROM work_items WHERE id=?`, id, id, id, id, id).Scan(&ledger.Status, &ledger.Revision, &ledger.Claims, &ledger.Leases, &ledger.Events, &ledger.ReconciledRows); err != nil {
		t.Fatalf("read ledger %s: %v", id, err)
	}
	return ledger
}

func readReconcileDetail(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var detail string
	if err := db.QueryRow(`SELECT detail FROM work_events WHERE item_id=? AND kind='reconciled'`, id).Scan(&detail); err != nil {
		t.Fatalf("read audit detail %s: %v", id, err)
	}
	return detail
}

func assertLedgerUnchanged(t *testing.T, want, got reconcileLedger) {
	t.Helper()
	if want != got {
		t.Fatalf("refusal mutated durable state\nwant: %+v\n got: %+v", want, got)
	}
}

func assertNoReconcileTokenMaterial(t *testing.T, claim delegation.WorkClaimReservation, outputs ...string) {
	t.Helper()
	secrets := []string{claim.Token}
	for _, lease := range claim.ReservedFiles {
		secrets = append(secrets, lease.Token)
	}
	blob := strings.Join(outputs, "\n")
	for _, secret := range secrets {
		if secret != "" && strings.Contains(blob, secret) {
			t.Fatal("token material leaked into the reconcile receipt or audit snapshot")
		}
	}
}
