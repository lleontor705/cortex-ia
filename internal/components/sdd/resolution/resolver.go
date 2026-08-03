// Package resolution deterministically resolves requested semantic capabilities
// without executing or probing an external runtime.
package resolution

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type State string

const (
	StateNative      State = "native"
	StateEmulated    State = "emulated"
	StateAdvisory    State = "advisory"
	StateUnsupported State = "unsupported"
)

type BindingKind string

const (
	BindingNative    BindingKind = "native"
	BindingEmulation BindingKind = "emulation"
	BindingAdvisory  BindingKind = "advisory"
)

type GuaranteeLevel string

const (
	GuaranteeEnforced   GuaranteeLevel = "enforced"
	GuaranteeEquivalent GuaranteeLevel = "equivalent"
	GuaranteeBestEffort GuaranteeLevel = "best-effort"
	GuaranteeNone       GuaranteeLevel = "none"
)

type BindingID = ir.SemanticID
type EvidenceRef string

type PermissionDelta struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// Binding describes one evidenced way to provide a semantic capability. Kind
// describes delivery while Enforcement independently describes the mechanism.
type Binding struct {
	ID              BindingID                   `json:"id"`
	CapabilityID    capability.CapabilityID     `json:"capability_id"`
	Kind            BindingKind                 `json:"kind"`
	Evidence        []EvidenceRef               `json:"evidence"`
	Guarantee       GuaranteeLevel              `json:"guarantee"`
	Enforcement     capability.EnforcementClass `json:"enforcement"`
	PermissionDelta PermissionDelta             `json:"permission_delta"`
}

type Request struct {
	ID                  capability.CapabilityID   `json:"id"`
	Required            bool                      `json:"required"`
	EnforcementRequired bool                      `json:"enforcement_required"`
	Substitutions       []capability.CapabilityID `json:"substitutions,omitempty"`
}

type Resolution struct {
	ID              capability.CapabilityID `json:"id"`
	State           State                   `json:"state"`
	Binding         Binding                 `json:"binding"`
	Evidence        []EvidenceRef           `json:"evidence"`
	Guarantee       GuaranteeLevel          `json:"guarantee"`
	Substitution    capability.CapabilityID `json:"substitution,omitempty"`
	PermissionDelta PermissionDelta         `json:"permission_delta"`
	Reason          string                  `json:"reason"`
}

// BlockedError indicates that a complete resolution cannot satisfy a hard
// request. Callers can still inspect the returned Resolution for diagnostics.
type BlockedError struct {
	ID     capability.CapabilityID
	State  State
	Reason string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("capability %q blocked in state %q: %s", e.ID, e.State, e.Reason)
}

type candidate struct {
	binding      Binding
	state        State
	substitution capability.CapabilityID
}

// Resolve chooses the strongest available binding independent of input order.
// A substitution is eligible only when the request declares its capability ID.
func Resolve(request Request, bindings []Binding) (Resolution, error) {
	unsupported := unsupportedResolution(request.ID, "no direct binding or declared substitution is available")
	if err := ir.ValidateSemanticID(request.ID); err != nil {
		unsupported.Reason = "capability request is invalid"
		return unsupported, fmt.Errorf("invalid capability request: %w", err)
	}

	declared := make(map[capability.CapabilityID]struct{}, len(request.Substitutions))
	for _, substitution := range request.Substitutions {
		if err := ir.ValidateSemanticID(substitution); err != nil {
			unsupported.Reason = "declared substitution is invalid"
			return unsupported, fmt.Errorf("invalid substitution %q: %w", substitution, err)
		}
		declared[substitution] = struct{}{}
	}

	candidates := make([]candidate, 0, len(bindings))
	seenBindings := make(map[BindingID]struct{}, len(bindings))
	for _, binding := range bindings {
		substitution := capability.CapabilityID("")
		if binding.CapabilityID != request.ID {
			if _, ok := declared[binding.CapabilityID]; !ok {
				continue
			}
			substitution = binding.CapabilityID
		}
		if err := binding.validate(); err != nil {
			unsupported.Reason = "candidate binding is invalid"
			return unsupported, fmt.Errorf("binding %q: %w", binding.ID, err)
		}
		if _, duplicate := seenBindings[binding.ID]; duplicate {
			unsupported.Reason = "candidate binding ID is ambiguous"
			return unsupported, fmt.Errorf("binding ID %q is duplicated", binding.ID)
		}
		seenBindings[binding.ID] = struct{}{}
		state := bindingState(binding.Kind)
		if substitution != "" && state == StateNative {
			state = StateEmulated
		}
		candidates = append(candidates, candidate{binding: binding.normalize(), state: state, substitution: substitution})
	}

	if len(candidates) == 0 {
		if request.Required {
			unsupported.Reason = "required capability is unsupported"
			return unsupported, blocked(unsupported)
		}
		return unsupported, nil
	}

	slices.SortFunc(candidates, compareCandidate)
	selected := candidates[0]
	resolution := Resolution{
		ID:              request.ID,
		State:           selected.state,
		Binding:         selected.binding,
		Evidence:        slices.Clone(selected.binding.Evidence),
		Guarantee:       selected.binding.Guarantee,
		Substitution:    selected.substitution,
		PermissionDelta: selected.binding.PermissionDelta,
		Reason:          selectionReason(selected),
	}
	if request.EnforcementRequired && resolution.State == StateAdvisory {
		resolution.Reason = "enforcement-required capability is advisory only"
		return resolution, blocked(resolution)
	}
	return resolution, nil
}

