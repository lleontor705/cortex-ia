package styles

import "testing"

func TestSpinnerChar_Cycles(t *testing.T) {
	for i := 0; i < 10; i++ {
		ch := SpinnerChar(i)
		if ch == "" {
			t.Errorf("SpinnerChar(%d) returned empty string", i)
		}
	}
}

func TestSpinnerChar_Wraps(t *testing.T) {
	if SpinnerChar(10) != SpinnerChar(0) {
		t.Errorf("SpinnerChar(10) = %q, want %q (should wrap)", SpinnerChar(10), SpinnerChar(0))
	}
	if SpinnerChar(20) != SpinnerChar(0) {
		t.Errorf("SpinnerChar(20) = %q, want %q (should wrap)", SpinnerChar(20), SpinnerChar(0))
	}
}

func TestLogo_NonEmpty(t *testing.T) {
	if Logo == "" {
		t.Error("Logo constant should not be empty")
	}
}

func TestCursorPrefix_NonEmpty(t *testing.T) {
	if CursorPrefix != "> " {
		t.Errorf("CursorPrefix = %q, want %q", CursorPrefix, "> ")
	}
}
