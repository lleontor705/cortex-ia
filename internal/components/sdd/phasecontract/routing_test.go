package phasecontract

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var trusted = ir.TrustTrustedPolicy
var trustedOperator = ir.TrustOperatorInput

func TestAuthorizeTransitionAllowsValidDAGAndRejectsInvalid(t *testing.T) {
	valid := []struct{ from, to PhaseID }{
		{PhaseBootstrap, PhaseExplore},
		{PhaseExplore, PhaseProposal},
		{PhaseProposal, PhaseSpec},
		{PhaseProposal, PhaseDesign},
		{PhaseSpec, PhaseTasks},
		{PhaseDesign, PhaseTasks},
		{PhaseTasks, PhaseApply},
		{PhaseApply, PhaseVerify},
		{PhaseVerify, PhaseArchive},
	}
	for _, tt := range valid {
		if err := AuthorizeTransition(tt.from, tt.to); err != nil {
			t.Errorf("AuthorizeTransition(%q->%q) error = %v", tt.from, tt.to, err)
		}
	}

	invalid := []struct{ from, to PhaseID }{
		{PhaseBootstrap, PhaseApply},  // skip pipeline
		{PhaseTasks, PhaseProposal},   // backwards
		{PhaseApply, PhaseSpec},       // backwards
		{PhaseExplore, PhaseArchive},  // skip to end
		{PhaseBootstrap, PhaseDesign}, // skip proposal
	}
	for _, tt := range invalid {
		if err := AuthorizeTransition(tt.from, tt.to); err == nil {
			t.Errorf("AuthorizeTransition(%q->%q) error = nil, want rejection", tt.from, tt.to)
		}
	}
}

func TestAuthorizeTransitionRejectsUnknownPhases(t *testing.T) {
	if err := AuthorizeTransition("unknown", PhaseApply); err == nil {
		t.Fatal("AuthorizeTransition from unknown phase error = nil, want rejection")
	}
	if err := AuthorizeTransition(PhaseApply, "unknown"); err == nil {
		t.Fatal("AuthorizeTransition to unknown phase error = nil, want rejection")
	}
}
