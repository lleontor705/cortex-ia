package delegation

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// legacyJobSeed materializes a durable delegation_jobs row directly.
// The AGY write helpers that used to produce these rows are retired (DP-2), but
// persisted legacy rows must stay readable through the query projections.
type legacyJobSeed struct {
	id        string
	role      string
	workspace string
	status    Status
	errorCode string
	createdAt string
	updatedAt string
	startedAt string
	attempt   int
	pid       int
}

func seedLegacyJob(t *testing.T, store *Store, seed legacyJobSeed) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if seed.createdAt == "" {
		seed.createdAt = now
	}
	if seed.updatedAt == "" {
		seed.updatedAt = seed.createdAt
	}
	_, err := store.db.Exec(
		`INSERT INTO delegation_jobs(id,role,task_id,objective_digest,status,transport,workspace,pid,attempt,error_code,error_message,created_at,updated_at,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		seed.id, seed.role, "", "sha256:legacy", string(seed.status), "direct", seed.workspace, seed.pid, seed.attempt, seed.errorCode, "", seed.createdAt, seed.updatedAt, seed.startedAt, "")
	if err != nil {
		t.Fatalf("seed legacy job %s: %v", seed.id, err)
	}
}

func TestJobQueryAndCoherentView(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Query a non-terminal accepted job: no receipt must be projected.
	seedLegacyJob(t, store, legacyJobSeed{id: "job-accepted", role: "implement", workspace: tempDir, status: StatusAccepted})

	view, err := store.Query(ctx, "job-accepted")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if view.ID != "job-accepted" || view.Status != StatusAccepted {
		t.Fatalf("unexpected view status: %+v", view)
	}
	if view.ReceiptAvailable || view.Receipt != nil || view.ReceiptMissing {
		t.Fatalf("expected non-terminal job to have no receipt: %+v", view)
	}

	// 2. Query a running job carrying a pending cancellation request.
	seedLegacyJob(t, store, legacyJobSeed{id: "job-running", role: "implement", workspace: tempDir, status: StatusRunning, errorCode: "CANCEL_REQUESTED"})

	view, err = store.Query(ctx, "job-running")
	if err != nil {
		t.Fatalf("Query running job failed: %v", err)
	}
	if !view.CancellationRequested || view.Status != StatusRunning {
		t.Fatalf("expected running job with CancellationRequested=true: %+v", view)
	}

	// 3. Query a terminal job with a persisted receipt in a separate workspace.
	seedLegacyJob(t, store, legacyJobSeed{id: "job-succeeded", role: "implement", workspace: filepath.Join(tempDir, "ws2"), status: StatusSucceeded})
	_, err = store.db.Exec(
		`INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,?,?,?,?,?)`,
		"job-succeeded", string(StatusSucceeded), `{"status":"completed","files":["a.go"]}`, "sha256:receipt1", 0, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("seed receipt failed: %v", err)
	}

	view, err = store.Query(ctx, "job-succeeded")
	if err != nil {
		t.Fatalf("Query completed job failed: %v", err)
	}
	if !view.ReceiptAvailable || view.Receipt == nil || view.ReceiptMissing {
		t.Fatalf("expected terminal job to have ReceiptAvailable=true: %+v", view)
	}
	if view.Receipt.ExitCode != 0 || view.Receipt.Status != StatusSucceeded {
		t.Fatalf("unexpected receipt: %+v", view.Receipt)
	}
}

func TestJobQueryTerminalWithoutReceipt(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	seedLegacyJob(t, store, legacyJobSeed{id: "job-failed", role: "reviewer", workspace: tempDir, status: StatusFailed})

	view, err := store.Query(ctx, "job-failed")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if view.Status != StatusFailed {
		t.Fatalf("expected failed status: %+v", view)
	}
	if view.ReceiptAvailable || !view.ReceiptMissing {
		t.Fatalf("expected ReceiptAvailable=false and ReceiptMissing=true: %+v", view)
	}
}

func TestJobWait(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	seedLegacyJob(t, store, legacyJobSeed{id: "job-wait", role: "implement", workspace: tempDir, status: StatusRunning})

	// Test timeout
	timedOutView, err := store.Wait(ctx, "job-wait", 150*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if !timedOutView.WaitTimedOut {
		t.Fatalf("expected WaitTimedOut=true: %+v", timedOutView)
	}

	// Asynchronously finish the legacy job
	go func() {
		time.Sleep(50 * time.Millisecond)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, _ = store.db.Exec(`UPDATE delegation_jobs SET status=?,updated_at=?,finished_at=? WHERE id=?`, string(StatusSucceeded), now, now, "job-wait")
		_, _ = store.db.Exec(
			`INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,?,?,?,?,?)`,
			"job-wait", string(StatusSucceeded), `{}`, "sha256:done", 0, now)
	}()

	view, err := store.Wait(ctx, "job-wait", 2*time.Second)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if view.WaitTimedOut || view.Status != StatusSucceeded || !view.ReceiptAvailable {
		t.Fatalf("expected completed view: %+v", view)
	}
}
