package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWorkBranchAndPruneCli(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", tempHome)

	// 1. Test board current
	stdout, err := captureOutput(func() error {
		return runBoard([]string{"current"})
	})
	if err != nil {
		t.Fatalf("runBoard current failed: %v", err)
	}
	if !strings.Contains(stdout, "branch-feat-dashboard-improvements") && !strings.Contains(stdout, "default") {
		t.Errorf("expected branch board or default in output, got: %s", stdout)
	}

	// 2. Test work create without --board (auto-detect branch)
	stdout, err = captureOutput(func() error {
		return runWork([]string{"create", "cli-test-task", "CLI Test Task"})
	})
	if err != nil {
		t.Fatalf("runWork create failed: %v", err)
	}
	if !strings.Contains(stdout, "cli-test-task") {
		t.Errorf("expected cli-test-task in output, got: %s", stdout)
	}

	// 3. Test work list --current
	stdout, err = captureOutput(func() error {
		return runWork([]string{"list", "--current"})
	})
	if err != nil {
		t.Fatalf("runWork list --current failed: %v", err)
	}
	if !strings.Contains(stdout, "cli-test-task") {
		t.Errorf("expected cli-test-task in list --current output, got: %s", stdout)
	}

	// 4. Test work prune --dry-run
	stdout, err = captureOutput(func() error {
		return runWork([]string{"prune", "--dry-run", "--older-than", "0s"})
	})
	if err != nil {
		t.Fatalf("runWork prune dry-run failed: %v", err)
	}
	if !strings.Contains(stdout, `"dry_run": true`) && !strings.Contains(stdout, `"dry_run":true`) {
		t.Errorf("expected dry_run in prune output, got: %s", stdout)
	}
}

func captureOutput(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	runErr := f()
	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	_ = r.Close()

	return out, runErr
}
