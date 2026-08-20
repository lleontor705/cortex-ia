//go:build !windows

package homelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errLocked signals plain contention to the bounded retry loop in Acquire.
var errLocked = errors.New("homelock: lock held")

// tryLock opens the lock file and takes an exclusive nonblocking flock(2).
// Closing the descriptor — normally or on process death — releases the lock
// to the OS; the lock file itself is intentionally left in place.
func tryLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("homelock: open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) ||
			errors.Is(err, syscall.EAGAIN) ||
			errors.Is(err, syscall.EINTR) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("homelock: flock: %w", err)
	}
	return &Lock{
		path: path,
		release: func() error {
			defer f.Close()
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
				return fmt.Errorf("homelock: flock unlock: %w", err)
			}
			return nil
		},
	}, nil
}