func (b Binding) validate() error {
	if err := ir.ValidateSemanticID(b.ID); err != nil {
		return err
	}
	if err := ir.ValidateSemanticID(b.CapabilityID); err != nil {
		return err
	}
	if bindingState(b.Kind) == StateUnsupported {
		return errors.New("binding kind must be native, emulation, or advisory")
	}
	if !validGuarantee(b.Guarantee) || b.Guarantee == GuaranteeNone {
		return errors.New("binding guarantee is required")
	}
	if !validEnforcement(b.Enforcement) {
		return errors.New("binding enforcement class is invalid")
	}
	if len(b.Evidence) == 0 {
		return errors.New("binding evidence is required")
	}
	for _, evidence := range b.Evidence {
		if strings.TrimSpace(string(evidence)) == "" {
			return errors.New("binding evidence reference is empty")
		}
	}
	return nil
}

func (b Binding) normalize() Binding {
	normalized := b
	normalized.Evidence = sortedUnique(b.Evidence)
	normalized.PermissionDelta = PermissionDelta{
		Added:   sortedUnique(b.PermissionDelta.Added),
		Removed: sortedUnique(b.PermissionDelta.Removed),
	}
	return normalized
}

func unsupportedResolution(id capability.CapabilityID, reason string) Resolution {
	return Resolution{
		ID:        id,
		State:     StateUnsupported,
		Evidence:  []EvidenceRef{},
		Guarantee: GuaranteeNone,
		PermissionDelta: PermissionDelta{
			Added:   []string{},
			Removed: []string{},
		},
		Reason: reason,
	}
}

func blocked(resolution Resolution) *BlockedError {
	return &BlockedError{ID: resolution.ID, State: resolution.State, Reason: resolution.Reason}
}

func bindingState(kind BindingKind) State {
	switch kind {
	case BindingNative:
		return StateNative
	case BindingEmulation:
		return StateEmulated
	case BindingAdvisory:
		return StateAdvisory
	default:
		return StateUnsupported
	}
}

func compareCandidate(left, right candidate) int {
	if difference := stateRank(left.state) - stateRank(right.state); difference != 0 {
		return difference
	}
	return strings.Compare(string(left.binding.ID), string(right.binding.ID))
}

func stateRank(state State) int {
	switch state {
	case StateNative:
		return 0
	case StateEmulated:
		return 1
	case StateAdvisory:
		return 2
	default:
		return 3
	}
}

func selectionReason(selected candidate) string {
	if selected.substitution != "" {
		return "selected declared substitution"
	}
	return "selected direct " + string(selected.state) + " binding"
}

func validGuarantee(guarantee GuaranteeLevel) bool {
	return guarantee == GuaranteeEnforced || guarantee == GuaranteeEquivalent ||
		guarantee == GuaranteeBestEffort || guarantee == GuaranteeNone
}

func validEnforcement(enforcement capability.EnforcementClass) bool {
	return enforcement == capability.EnforcementRuntime || enforcement == capability.EnforcementHook ||
		enforcement == capability.EnforcementMCP || enforcement == capability.EnforcementPrompt ||
		enforcement == capability.EnforcementNone
}

func sortedUnique[T ~string](values []T) []T {
	if values == nil {
		return []T{}
	}
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}
