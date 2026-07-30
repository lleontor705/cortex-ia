package renderers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

const (
	ErrorKiroUnsupportedProfile ir.SemanticID = "kiro/unsupported-profile"
	ErrorKiroUnqualifiedProfile ir.SemanticID = "kiro/unqualified-profile"
	ErrorKiroInvalidWorkflow    ir.SemanticID = "kiro/invalid-workflow"
)

const kiroDirectChild capability.CapabilityID = "delegation/direct-child"

// KiroProfileError blocks rendering when the selected profile would claim a
// Kiro feature that has not been qualified by runtime-enforced evidence.
type KiroProfileError struct {
	ID      ir.SemanticID
	Profile string
	Reason  string
}

func (e *KiroProfileError) Error() string {
	return fmt.Sprintf("Kiro renderer %s for profile %q: %s", e.ID, e.Profile, e.Reason)
}

// KiroRenderer lowers portable workflow semantics into Kiro steering and
// custom-agent files. It intentionally does not advertise native-advanced:
// current Kiro evidence qualifies only opt-in direct-child delegation.
type KiroRenderer struct{}

func NewKiroRenderer() KiroRenderer { return KiroRenderer{} }

func (KiroRenderer) Target() TargetID { return "kiro" }

func (KiroRenderer) Render(_ context.Context, resolved ResolvedWorkflow) (Bundle, error) {
	roles, phases, err := normalizeKiroWorkflow(resolved.Workflow)
	if err != nil {
		return Bundle{}, &KiroProfileError{ID: ErrorKiroInvalidWorkflow, Profile: resolved.Profile, Reason: err.Error()}
	}

	switch resolved.Profile {
	case "portable-sequential":
		content := renderKiroSteering(resolved, roles, phases, false)
		assets := []Asset{kiroSteeringAsset(content)}
		assets, err = appendCompositionAsset(resolved, assets)
		if err != nil {
			return Bundle{}, err
		}
		return Bundle{Assets: assets}, nil
	case "portable-flat":
		if reason := unqualifiedKiroDirectChild(resolved.Capabilities); reason != "" {
			return Bundle{}, &KiroProfileError{ID: ErrorKiroUnqualifiedProfile, Profile: resolved.Profile, Reason: reason}
		}
		assets := make([]Asset, 0, len(roles)+1)
		for _, role := range roles {
			content, marshalErr := json.MarshalIndent(kiroAgentForRole(role), "", "  ")
			if marshalErr != nil {
				return Bundle{}, fmt.Errorf("marshal Kiro agent %q: %w", role.ID, marshalErr)
			}
			content = append(content, '\n')
			name := kiroSemanticName(role.ID)
			assets = append(assets, Asset{
				Path:       "agents/" + name + ".json",
				SemanticID: ir.SemanticID("kiro/agent/" + name),
				Kind:       AssetAgent,
				Content:    content,
				Mode:       0o644,
			})
		}
		assets = append(assets, kiroSteeringAsset(renderKiroSteering(resolved, roles, phases, true)))
		assets, err = appendCompositionAsset(resolved, assets)
		if err != nil {
			return Bundle{}, err
		}
		return Bundle{Assets: assets}, nil
	default:
		return Bundle{}, &KiroProfileError{
			ID:      ErrorKiroUnsupportedProfile,
			Profile: resolved.Profile,
			Reason:  "supported profiles are portable-sequential and qualified portable-flat; native-advanced Kiro guarantees are not advertised",
		}
	}
}

type kiroAgent struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools"`
}

func kiroAgentForRole(role ir.Role) kiroAgent {
	effects := kiroSortedStrings(role.AllowedEffects)
	terminalStates := kiroTerminalStates(role.TerminalStates)
	return kiroAgent{
		Name:        kiroSemanticName(role.ID),
		Description: role.Objective,
		Prompt: fmt.Sprintf(
			"Objective: %s\nAllowed effects: %s\nTerminal states: %s\nReturn a contract for the bounded work unit.",
			role.Objective,
			strings.Join(effects, ", "),
			strings.Join(terminalStates, ", "),
		),
		Tools: []string{},
	}
}

func kiroSteeringAsset(content []byte) Asset {
	return Asset{
		Path:       "steering/cortex-ia.md",
		SemanticID: "kiro/instruction/workflow",
		Kind:       AssetInstruction,
		Content:    content,
		Mode:       0o644,
	}
}

func renderKiroSteering(resolved ResolvedWorkflow, roles []ir.Role, phases []ir.Phase, delegated bool) []byte {
	roleByID := make(map[ir.SemanticID]ir.Role, len(roles))
	for _, role := range roles {
		roleByID[role.ID] = role
	}

	var output strings.Builder
	fmt.Fprintf(&output, "# cortex-ia workflow\n\n- Workflow: `%s`\n- Version: `%s`\n- Profile: `%s`\n- Generation fingerprint: `%s`\n", resolved.Workflow.ID, resolved.Workflow.Version, resolved.Profile, resolved.GenerationFingerprint)
	if delegated {
		output.WriteString("- Delegation: qualified direct-child only; nested delegation is forbidden.\n\n## Flat phases\n\n")
		for _, phase := range phases {
			fmt.Fprintf(&output, "- `%s` delegates directly to `%s`", phase.ID, kiroSemanticName(phase.Role))
			if len(phase.DependsOn) > 0 {
				fmt.Fprintf(&output, "; wait for %s", kiroQuotedSemanticIDs(phase.DependsOn))
			}
			output.WriteByte('\n')
		}
	} else {
		reason := sequentialDegradationReason(resolved.Capabilities)
		fmt.Fprintf(&output, "- Delegation: disabled; phases execute sequentially in dependency order because %s.\n\n## Sequential phases\n\n", reason)
		for index, phase := range phases {
			role := roleByID[phase.Role]
			fmt.Fprintf(&output, "%d. `%s` — role `%s`: %s", index+1, phase.ID, phase.Role, role.Objective)
			if len(phase.DependsOn) > 0 {
				fmt.Fprintf(&output, "; depends on %s", kiroQuotedSemanticIDs(phase.DependsOn))
			}
			output.WriteByte('\n')
		}
	}
	output.WriteString("\nDo not claim parallel, nested, isolated-write, hook, or structured-output enforcement unless it appears as a qualified capability in the generated manifests.\n")
	return []byte(output.String())
}

