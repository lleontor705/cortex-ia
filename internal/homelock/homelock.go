// Package homelock provides a cross-process, per-home exclusive lock used to
// serialize mutating cortex-ia operations that target the same user home.
//
// The lock is keyed by the canonical home path and materialized as a single
// lock file at <home>/.cortex-ia/operation.lock. Acquisition relies on native
// operating-system locks — LockFileEx on Windows and flock(2) on Unix — so the
// lock is released automatically by the OS when the owning process exits or
// crashes. No lock state is persisted beyond the empty lock file, lock
// authority is process-local only, and no symlinked path component is
// followed: a symlinked home or metadata directory fails closed with
// ErrUnsafeHome.
package homelock

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// MetadataDirName is the per-home metadata directory that owns the lock file.
const MetadataDirName = ".cortex-ia"

// LockFileName is the lock file name inside the metadata directory.
const LockFileName = "operation.lock"

// ErrHomeBusy is returned when another process already holds the home lock
// and the requested acquisition timeout elapsed. It never implies mutation.
var ErrHomeBusy = errors.New("homelock: home locked by another process")

// ErrUnsafeHome is returned when the home, the metadata directory, or the
// lock file itself resolves to a symlink or otherwise irregular node; the
// lock never follows such links.
var ErrUnsafeHome = errors.New("homelock: unsafe lock path (symlink or irregular node)")

// ErrInvalidHome is returned for empty, missing, or non-directory homes.
var ErrInvalidHome = errors.New("homelock: invalid home directory")

// Bounds for the bounded nonblocking retry loop in Acquire.
const (
	minRetryInterval = 2 * time.Millisecond
	maxRetryInterval = 50 * time.Millisecond
)

// Lock is an acquired exclusive cross-process home lock. Lock authority is
// process-local: it is never persisted and never rendered into receipts.
type Lock struct {
	path    string
	release func() error
}

// Path returns the canonical lock file path backing the lock.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Release releases the lock and closes the underlying OS handle. It is
// idempotent: releasing an already released (or nil) Lock is a no-op that
// returns nil.
func (l *Lock) Release() error {
	if l == nil || l.release == nil {
		return nil
	}
	release := l.release
	l.release = nil
	return release()
}

// Acquire acquires the exclusive cross-process lock for home using bounded
// nonblocking retries until the timeout elapses. A zero or negative timeout
// performs exactly one attempt. On contention past the deadline it returns an
// error wrapping ErrHomeBusy; no install state is touched. The home and the
// <home>/.cortex-ia metadata directory must exist as real directories or be
// creatable; symlinked or irregular components fail closed with ErrUnsafeHome
// before any lock file is opened.
func Acquire(home string, timeout time.Duration) (*Lock, error) {
	path, err := LockPath(home)
	if err != nil {
		return nil, err
	}
	if err := rejectLink(path); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	interval := minRetryInterval
	for {
		lk, err := tryLock(path)
		if err == nil {
			return lk, nil
		}
		if !errors.Is(err, errLocked) {
			return nil, fmt.Errorf("homelock: acquire %s: %w", path, err)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("%w: %s", ErrHomeBusy, path)
		}
		sleep := interval
		if sleep > remaining {
			sleep = remaining
		}
		time.Sleep(sleep)
		interval *= 2
		if interval > maxRetryInterval {
			interval = maxRetryInterval
		}
	}
}

// LockPath returns the canonical lock file path for home, creating the
// <home>/.cortex-ia metadata directory (mode 0o700) when absent. It validates
// via Lstat that neither home nor the metadata directory nor the lock file is
// a symlink or irregular node, so the lock never follows links.
func LockPath(home string) (string, error) {
	if home == "" {
		return "", ErrInvalidHome
	}
	canonical, err := filepath.Abs(filepath.Clean(home))
	if err != nil {
		return "", fmt.Errorf("homelock: canonicalize home: %w", err)
	}
	if err := validateRealDir(canonical); err != nil {
		return "", err
	}
	meta := filepath.Join(canonical, MetadataDirName)
	if err := ensureMetaDir(meta); err != nil {
		return "", err
	}
	return filepath.Join(meta, LockFileName), nil
}

// validateRealDir reports via Lstat that path is a real directory, not a
// symlink, reparse-like link, or other irregular node.
func validateRealDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidHome, path, err)
	}
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("%w: %s", ErrUnsafeHome, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrInvalidHome, path)
	}
	return nil
}

// ensureMetaDir creates the metadata directory when absent and revalidates it
// with Lstat so a symlink swapped in around creation is rejected.
func ensureMetaDir(meta string) error {
	err := os.Mkdir(meta, 0o700)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("homelock: create metadata directory: %w", err)
	}
	if err := validateRealDir(meta); err != nil {
		return err
	}
	return nil
}

// rejectLink fails closed when the lock file already exists as a symlink or
// irregular node; a missing file is fine because the first holder creates it.
func rejectLink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("homelock: inspect lock file: %w", err)
	}
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("%w: %s", ErrUnsafeHome, path)
	}
	return nil
}
