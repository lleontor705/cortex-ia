package model

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateCurrentComponentsRejectsRetiredMailbox(t *testing.T) {
	err := ValidateCurrentComponents([]ComponentID{ComponentCortex, ComponentMailbox})
	if err == nil {
		t.Fatal("expected retired component selection to fail")
	}

	var retiredErr *RetiredComponentError
	if !errors.As(err, &retiredErr) {
		t.Fatalf("expected RetiredComponentError, got %T: %v", err, err)
	}
	if retiredErr.Component != ComponentMailbox {
		t.Fatalf("component = %q, want %q", retiredErr.Component, ComponentMailbox)
	}
	if !strings.Contains(err.Error(), "retired") || !strings.Contains(err.Error(), "external provider") {
		t.Fatalf("error lacks actionable retirement guidance: %v", err)
	}
}

func TestValidateCurrentComponentsAllowsCurrentComponents(t *testing.T) {
	if err := ValidateCurrentComponents([]ComponentID{ComponentCortex, ComponentForgeSpec}); err != nil {
		t.Fatalf("current components rejected: %v", err)
	}
}

func TestSelectionValidateCurrentUsesRetirementBoundary(t *testing.T) {
	selection := Selection{Components: []ComponentID{ComponentMailbox}}
	if err := selection.ValidateCurrent(); err == nil {
		t.Fatal("current selection accepted retired Mailbox tombstone")
	}
}

func TestMailboxIsDecodeOnlyTombstone(t *testing.T) {
	retired, ok := RetiredComponent(ComponentMailbox)
	if !ok {
		t.Fatal("agent-mailbox tombstone not registered")
	}
	if retired.Selectable || retired.Installable {
		t.Fatalf("retired component is live: %+v", retired)
	}
}
