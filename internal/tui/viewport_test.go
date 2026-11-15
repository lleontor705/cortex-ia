package tui

import (
	"testing"
)

// TestCursorOffsetCalculation exercises boundary cases of cursor-relative offset computation.
func TestCursorOffsetCalculation(t *testing.T) {
	// Zero/negative budget returns 0.
	if got := cursorOffset(5, 10, 0, 2, 1); got != 0 {
		t.Fatalf("expected 0 for budget 0, got %d", got)
	}

	// Content fits within content budget returns 0.
	// budget 20, top 2, bottom 1 -> content budget = 17 >= totalLines 10.
	if got := cursorOffset(8, 10, 20, 2, 1); got != 0 {
		t.Fatalf("expected 0 when content fits, got %d", got)
	}

	// Cursor within initial keep returns 0.
	// budget 8, top 2, bottom 1 -> content budget = 5, keep = 4.
	// cursorLine 2 < keep 4 -> offset = 0.
	if got := cursorOffset(2, 10, 8, 2, 1); got != 0 {
		t.Fatalf("expected 0 when cursorLine < keep, got %d", got)
	}

	// Cursor beyond keep scrolls.
	// budget 8, top 2, bottom 1 -> content budget = 5, keep = 4.
	// cursorLine 5 -> offset = 5 - 4 + 1 = 2.
	if got := cursorOffset(5, 10, 8, 2, 1); got != 2 {
		t.Fatalf("expected 2 when cursorLine is 5 with keep 4, got %d", got)
	}

	// Offset clamped to maxOffset (totalLines - keep = 10 - 4 = 6).
	// cursorLine 9 -> unconstrained 9 - 4 + 1 = 6.
	if got := cursorOffset(9, 10, 8, 2, 1); got != 6 {
		t.Fatalf("expected 6 (maxOffset) when cursorLine is at end, got %d", got)
	}
}