func normalizeKiroWorkflow(workflow ir.WorkflowIR) ([]ir.Role, []ir.Phase, error) {
	roles := slices.Clone(workflow.Roles)
	slices.SortFunc(roles, func(left, right ir.Role) int { return strings.Compare(string(left.ID), string(right.ID)) })
	roleByID := make(map[ir.SemanticID]struct{}, len(roles))
	for _, role := range roles {
		if err := ir.ValidateSemanticID(role.ID); err != nil || strings.TrimSpace(role.Objective) == "" {
			return nil, nil, fmt.Errorf("role %q requires a semantic ID and objective", role.ID)
		}
		roleByID[role.ID] = struct{}{}
	}

	phases := slices.Clone(workflow.Phases)
	phaseByID := make(map[ir.SemanticID]ir.Phase, len(phases))
	for _, phase := range phases {
		if err := ir.ValidateSemanticID(phase.ID); err != nil {
			return nil, nil, fmt.Errorf("phase %q has an invalid semantic ID", phase.ID)
		}
		if _, exists := phaseByID[phase.ID]; exists {
			return nil, nil, fmt.Errorf("phase %q is duplicated", phase.ID)
		}
		if _, exists := roleByID[phase.Role]; !exists {
			return nil, nil, fmt.Errorf("phase %q references missing role %q", phase.ID, phase.Role)
		}
		phase.DependsOn = kiroSortedSemanticIDs(phase.DependsOn)
		phaseByID[phase.ID] = phase
	}
	for _, phase := range phases {
		for _, dependency := range phase.DependsOn {
			if _, exists := phaseByID[dependency]; !exists {
				return nil, nil, fmt.Errorf("phase %q references missing dependency %q", phase.ID, dependency)
			}
		}
	}
	ordered, ok := topologicalKiroPhases(phaseByID)
	if !ok {
		return nil, nil, fmt.Errorf("phase dependency graph contains a cycle")
	}
	return roles, ordered, nil
}

func topologicalKiroPhases(phases map[ir.SemanticID]ir.Phase) ([]ir.Phase, bool) {
	remaining := make(map[ir.SemanticID]ir.Phase, len(phases))
	for id, phase := range phases {
		remaining[id] = phase
	}
	done := make(map[ir.SemanticID]struct{}, len(phases))
	ordered := make([]ir.Phase, 0, len(phases))
	for len(remaining) > 0 {
		ready := make([]ir.SemanticID, 0)
		for id, phase := range remaining {
			if kiroDependenciesComplete(phase.DependsOn, done) {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			return nil, false
		}
		slices.Sort(ready)
		for _, id := range ready {
			ordered = append(ordered, remaining[id])
			delete(remaining, id)
			done[id] = struct{}{}
		}
	}
	return ordered, true
}

func kiroDependenciesComplete(dependencies []ir.SemanticID, done map[ir.SemanticID]struct{}) bool {
	for _, dependency := range dependencies {
		if _, ok := done[dependency]; !ok {
			return false
		}
	}
	return true
}

func unqualifiedKiroDirectChild(capabilities []resolution.Resolution) string {
	for _, item := range capabilities {
		if item.ID != kiroDirectChild {
			continue
		}
		if item.State == resolution.StateNative && item.Binding.Enforcement == capability.EnforcementRuntime && len(item.Evidence) > 0 && (item.Guarantee == resolution.GuaranteeEnforced || item.Guarantee == resolution.GuaranteeEquivalent) {
			return ""
		}
		if item.Reason != "" {
			return "direct-child delegation is not runtime-qualified: " + item.Reason
		}
		return "direct-child delegation is not runtime-qualified"
	}
	return "direct-child delegation resolution is missing"
}

func sequentialDegradationReason(capabilities []resolution.Resolution) string {
	for _, item := range capabilities {
		if item.ID == kiroDirectChild && item.State != resolution.StateNative {
			if item.Reason != "" {
				return "Kiro direct-child delegation is not qualified (" + item.Reason + ")"
			}
			return "Kiro direct-child delegation is not qualified"
		}
	}
	return "the portable-sequential profile explicitly disables delegation"
}

func kiroSemanticName(id ir.SemanticID) string {
	value := string(id)
	if index := strings.LastIndexByte(value, '/'); index >= 0 {
		return value[index+1:]
	}
	return value
}

func kiroQuotedSemanticIDs(ids []ir.SemanticID) string {
	values := make([]string, len(ids))
	for index, id := range kiroSortedSemanticIDs(ids) {
		values[index] = "`" + string(id) + "`"
	}
	return strings.Join(values, ", ")
}

func kiroSortedSemanticIDs(values []ir.SemanticID) []ir.SemanticID {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}

func kiroSortedStrings[T ~string](values []T) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func kiroTerminalStates(values []ir.TerminalState) []string {
	// Keep the contract's declared order while removing duplicates; terminal
	// state order is meaningful in generated role instructions.
	seen := make(map[ir.TerminalState]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, string(value))
	}
	return result
}
