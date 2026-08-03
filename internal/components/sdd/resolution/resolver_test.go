package resolution

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

func TestResolveProducesExactlyOneFourStateResolution(t *testing.T) {
	tests := []struct {
		name       string
		request    Request
		bindings   []Binding
		wantState  State
		wantBind   BindingID
		wantSub    capability.CapabilityID
		wantReason string
		wantBlock  bool
	}{
		{
			name:    "native binding preserves complete evidence",
			request: Request{ID: "isolation/writes", Required: true, EnforcementRequired: true},
			bindings: []Binding{{
				ID:           "binding/runtime-isolation",
				CapabilityID: "isolation/writes",
				Kind:         BindingNative,
				Evidence:     []EvidenceRef{"qualification/runtime/isolation"},
				Guarantee:    GuaranteeEnforced,
				Enforcement:  capability.EnforcementRuntime,
				PermissionDelta: PermissionDelta{
					Added:   []string{"filesystem/worktree"},
					Removed: []string{"filesystem/repository"},
				},
			}},
			wantState:  StateNative,
			wantBind:   "binding/runtime-isolation",
			wantReason: "selected direct native binding",
		},
		{
			name: "declared substitution resolves emulated",
			request: Request{
				ID:            "coordination/transactional-claim",
				Required:      true,
				Substitutions: []capability.CapabilityID{"coordination/forgespec-claim"},
			},
			bindings: []Binding{{
				ID:           "binding/forgespec-claim",
				CapabilityID: "coordination/forgespec-claim",
				Kind:         BindingEmulation,
				Evidence:     []EvidenceRef{"service/forgespec/v1"},
				Guarantee:    GuaranteeEquivalent,
				Enforcement:  capability.EnforcementMCP,
			}},
			wantState:  StateEmulated,
			wantBind:   "binding/forgespec-claim",
			wantSub:    "coordination/forgespec-claim",
			wantReason: "selected declared substitution",
		},
		{
			name:    "prompt-only binding resolves advisory",
			request: Request{ID: "policy/stop-on-no-progress"},
			bindings: []Binding{{
				ID:           "binding/prompt-stop",
				CapabilityID: "policy/stop-on-no-progress",
				Kind:         BindingAdvisory,
				Evidence:     []EvidenceRef{"documentation/runtime/instructions"},
				Guarantee:    GuaranteeBestEffort,
				Enforcement:  capability.EnforcementPrompt,
			}},
			wantState:  StateAdvisory,
			wantBind:   "binding/prompt-stop",
			wantReason: "selected direct advisory binding",
		},
		{
			name:       "missing optional binding resolves unsupported",
			request:    Request{ID: "delegation/nested"},
			wantState:  StateUnsupported,
			wantReason: "no direct binding or declared substitution is available",
		},
		{
			name:       "required unsupported blocks",
			request:    Request{ID: "isolation/writes", Required: true},
			wantState:  StateUnsupported,
			wantReason: "required capability is unsupported",
			wantBlock:  true,
		},
		{
			name:    "enforcement-required advisory blocks",
			request: Request{ID: "approval/irreversible", EnforcementRequired: true},
			bindings: []Binding{{
				ID:           "binding/prompt-approval",
				CapabilityID: "approval/irreversible",
				Kind:         BindingAdvisory,
				Evidence:     []EvidenceRef{"documentation/runtime/approval"},
				Guarantee:    GuaranteeBestEffort,
				Enforcement:  capability.EnforcementPrompt,
			}},
			wantState:  StateAdvisory,
			wantBind:   "binding/prompt-approval",
			wantReason: "enforcement-required capability is advisory only",
			wantBlock:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.request, tt.bindings)
			var blocked *BlockedError
			if errors.As(err, &blocked) != tt.wantBlock {
				t.Fatalf("Resolve() error = %v, want blocked=%v", err, tt.wantBlock)
			}
			if got.ID != tt.request.ID || got.State != tt.wantState || got.Binding.ID != tt.wantBind || got.Substitution != tt.wantSub || got.Reason != tt.wantReason {
				t.Errorf("Resolve() = %+v", got)
			}
			if got.Guarantee == "" || got.PermissionDelta.Added == nil || got.PermissionDelta.Removed == nil || got.Evidence == nil {
				t.Errorf("resolution omits required fields: %+v", got)
			}
		})
	}
}

