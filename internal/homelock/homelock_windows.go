//go:build windows

package homelock

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// The locked byte range [0, lockRange) covers the whole lock file so every
// writer in this package contends on the identical range.
const (
	lockRangeLow  = 0xFFFFFFFF
	lockRangeHigh = 0x7FFFFFFF
)

// errLocked signals plain contention to the bounded retry loop in Acquire.
var errLocked = errors.New("homelock: lock held")

// tryLock opens the lock file and takes an exclusive LockFileEx byte-range
// lock. The handle is opened with read sharing only: no
// FILE_FLAG_DELETE_ON_CLOSE and no FILE_SHARE_DELETE, so the lock file
// survives process exit and is never deleted through the lock handle. The
// exclusive range lock is what serializes writers; closing the handle —
// normally or on process death — releases it to the OS.
func tryLock(path string) (*Lock, error) {
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("homelock: encode lock path: %w", err)
	}
	h, err := windows.CreateFile(
		p16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("homelock: open lock file: %w", err)
	}
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(
		h,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		lockRangeLow,
		lockRangeHigh,
		ol,
	)
	if err != nil {
		_ = windows.CloseHandle(h)
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("homelock: LockFileEx: %w", err)
	}
	return &Lock{
		path: path,
		release: func() error {
			if err := windows.UnlockFileEx(h, 0, lockRangeLow, lockRangeHigh, ol); err != nil {
				_ = windows.CloseHandle(h)
				return fmt.Errorf("homelock: UnlockFileEx: %w", err)
			}
			if err := windows.CloseHandle(h); err != nil {
				return fmt.Errorf("homelock: close lock handle: %w", err)
			}
			return nil
		},
	}, nil
}
