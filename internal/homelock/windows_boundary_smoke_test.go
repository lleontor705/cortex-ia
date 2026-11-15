package homelock

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWindowsBoundarySmoke(t *testing.T) {
	t.Logf("OS: %s, Toolchain: %s, Architecture: %s", runtime.GOOS, runtime.Version(), runtime.GOARCH)

	tempHome := t.TempDir()
	targetPayload := []byte("boundary-smoke-payload-homelock")
	targetFile := filepath.Join(tempHome, "smoke_target.txt")

	// 1. Initial write to target
	if err := os.WriteFile(targetFile, targetPayload, 0600); err != nil {
		t.Fatalf("failed to write initial target: %v", err)
	}

	// 2. Controlled lock acquisition
	t.Log("Operation: homelock Acquire")
	lock, err := Acquire(tempHome, 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// 3. Controlled contention: second acquire attempt with short timeout
	t.Log("Operation: homelock contention check")
	_, err = Acquire(tempHome, 50*time.Millisecond)
	if !errors.Is(err, ErrHomeBusy) {
		t.Fatalf("expected ErrHomeBusy under contention, got: %v", err)
	}

	// 4. Verify target file is not corrupted during contention
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read target during contention: %v", err)
	}
	if string(content) != string(targetPayload) {
		t.Fatalf("target corrupted during contention: got %s, want %s", content, targetPayload)
	}

	// 5. Release lock
	t.Log("Operation: homelock Release")
	if err := lock.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// 6. Post-release re-acquisition and cleanup check
	lock2, err := Acquire(tempHome, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("post-release Acquire failed: %v", err)
	}
	_ = lock2.Release()

	// Final verification of target integrity
	finalContent, err := os.ReadFile(targetFile)
	if err != nil || string(finalContent) != string(targetPayload) {
		t.Fatalf("target corrupted after release: %v", err)
	}
	t.Log("Assessment: homelock handle boundaries and contention handled safely without target corruption")
}
