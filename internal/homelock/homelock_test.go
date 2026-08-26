package homelock

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestHomelockLifecycle(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Acquire lock
	lock, err := Acquire(tempHome, 2*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if lock.Path() != filepath.Join(tempHome, MetadataDirName, LockFileName) {
		t.Errorf("unexpected lock path: %s", lock.Path())
	}

	// 2. Contention: Attempt second acquire with short timeout -> ErrHomeBusy
	_, err = Acquire(tempHome, 50*time.Millisecond)
	if !errors.Is(err, ErrHomeBusy) {
		t.Fatalf("expected ErrHomeBusy on contention, got: %v", err)
	}

	// 3. Release lock
	if err := lock.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// 4. Re-acquire after release succeeds
	lock2, err := Acquire(tempHome, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Re-acquire after release failed: %v", err)
	}
	_ = lock2.Release()
}

func TestHomelockErrors(t *testing.T) {
	// Empty home
	_, err := Acquire("", time.Second)
	if !errors.Is(err, ErrInvalidHome) {
		t.Errorf("expected ErrInvalidHome for empty string, got: %v", err)
	}
}
