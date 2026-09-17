package delegation

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func cancellationStore(t *testing.T) (*Store, NewJob) {
	t.Helper()
	home := t.TempDir()
	s, err := OpenStore(filepath.Join(home, "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, NewJob{Role: "implement", Transport: "direct", Workspace: home, ObjectiveDigest: "synthetic"}
}

func TestJobCancellationWaitsForSyntheticTree(t *testing.T) {
	s, input := cancellationStore(t)
	ctx := context.Background()
	job, err := s.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, job.ID, "worker", os.Getpid(), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRunning(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, os.Args[0], "-test.run=^TestProcessTreeChild$")
	cmd.Env = []string{"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "CORTEX_TEST_TREE=parent", "CORTEX_TEST_MARKER=" + filepath.Join(home, "heartbeat")}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stop, err := startProcessTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	scanner := bufio.NewScanner(pipe)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatal("synthetic child not ready")
	}
	for i := 0; i < 2; i++ {
		if err := s.Cancel(ctx, job.ID); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := s.Get(ctx, job.ID)
	if err != nil || pending.Status != StatusRunning || !pending.CancellationRequested || pending.LeaseOwner != "worker" || pending.LeaseExpiresAt == nil {
		t.Fatal("request released worker authority")
	}
	if _, err := s.Result(ctx, job.ID); err == nil {
		t.Fatal("premature terminal receipt")
	}
	if _, err := s.Create(ctx, input); err == nil {
		t.Fatal("workspace admission not fenced")
	}
	receipt := Receipt{Output: json.RawMessage(`{}`)}
	if s.CompleteWorker(ctx, job.ID, "stale", StatusSucceeded, receipt, "", "") == nil {
		t.Fatal("stale owner acknowledged")
	}
	if s.Complete(ctx, job.ID, StatusSucceeded, receipt, "", "") == nil {
		t.Fatal("ownerless cancellation acknowledgement")
	}
	done := make(chan struct{})
	go watchCancellation(runCtx, s, job.ID, cancel, done)
	_ = cmd.Wait()
	close(done)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(home, "heartbeat"))
	time.Sleep(100 * time.Millisecond)
	after, _ := os.ReadFile(filepath.Join(home, "heartbeat"))
	if string(before) != string(after) {
		t.Fatal("descendant still running")
	}
	if err := os.RemoveAll(home); err != nil {
		t.Fatal("cleanup incomplete")
	}
	if err := s.CompleteWorker(ctx, job.ID, "worker", StatusFailed, receipt, "AGY_FAILED", "cancelled context"); err != nil {
		t.Fatal(err)
	}
	confirmed, _ := s.Get(ctx, job.ID)
	if confirmed.Status != StatusCancelled || confirmed.LeaseOwner != "" || confirmed.CancellationRequested {
		t.Fatal("termination not acknowledged")
	}
	if _, err := s.Create(ctx, input); err != nil {
		t.Fatal(err)
	}
}

func TestJobCancellationAcceptedAndCompletionRaces(t *testing.T) {
	ctx := context.Background()
	s, input := cancellationStore(t)
	job, err := s.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if s.Claim(ctx, job.ID, "worker", 1, time.Minute) == nil {
		t.Fatal("claimed cancelled acceptance")
	}
	for i := 0; i < 8; i++ {
		job, err = s.Create(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Claim(ctx, job.ID, "worker", 1, time.Minute); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		errs := make(chan error, 2)
		go func() { defer wg.Done(); errs <- s.Cancel(ctx, job.ID) }()
		go func() {
			defer wg.Done()
			errs <- s.CompleteWorker(ctx, job.ID, "worker", StatusSucceeded, Receipt{Output: json.RawMessage(`{}`)}, "", "")
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		current, _ := s.Get(ctx, job.ID)
		if current.Status != StatusSucceeded && current.Status != StatusCancelled {
			t.Fatal("race did not serialize")
		}
		result, err := s.Result(ctx, job.ID)
		if err != nil || result.Status != current.Status {
			t.Fatal("receipt disagrees with state")
		}
	}
}

func TestJobCancellationExpiryRetainsFence(t *testing.T) {
	for _, readRecovery := range []bool{false, true} {
		s, input := cancellationStore(t)
		ctx := context.Background()
		now := time.Now().UTC()
		s.now = func() time.Time { return now }
		job, err := s.Create(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Claim(ctx, job.ID, "worker", 1, time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := s.Cancel(ctx, job.ID); err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Minute)
		if !readRecovery {
			if _, err := s.Recover(ctx); err != nil {
				t.Fatal(err)
			}
		}
		lost, err := s.Get(ctx, job.ID)
		if err != nil || lost.Status != StatusLost || !lost.CancellationRequested || !lost.ReconciliationRequired {
			t.Fatal("expiry erased cancellation marker")
		}
		if _, err := s.Create(ctx, input); err == nil {
			t.Fatal("lost cancellation admitted another job")
		}
		if s.CompleteWorker(ctx, job.ID, "worker", StatusCancelled, Receipt{Output: json.RawMessage(`{}`)}, "", "") == nil {
			t.Fatal("expired owner acknowledged")
		}
	}
}

func TestJobCancellationUnconfirmedCleanupStaysFenced(t *testing.T) {
	s, input := cancellationStore(t)
	ctx := context.Background()
	job, err := s.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, job.ID, "worker", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkTerminationUnconfirmed(ctx, job.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	if s.CompleteWorker(ctx, job.ID, "worker", StatusFailed, Receipt{Output: json.RawMessage(`{}`)}, "", "") == nil {
		t.Fatal("unconfirmed cleanup released fence")
	}
	if _, err := s.Create(ctx, input); err == nil {
		t.Fatal("unconfirmed cleanup admitted job")
	}
	if _, err := s.Result(ctx, job.ID); err == nil {
		t.Fatal("unconfirmed cleanup published completion")
	}
}
