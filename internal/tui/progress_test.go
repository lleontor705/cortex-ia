package tui

import "testing"

func TestNewProgressState(t *testing.T) {
	ps := NewProgressState([]string{"a", "b", "c"})
	if len(ps.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(ps.Items))
	}
	for i, item := range ps.Items {
		if item.Status != ProgressStatusPending {
			t.Errorf("item %d status = %q, want %q", i, item.Status, ProgressStatusPending)
		}
	}
	if ps.Current != -1 {
		t.Errorf("Current = %d, want -1", ps.Current)
	}
}

func TestProgressStart(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	ps.Start(0)
	if ps.Current != 0 {
		t.Errorf("Current = %d, want 0", ps.Current)
	}
	if ps.Items[0].Status != ProgressStatusRunning {
		t.Errorf("status = %q, want %q", ps.Items[0].Status, ProgressStatusRunning)
	}
}

func TestProgressMark_Succeeded(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	ps.Start(0)
	ps.Mark(0, ProgressStatusSucceeded)
	if ps.Items[0].Status != ProgressStatusSucceeded {
		t.Errorf("status = %q, want %q", ps.Items[0].Status, ProgressStatusSucceeded)
	}
	if ps.Current != 1 {
		t.Errorf("Current = %d, want 1 (should advance)", ps.Current)
	}
}

func TestProgressMark_Failed(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	ps.Start(0)
	ps.Mark(0, ProgressStatusFailed)
	if ps.Items[0].Status != ProgressStatusFailed {
		t.Errorf("status = %q, want %q", ps.Items[0].Status, ProgressStatusFailed)
	}
	if ps.Current != 1 {
		t.Errorf("Current = %d, want 1 (should advance even on failure)", ps.Current)
	}
}

func TestProgressPercent_Empty(t *testing.T) {
	ps := NewProgressState(nil)
	if ps.Percent() != 100 {
		t.Errorf("Percent() = %d, want 100 for empty items", ps.Percent())
	}
}

func TestProgressPercent_Partial(t *testing.T) {
	ps := NewProgressState([]string{"a", "b", "c", "d"})
	ps.Mark(0, ProgressStatusSucceeded)
	ps.Mark(1, ProgressStatusSucceeded)
	if ps.Percent() != 50 {
		t.Errorf("Percent() = %d, want 50", ps.Percent())
	}
}

func TestProgressPercent_All(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	ps.Mark(0, ProgressStatusSucceeded)
	ps.Mark(1, ProgressStatusSucceeded)
	if ps.Percent() != 100 {
		t.Errorf("Percent() = %d, want 100", ps.Percent())
	}
}

func TestProgressDone(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	if ps.Done() {
		t.Error("Done() should be false initially")
	}
	ps.Mark(0, ProgressStatusSucceeded)
	ps.Mark(1, ProgressStatusSucceeded)
	if !ps.Done() {
		t.Error("Done() should be true when all completed")
	}
}

func TestProgressHasFailures(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	if ps.HasFailures() {
		t.Error("HasFailures() should be false initially")
	}
	ps.Mark(0, ProgressStatusFailed)
	if !ps.HasFailures() {
		t.Error("HasFailures() should be true after a failure")
	}
}

func TestProgressAppendLog(t *testing.T) {
	ps := NewProgressState([]string{"a"})
	ps.AppendLog("step %d done", 1)
	ps.AppendLog("step %d done", 2)
	if len(ps.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(ps.Logs))
	}
	if ps.Logs[0] != "step 1 done" {
		t.Errorf("log[0] = %q, want %q", ps.Logs[0], "step 1 done")
	}
	if ps.Logs[1] != "step 2 done" {
		t.Errorf("log[1] = %q, want %q", ps.Logs[1], "step 2 done")
	}
}

func TestProgressViewModel(t *testing.T) {
	ps := NewProgressState([]string{"a", "b"})
	ps.Start(0)
	ps.Mark(0, ProgressStatusSucceeded)
	ps.AppendLog("log line")

	vm := ps.ViewModel()
	if vm.Percent != 50 {
		t.Errorf("vm.Percent = %d, want 50", vm.Percent)
	}
	if vm.CurrentStep != "b" {
		t.Errorf("vm.CurrentStep = %q, want %q", vm.CurrentStep, "b")
	}
	if len(vm.Items) != 2 {
		t.Fatalf("vm.Items len = %d, want 2", len(vm.Items))
	}
	if vm.Items[0].Label != "a" || vm.Items[0].Status != ProgressStatusSucceeded {
		t.Errorf("vm.Items[0] = %+v, unexpected", vm.Items[0])
	}
	if len(vm.Logs) != 1 || vm.Logs[0] != "log line" {
		t.Errorf("vm.Logs = %v, unexpected", vm.Logs)
	}
	if vm.Done {
		t.Error("vm.Done should be false")
	}
	if vm.Failed {
		t.Error("vm.Failed should be false")
	}
}

func TestProgressStart_OutOfBounds(t *testing.T) {
	ps := NewProgressState([]string{"a"})
	// Should not panic
	ps.Start(-1)
	ps.Start(5)
	if ps.Current != -1 {
		t.Errorf("Current should remain -1 for out-of-bounds Start, got %d", ps.Current)
	}
}

func TestProgressMark_OutOfBounds(t *testing.T) {
	ps := NewProgressState([]string{"a"})
	// Should not panic
	ps.Mark(-1, ProgressStatusSucceeded)
	ps.Mark(5, ProgressStatusFailed)
	if ps.Items[0].Status != ProgressStatusPending {
		t.Errorf("item status should remain pending for out-of-bounds Mark")
	}
}
