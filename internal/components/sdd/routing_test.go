package sdd

import (
	"strings"
	"testing"
)

func TestInjectOrganicRouting(t *testing.T) {
	initial := "# System Prompt\nSome existing prompt."
	updated := InjectOrganicRouting(initial)

	if !strings.Contains(updated, "<!-- cortex-ia:organic-routing -->") {
		t.Errorf("expected organic-routing marker in updated content")
	}
	if !strings.Contains(updated, "Direct Inline (1–3 files)") {
		t.Errorf("expected Direct Inline rule in updated content")
	}
}
