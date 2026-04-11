package tui

import (
	"testing"
)

func TestTextInsert_AtStart(t *testing.T) {
	text, pos := textInsert("hello", 0, "X")
	if text != "Xhello" {
		t.Errorf("got %q, want %q", text, "Xhello")
	}
	if pos != 1 {
		t.Errorf("pos = %d, want 1", pos)
	}
}

func TestTextInsert_AtEnd(t *testing.T) {
	text, pos := textInsert("hello", 5, "!")
	if text != "hello!" {
		t.Errorf("got %q, want %q", text, "hello!")
	}
	if pos != 6 {
		t.Errorf("pos = %d, want 6", pos)
	}
}

func TestTextInsert_InMiddle(t *testing.T) {
	text, pos := textInsert("helo", 2, "l")
	if text != "hello" {
		t.Errorf("got %q, want %q", text, "hello")
	}
	if pos != 3 {
		t.Errorf("pos = %d, want 3", pos)
	}
}

func TestTextInsert_EmptyString(t *testing.T) {
	text, pos := textInsert("", 0, "a")
	if text != "a" {
		t.Errorf("got %q, want %q", text, "a")
	}
	if pos != 1 {
		t.Errorf("pos = %d, want 1", pos)
	}
}

func TestTextBackspace_AtStart(t *testing.T) {
	text, pos := textBackspace("hello", 0)
	if text != "hello" {
		t.Errorf("got %q, want %q", text, "hello")
	}
	if pos != 0 {
		t.Errorf("pos = %d, want 0", pos)
	}
}

func TestTextBackspace_AtEnd(t *testing.T) {
	text, pos := textBackspace("hello", 5)
	if text != "hell" {
		t.Errorf("got %q, want %q", text, "hell")
	}
	if pos != 4 {
		t.Errorf("pos = %d, want 4", pos)
	}
}

func TestTextBackspace_InMiddle(t *testing.T) {
	text, pos := textBackspace("hello", 3)
	if text != "helo" {
		t.Errorf("got %q, want %q", text, "helo")
	}
	if pos != 2 {
		t.Errorf("pos = %d, want 2", pos)
	}
}

func TestTextDelete_AtEnd(t *testing.T) {
	text := textDelete("hello", 5)
	if text != "hello" {
		t.Errorf("got %q, want %q", text, "hello")
	}
}

func TestTextDelete_AtStart(t *testing.T) {
	text := textDelete("hello", 0)
	if text != "ello" {
		t.Errorf("got %q, want %q", text, "ello")
	}
}

func TestTextDelete_InMiddle(t *testing.T) {
	text := textDelete("hello", 2)
	if text != "helo" {
		t.Errorf("got %q, want %q", text, "helo")
	}
}

func TestTextRenderWithCursor_Empty(t *testing.T) {
	result := textRenderWithCursor("", 0)
	if result != "_" {
		t.Errorf("got %q, want %q", result, "_")
	}
}

func TestTextRenderWithCursor_AtEnd(t *testing.T) {
	result := textRenderWithCursor("abc", 3)
	if result != "abc_" {
		t.Errorf("got %q, want %q", result, "abc_")
	}
}

func TestTextRenderWithCursor_InMiddle(t *testing.T) {
	result := textRenderWithCursor("abc", 1)
	if result != "a_bc" {
		t.Errorf("got %q, want %q", result, "a_bc")
	}
}

func TestClampPos_InRange(t *testing.T) {
	pos := clampPos("hello", 3)
	if pos != 3 {
		t.Errorf("got %d, want 3", pos)
	}
}

func TestClampPos_Negative(t *testing.T) {
	pos := clampPos("hello", -5)
	if pos != 0 {
		t.Errorf("got %d, want 0", pos)
	}
}

func TestClampPos_OverLength(t *testing.T) {
	pos := clampPos("hello", 100)
	if pos != 5 {
		t.Errorf("got %d, want 5", pos)
	}
}
