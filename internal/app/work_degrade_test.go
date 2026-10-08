package app

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

// degradeStaleAge exceeds the 168h default TTL so aged fixtures are unambiguously
// eligible for degradation regardless of wall-clock jitter.
const degradeStaleAge = 200 * time.Hour

// TestWorkDegrade covers the CLI wiring for REQ-W3-004 (work degrade) and the
// --blocked-reason plumbing for REQ-W3-003 on work transition, against a
// temporary CORTEX_IA_HOME.
func TestWorkDegrade(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	db := openReconcileLedger(t, home)

	t.Run("REQ_CLI_001_degrade_receipt_degrades_stale_fixture", func(t *testing.T) {
		id := "degrade-stale"
		seedDegradeTask(t, id)
		ageDegradeTask(t, db, id)

		out, err := captureStdout(func() error {
			return runWork([]string{"degrade", "--board", delegation.DefaultBoardID})
		})
		if err != nil {
			t.Fatalf("work degrade failed: %v", err)
		}
		result := firstJSON[delegation.DegradeStaleWorkResult](t, out)
		if result.Degraded != 1 || result.DryRun {
			t.Fatalf("unexpected degrade receipt: %+v", result)
		}
		if got := result.ByReason["ready"]; len(got) != 1 || got[0] != id {
			t.Fatalf("by_reason[ready] = %v, want [%s]", got, id)
		}
		status, revision := readDegradeState(t, db, id)
		if status != string(delegation.WorkBacklog) || revision != 2 {
			t.Fatalf("post-degrade row = (%s, rev %d), want (backlog, rev 2)", status, revision)
		}
	})

	t.Run("REQ_CLI_002_dry_run_mutates_nothing", func(t *testing.T) {
		id := "degrade-dry"
		seedDegradeTask(t, id)
		ageDegradeTask(t, db, id)

		out, err := captureStdout(func() error {
			return runWork([]string{"degrade", "--board", delegation.DefaultBoardID, "--dry-run"})
		})
		if err != nil {
			t.Fatalf("work degrade --dry-run failed: %v", err)
		}
		result := firstJSON[delegation.DegradeStaleWorkResult](t, out)
		if !result.DryRun || result.Degraded != 1 {
			t.Fatalf("unexpected dry-run receipt: %+v", result)
		}
		status, revision := readDegradeState(t, db, id)
		if status != string(delegation.WorkReady) || revision != 1 {
			t.Fatalf("dry-run mutated state to (%s, rev %d)", status, revision)
		}
	})

	t.Run("REQ_CLI_003_blocked_reason_forwarded_and_stored", func(t *testing.T) {
		id := "blocked-reason-valid"
		seedDegradeTask(t, id)
		claim := workJSON[delegation.WorkClaimReservation](t, []string{"claim", id, "--owner", "opencode-session:degrade", "--ttl", "1h"})

		out, err := captureStdout(func() error {
			return runWork([]string{
				"transition", id,
				"--claim-token", claim.Token,
				"--revision", strconv.FormatInt(claim.Revision, 10),
				"--to", "blocked",
				"--blocked-reason", delegation.WorkBlockedReasonUpstream,
			})
		})
		if err != nil {
			t.Fatalf("blocked transition failed: %v", err)
		}
		item := firstJSON[delegation.WorkItem](t, out)
		if item.Status != delegation.WorkBlocked || item.BlockedReason != delegation.WorkBlockedReasonUpstream {
			t.Fatalf("transitioned item = (%s, %q), want (blocked, upstream)", item.Status, item.BlockedReason)
		}
	})

	t.Run("REQ_CLI_004_unknown_taxonomy_is_usage_error", func(t *testing.T) {
		id := "blocked-reason-bogus"
		seedDegradeTask(t, id)
		claim := workJSON[delegation.WorkClaimReservation](t, []string{"claim", id, "--owner", "opencode-session:degrade", "--ttl", "1h"})

		err := workError([]string{
			"transition", id,
			"--claim-token", claim.Token,
			"--revision", strconv.FormatInt(claim.Revision, 10),
			"--to", "blocked",
			"--blocked-reason", "not_a_class",
		})
		if err == nil || !strings.Contains(err.Error(), "blocked-reason") {
			t.Fatalf("unknown taxonomy error = %v, want usage error naming blocked-reason", err)
		}
		status, revision := readDegradeState(t, db, id)
		if status != string(delegation.WorkInProgress) || revision != claim.Revision {
			t.Fatalf("rejected transition mutated state to (%s, rev %d)", status, revision)
		}
	})

	t.Run("REQ_CLI_005_default_ttl_is_168h", func(t *testing.T) {
		id := "degrade-default-ttl"
		seedDegradeTask(t, id)
		ageDegradeTask(t, db, id)

		out, err := captureStdout(func() error {
			return runWork([]string{"degrade", "--board", delegation.DefaultBoardID})
		})
		if err != nil {
			t.Fatalf("work degrade failed: %v", err)
		}
		result := firstJSON[delegation.DegradeStaleWorkResult](t, out)
		if result.TTL != delegation.DefaultDegradeStaleWorkTTL.String() {
			t.Fatalf("default ttl = %q, want %q", result.TTL, delegation.DefaultDegradeStaleWorkTTL)
		}
	})
}

func seedDegradeTask(t *testing.T, id string) {
	t.Helper()
	workJSON[delegation.WorkItem](t, []string{"create", id, id, "--board", delegation.DefaultBoardID})
}

func ageDegradeTask(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	stamp := time.Now().UTC().Add(-degradeStaleAge).Format(time.RFC3339Nano)
	if _, err := db.Exec(`UPDATE work_items SET updated_at=? WHERE id=?`, stamp, id); err != nil {
		t.Fatalf("age task %s: %v", id, err)
	}
}

func readDegradeState(t *testing.T, db *sql.DB, id string) (string, int64) {
	t.Helper()
	var status string
	var revision int64
	if err := db.QueryRow(`SELECT status, revision FROM work_items WHERE id=?`, id).Scan(&status, &revision); err != nil {
		t.Fatalf("read task %s: %v", id, err)
	}
	return status, revision
}
