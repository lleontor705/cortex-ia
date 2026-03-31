package update

import (
	"fmt"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"v0.1.0", "0.1.0"},
		{"0.1.0", "0.1.0"},
		{"v1.2.3", "1.2.3"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeVersion(tt.input)
		if got != tt.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheck_DevVersion(t *testing.T) {
	result := Check("dev")
	if !result.UpToDate {
		t.Error("dev version should always be up to date")
	}
	if result.Error != nil {
		t.Errorf("dev check should not error: %v", result.Error)
	}
}

func TestCheck_EmptyVersion(t *testing.T) {
	result := Check("")
	if !result.UpToDate {
		t.Error("empty version should be treated as up to date")
	}
}

func TestFormatCheckResult_UpToDate(t *testing.T) {
	r := CheckResult{CurrentVersion: "v0.1.0", UpToDate: true}
	msg := FormatCheckResult(r)
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestFormatCheckResult_Error(t *testing.T) {
	r := CheckResult{Error: fmt.Errorf("network error")}
	msg := FormatCheckResult(r)
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}
