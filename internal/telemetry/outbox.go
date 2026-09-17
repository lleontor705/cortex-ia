package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/homelock"
)

const maxQueuedReports = 128

func lockOutbox(home string) (string, *homelock.Lock, error) {
	dir := filepath.Join(home, ".cortex-ia", "report-outbox")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	lock, err := homelock.Acquire(dir, time.Second)
	return dir, lock, err
}

// EnqueueReport commits sanitized evidence before any network operation.
func EnqueueReport(home string, report *ErrorReport, secret string) error {
	if report == nil || secret == "" {
		return errors.New("authenticated reporting requires a configured signing secret")
	}
	copy := *report
	SanitizeReport(&copy, secret)
	SignReport(&copy, secret)
	if err := ValidateReport(&copy); err != nil {
		return err
	}
	data, err := json.Marshal(copy)
	if err != nil {
		return err
	}
	dir, lock, err := lockOutbox(home)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	destination := filepath.Join(dir, copy.ID+".json")
	if existing, err := os.ReadFile(destination); err == nil {
		if string(existing) == string(data) {
			return nil
		}
		return errors.New("queued report identity conflicts with existing evidence")
	} else if !os.IsNotExist(err) {
		return err
	}
	if count >= maxQueuedReports {
		return errors.New("report outbox is full; flush queued reports before retrying")
	}
	// A fixed pending file keeps crash leftovers bounded to one payload.
	pending := filepath.Join(dir, "pending")
	file, err := os.OpenFile(pending, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pending) }()
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(pending, destination)
}

// FlushReports retries at most eight reports. It removes evidence only after a
// matching acknowledgement; crashes may duplicate delivery but retain stable IDs.
func FlushReports(ctx context.Context, home string, cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Secret == "" {
		return errors.New("authenticated reporting requires a configured signing secret")
	}
	dir, lock, err := lockOutbox(home)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sent := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if sent == 8 {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxReportBytes {
			return errors.New("invalid queued report file")
		}
		file := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var report ErrorReport
		if err := json.Unmarshal(raw, &report); err != nil {
			return errors.New("invalid queued report JSON")
		}
		if entry.Name() != report.ID+".json" || !VerifyReport(&report, cfg.Secret) {
			return errors.New("queued report identity or signature is invalid")
		}
		if _, err := SendReport(ctx, cfg.Endpoint, &report); err != nil {
			return err
		}
		if err := os.Remove(file); err != nil {
			return err
		}
		sent++
	}
	return nil
}
