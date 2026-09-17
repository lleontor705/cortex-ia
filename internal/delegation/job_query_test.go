package delegation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestJobQueryAndCoherentView(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Query active (accepted) job
	job, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-query-1",
		ObjectiveDigest: "sha256:digest1",
		Transport:       "direct",
		Workspace:       filepath.Join(tempDir, "ws1"),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	view, err := store.Query(ctx, job.ID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if view.ID != job.ID || view.Status != StatusAccepted {
		t.Fatalf("unexpected view status: %+v", view)
	}
	if view.ReceiptAvailable || view.Receipt != nil || view.ReceiptMissing {
		t.Fatalf("expected non-terminal job to have no receipt: %+v", view)
	}

	// 2. Mark running, request cancellation
	err = store.Claim(ctx, job.ID, "worker-1", 12345, 5*time.Minute)
	if err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	err = store.MarkRunning(ctx, job.ID)
	if err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
	err = store.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	view, err = store.Query(ctx, job.ID)
	if err != nil {
		t.Fatalf("Query after cancel request failed: %v", err)
	}
	if !view.CancellationRequested || view.Status != StatusRunning {
		t.Fatalf("expected running job with CancellationRequested=true: %+v", view)
	}

	// 3. Complete job with receipt in separate workspace
	job2, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-query-2",
		ObjectiveDigest: "sha256:digest2",
		Transport:       "direct",
		Workspace:       filepath.Join(tempDir, "ws2"),
	})
	if err != nil {
		t.Fatalf("Create job2 failed: %v", err)
	}
	if err := store.Claim(ctx, job2.ID, "worker-2", 12346, 5*time.Minute); err != nil {
		t.Fatalf("Claim job2 failed: %v", err)
	}
	if err := store.MarkRunning(ctx, job2.ID); err != nil {
		t.Fatalf("MarkRunning job2 failed: %v", err)
	}

	receipt := Receipt{
		JobID:      job2.ID,
		Status:     StatusSucceeded,
		Output:     json.RawMessage(`{"status":"completed","files":["a.go"]}`),
		OutputHash: "sha256:receipt1",
		ExitCode:   0,
	}
	err = store.Complete(ctx, job2.ID, StatusSucceeded, receipt, "", "")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	view, err = store.Query(ctx, job2.ID)
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

	job, err := store.Create(ctx, NewJob{
		Role:            "reviewer",
		TaskID:          "task-query-missing",
		ObjectiveDigest: "sha256:digest2",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Directly update status to failed without writing to delegation_receipts
	_, err = store.db.ExecContext(ctx, "UPDATE delegation_jobs SET status='failed' WHERE id=?", job.ID)
	if err != nil {
		t.Fatalf("failed update: %v", err)
	}

	view, err := store.Query(ctx, job.ID)
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

	job, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-wait",
		ObjectiveDigest: "sha256:digest3",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := store.Claim(ctx, job.ID, "worker-wait", 12347, 5*time.Minute); err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	if err := store.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}

	// Test timeout
	timedOutView, err := store.Wait(ctx, job.ID, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if !timedOutView.WaitTimedOut {
		t.Fatalf("expected WaitTimedOut=true: %+v", timedOutView)
	}

	// Asynchronously finish job
	go func() {
		time.Sleep(50 * time.Millisecond)
		receipt := Receipt{
			JobID:      job.ID,
			Status:     StatusSucceeded,
			Output:     json.RawMessage(`{}`),
			OutputHash: "sha256:done",
			ExitCode:   0,
		}
		_ = store.Complete(ctx, job.ID, StatusSucceeded, receipt, "", "")
	}()

	view, err := store.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if view.WaitTimedOut || view.Status != StatusSucceeded || !view.ReceiptAvailable {
		t.Fatalf("expected completed view: %+v", view)
	}
}
