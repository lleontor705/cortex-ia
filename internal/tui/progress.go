package tui

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/tui/screens"
)

const (
	// ProgressStatusPending indicates a step has not started yet.
	ProgressStatusPending = "pending"
	// ProgressStatusRunning indicates a step is currently executing.
	ProgressStatusRunning = "running"
	// ProgressStatusSucceeded indicates a step completed successfully.
	ProgressStatusSucceeded = "succeeded"
	// ProgressStatusFailed indicates a step failed.
	ProgressStatusFailed = "failed"
)

// ProgressItem represents a single step in the installation progress tracker.
type ProgressItem struct {
	Label  string
	Status string
}

// ProgressState tracks the step-level progress of the installation pipeline.
type ProgressState struct {
	Items   []ProgressItem
	Current int
	Logs    []string
}

// NewProgressState creates a ProgressState from a list of step labels.
func NewProgressState(labels []string) ProgressState {
	items := make([]ProgressItem, 0, len(labels))
	for _, label := range labels {
		items = append(items, ProgressItem{Label: label, Status: ProgressStatusPending})
	}
	return ProgressState{Items: items, Current: -1}
}

// Start marks the given step index as running.
func (p *ProgressState) Start(index int) {
	if index < 0 || index >= len(p.Items) {
		return
	}
	p.Current = index
	p.Items[index].Status = ProgressStatusRunning
}

// Mark sets the status of the given step index and advances the current pointer.
func (p *ProgressState) Mark(index int, status string) {
	if index < 0 || index >= len(p.Items) {
		return
	}
	p.Items[index].Status = status
	if index+1 < len(p.Items) {
		p.Current = index + 1
		return
	}
	p.Current = len(p.Items)
}

// AppendLog adds a formatted log line to the progress state.
func (p *ProgressState) AppendLog(format string, args ...any) {
	p.Logs = append(p.Logs, fmt.Sprintf(format, args...))
}

// Done returns true when all steps have completed (succeeded or failed).
func (p ProgressState) Done() bool {
	return p.Percent() >= 100
}

// Percent returns the completion percentage (0-100).
func (p ProgressState) Percent() int {
	if len(p.Items) == 0 {
		return 100
	}
	completed := 0
	for _, item := range p.Items {
		if item.Status == ProgressStatusSucceeded || item.Status == ProgressStatusFailed {
			completed++
		}
	}
	return (completed * 100) / len(p.Items)
}

// HasFailures returns true if any step has failed.
func (p ProgressState) HasFailures() bool {
	for _, item := range p.Items {
		if item.Status == ProgressStatusFailed {
			return true
		}
	}
	return false
}

// ViewModel converts the progress state to a screens-compatible render model.
func (p ProgressState) ViewModel() screens.InstallProgress {
	items := make([]screens.ProgressItem, 0, len(p.Items))
	for _, item := range p.Items {
		items = append(items, screens.ProgressItem{Label: item.Label, Status: item.Status})
	}
	current := ""
	if p.Current >= 0 && p.Current < len(p.Items) {
		current = p.Items[p.Current].Label
	}
	return screens.InstallProgress{
		Percent:     p.Percent(),
		CurrentStep: current,
		Items:       items,
		Logs:        append([]string(nil), p.Logs...),
		Done:        p.Percent() >= 100,
		Failed:      p.HasFailures(),
	}
}
