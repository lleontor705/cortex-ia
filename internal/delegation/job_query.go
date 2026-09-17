package delegation

import (
	"context"
	"time"
)

// JobView provides a coherent single-snapshot view of a delegation job,
// combining execution state, receipt availability, cancellation facts,
// and reconciliation status.
type JobView struct {
	Job
	Receipt          *Receipt `json:"receipt,omitempty"`
	ReceiptAvailable bool     `json:"receipt_available"`
	ReceiptMissing   bool     `json:"receipt_missing,omitempty"`
	WaitTimedOut     bool     `json:"wait_timed_out,omitempty"`
}

// Query returns a coherent JobView combining job metadata and any available receipt.
// Reads are performed outside long-lived transactions so callers do not hold open locks.
func (s *Store) Query(ctx context.Context, id string) (JobView, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return JobView{}, err
	}

	view := JobView{
		Job: job,
	}

	receipt, err := s.Result(ctx, id)
	if err == nil {
		view.Receipt = &receipt
		view.ReceiptAvailable = true
	} else if isTerminalStatus(job.Status) {
		view.ReceiptMissing = true
	}

	return view, nil
}

// Wait polls for a delegation job to reach a terminal state within the specified timeout.
// Polling takes place outside transactions at discrete intervals.
func (s *Store) Wait(ctx context.Context, id string, timeout time.Duration) (JobView, error) {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		view, err := s.Query(ctx, id)
		if err != nil {
			return JobView{}, err
		}
		if isTerminalStatus(view.Status) {
			return view, nil
		}

		select {
		case <-ctx.Done():
			return view, ctx.Err()
		case now := <-ticker.C:
			if now.After(deadline) {
				view.WaitTimedOut = true
				return view, nil
			}
		}
	}
}

func isTerminalStatus(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusCancelled, StatusLost:
		return true
	default:
		return false
	}
}