func TestResolveIsDeterministicAcrossBindingOrder(t *testing.T) {
	request := Request{ID: "delegation/direct-child", Required: true}
	preferred := Binding{
		ID:           "binding/runtime-direct-child",
		CapabilityID: request.ID,
		Kind:         BindingNative,
		Evidence:     []EvidenceRef{"evidence/z", "evidence/a", "evidence/a"},
		Guarantee:    GuaranteeEnforced,
		Enforcement:  capability.EnforcementRuntime,
		PermissionDelta: PermissionDelta{
			Added: []string{"tool/task", "tool/task"},
		},
	}
	bindings := []Binding{
		{
			ID:           "binding/prompt-direct-child",
			CapabilityID: request.ID,
			Kind:         BindingAdvisory,
			Evidence:     []EvidenceRef{"documentation/delegation"},
			Guarantee:    GuaranteeBestEffort,
			Enforcement:  capability.EnforcementPrompt,
		},
		preferred,
		{
			ID:           "binding/mcp-direct-child",
			CapabilityID: request.ID,
			Kind:         BindingEmulation,
			Evidence:     []EvidenceRef{"service/forgespec/task-claim"},
			Guarantee:    GuaranteeEquivalent,
			Enforcement:  capability.EnforcementMCP,
		},
	}

	forward, err := Resolve(request, bindings)
	if err != nil {
		t.Fatalf("Resolve(forward) error = %v", err)
	}
	for left, right := 0, len(bindings)-1; left < right; left, right = left+1, right-1 {
		bindings[left], bindings[right] = bindings[right], bindings[left]
	}
	reversed, err := Resolve(request, bindings)
	if err != nil {
		t.Fatalf("Resolve(reversed) error = %v", err)
	}
	if !reflect.DeepEqual(forward, reversed) {
		t.Fatalf("order changed resolution:\nforward=%+v\nreversed=%+v", forward, reversed)
	}
	if !reflect.DeepEqual(forward.Evidence, []EvidenceRef{"evidence/a", "evidence/z"}) {
		t.Errorf("evidence = %v", forward.Evidence)
	}
	if !reflect.DeepEqual(forward.PermissionDelta.Added, []string{"tool/task"}) || len(forward.PermissionDelta.Removed) != 0 {
		t.Errorf("permission delta = %+v", forward.PermissionDelta)
	}
}

func TestResolveRejectsInvalidBindingWithoutNondeterministicFallback(t *testing.T) {
	request := Request{ID: "isolation/writes", Required: true}
	bindings := []Binding{{
		ID:           "binding/native-isolation",
		CapabilityID: request.ID,
		Kind:         BindingNative,
		Guarantee:    GuaranteeEnforced,
		Enforcement:  capability.EnforcementRuntime,
	}}

	got, err := Resolve(request, bindings)
	if err == nil {
		t.Fatal("Resolve() error = nil")
	}
	if got.State != StateUnsupported || got.Reason != "candidate binding is invalid" {
		t.Fatalf("Resolve() = %+v, error = %v", got, err)
	}
}

func TestResolveRejectsDuplicateBindingIDs(t *testing.T) {
	request := Request{ID: "delegation/direct-child", Required: true}
	bindings := []Binding{
		{
			ID:           "binding/runtime-direct-child",
			CapabilityID: request.ID,
			Kind:         BindingNative,
			Evidence:     []EvidenceRef{"qualification/runtime/a"},
			Guarantee:    GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
		{
			ID:           "binding/runtime-direct-child",
			CapabilityID: request.ID,
			Kind:         BindingNative,
			Evidence:     []EvidenceRef{"qualification/runtime/b"},
			Guarantee:    GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
	}

	got, err := Resolve(request, bindings)
	if err == nil {
		t.Fatal("Resolve() error = nil")
	}
	if got.State != StateUnsupported || got.Reason != "candidate binding ID is ambiguous" {
		t.Fatalf("Resolve() = %+v, error = %v", got, err)
	}
}
