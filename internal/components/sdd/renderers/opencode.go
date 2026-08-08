package renderers

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

const (
	openCodeTarget          TargetID                = "opencode"
	openCodeNativeExtension ir.SemanticID           = "opencode/native-advanced"
	directChildDelegation   capability.CapabilityID = "delegation/direct-child"
	nestedDelegation        capability.CapabilityID = "delegation/nested"
	ErrorOpenCodeProfile    ir.SemanticID           = "renderer/opencode/profile"
	ErrorOpenCodeWorkflow   ir.SemanticID           = "renderer/opencode/workflow"
)

// OpenCodeProfileError reports a profile that cannot be lowered without
// claiming runtime behavior absent from the resolved capability evidence.
type OpenCodeProfileError struct {
	ID      ir.SemanticID
	Profile string
	Reason  string
}

func (e *OpenCodeProfileError) Error() string {
	return fmt.Sprintf("OpenCode renderer %s for profile %q: %s", e.ID, e.Profile, e.Reason)
}

// OpenCodeRenderer lowers canonical workflow roles into OpenCode instructions,
// commands, and direct-child agent definitions. It performs no filesystem I/O.
type OpenCodeRenderer struct{}

func NewOpenCodeRenderer() OpenCodeRenderer { return OpenCodeRenderer{} }

func (OpenCodeRenderer) Target() TargetID { return openCodeTarget }

func (OpenCodeRenderer) Render(_ context.Context, resolved ResolvedWorkflow) (Bundle, error) {
	if err := validateOpenCodeProfile(resolved); err != nil {
		return Bundle{}, err
	}
	phases, err := orderedOpenCodePhases(resolved.Workflow)
	if err != nil {
		return Bundle{}, err
	}

	assets := []Asset{
		{
			Path:       "AGENTS.md",
			SemanticID: "asset/opencode/instruction/root",
			Kind:       AssetInstruction,
			Content:    renderOpenCodeInstructions(resolved, phases),
			Mode:       0o644,
		},
		{
			Path:       "commands/run-workflow.md",
			SemanticID: "asset/opencode/command/workflow",
			Kind:       AssetCommand,
			Content:    renderOpenCodeCommand(resolved),
			Mode:       0o644,
			Extensions: openCodeCommandExtensions(resolved.Profile),
		},
	}

	if resolved.Profile != "portable-sequential" && hasCanonicalOpenCodeCoreV2Composition(resolved) {
		agents, agentErr := renderOpenCodeCoreV2Agents(resolved)
		if agentErr != nil {
			return Bundle{}, agentErr
		}
		assets = append(assets, agents...)
		return Bundle{Assets: assets}, nil
	}
	if resolved.Profile != "portable-sequential" {
		roles := slices.Clone(resolved.Workflow.Roles)
		slices.SortFunc(roles, func(left, right ir.Role) int {
			return strings.Compare(string(left.ID), string(right.ID))
		})
		for _, role := range roles {
			name := openCodeSemanticName(role.ID)
			permissions := make([]string, 0, len(role.AllowedEffects))
			for _, effect := range role.AllowedEffects {
				permissions = append(permissions, string(effect))
			}
			assets = append(assets, Asset{
				Path: "agents/" + name + ".md", SemanticID: ir.SemanticID("asset/opencode/agent/" + name), Kind: AssetAgent,
				Content: renderOpenCodeCompatibilityAgent(role), Mode: 0o644, Permissions: permissions,
			})
		}
	}
	assets, err = appendCompositionAsset(resolved, assets)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{Assets: assets}, nil
}

func validateOpenCodeProfile(resolved ResolvedWorkflow) error {
	switch resolved.Profile {
	case "portable-sequential":
		return nil
	case "portable-flat":
		if !hasQualifiedOpenCodeCapability(resolved.Capabilities, directChildDelegation) {
			return &OpenCodeProfileError{ID: ErrorOpenCodeProfile, Profile: resolved.Profile, Reason: "direct-child delegation is not qualified"}
		}
		return nil
	case "native-advanced":
		if !hasQualifiedOpenCodeCapability(resolved.Capabilities, directChildDelegation) ||
			!hasQualifiedOpenCodeCapability(resolved.Capabilities, nestedDelegation) ||
			!hasOpenCodeExtension(resolved.Extensions, openCodeNativeExtension) {
			return &OpenCodeProfileError{ID: ErrorOpenCodeProfile, Profile: resolved.Profile, Reason: "qualified native delegation and explicit opencode/native-advanced opt-in are required"}
		}
		return nil
	default:
		return &OpenCodeProfileError{ID: ErrorOpenCodeProfile, Profile: resolved.Profile, Reason: "supported profiles are portable-sequential, portable-flat, and native-advanced"}
	}
}

func hasQualifiedOpenCodeCapability(resolutions []resolution.Resolution, id capability.CapabilityID) bool {
	for _, item := range resolutions {
		if item.ID != id || item.State != resolution.StateNative || item.Guarantee != resolution.GuaranteeEnforced {
			continue
		}
		if item.Binding.Kind != resolution.BindingNative || item.Binding.Enforcement != capability.EnforcementRuntime || len(item.Evidence) == 0 {
			continue
		}
		if len(item.PermissionDelta.Added) != 0 || len(item.Binding.PermissionDelta.Added) != 0 {
			continue
		}
		return true
	}
	return false
}

func hasOpenCodeExtension(extensions []ExtensionDeclaration, id ir.SemanticID) bool {
	return slices.ContainsFunc(extensions, func(extension ExtensionDeclaration) bool { return extension.ID == id })
}

func orderedOpenCodePhases(workflow ir.WorkflowIR) ([]ir.Phase, error) {
	if err := ir.ValidateSemanticID(workflow.ID); err != nil {
		return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: "workflow ID must be canonical"}
	}
	roles := make(map[ir.SemanticID]struct{}, len(workflow.Roles))
	for _, role := range workflow.Roles {
		if err := ir.ValidateSemanticID(role.ID); err != nil {
			return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: "role ID must be canonical"}
		}
		roles[role.ID] = struct{}{}
	}

	byID := make(map[ir.SemanticID]ir.Phase, len(workflow.Phases))
	remainingDependencies := make(map[ir.SemanticID]map[ir.SemanticID]struct{}, len(workflow.Phases))
	for _, phase := range workflow.Phases {
		if err := ir.ValidateSemanticID(phase.ID); err != nil {
			return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: "phase ID must be canonical"}
		}
		if _, exists := byID[phase.ID]; exists {
			return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: "phase IDs must be unique"}
		}
		if _, exists := roles[phase.Role]; !exists {
			return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: fmt.Sprintf("phase %s references an unknown role", phase.ID)}
		}
		byID[phase.ID] = phase
		remainingDependencies[phase.ID] = make(map[ir.SemanticID]struct{}, len(phase.DependsOn))
		for _, dependency := range phase.DependsOn {
			remainingDependencies[phase.ID][dependency] = struct{}{}
		}
	}
	for phaseID, dependencies := range remainingDependencies {
		for dependency := range dependencies {
			if _, exists := byID[dependency]; !exists {
				return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: fmt.Sprintf("phase %s references unknown dependency %s", phaseID, dependency)}
			}
		}
	}

	ordered := make([]ir.Phase, 0, len(workflow.Phases))
	for len(ordered) < len(workflow.Phases) {
		ready := make([]ir.SemanticID, 0)
		for id, dependencies := range remainingDependencies {
			if len(dependencies) == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			return nil, &OpenCodeProfileError{ID: ErrorOpenCodeWorkflow, Reason: "phase dependencies contain a cycle"}
		}
		slices.Sort(ready)
		for _, id := range ready {
			ordered = append(ordered, byID[id])
			delete(remainingDependencies, id)
		}
		for _, dependencies := range remainingDependencies {
			for _, id := range ready {
				delete(dependencies, id)
			}
		}
	}
	return ordered, nil
}

func renderOpenCodeInstructions(resolved ResolvedWorkflow, phases []ir.Phase) []byte {
	var output strings.Builder
	fmt.Fprintln(&output, "# OpenCode Workflow Bundle")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Workflow: `%s`\n", resolved.Workflow.ID)
	fmt.Fprintf(&output, "- Version: `%s`\n", resolved.Workflow.Version.String())
	fmt.Fprintf(&output, "- Profile: `%s`\n", resolved.Profile)
	fmt.Fprintf(&output, "- Generation fingerprint: `%s`\n", resolved.GenerationFingerprint)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Execution")
	fmt.Fprintln(&output, openCodeExecutionRule(resolved.Profile))
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Phases")
	for _, phase := range phases {
		dependencies := slices.Clone(phase.DependsOn)
		slices.Sort(dependencies)
		fmt.Fprintf(&output, "- `%s` -> `%s`", phase.ID, phase.Role)
		if len(dependencies) > 0 {
			fmt.Fprintf(&output, " (after: `%s`)", strings.Join(openCodeSemanticStrings(dependencies), "`, `"))
		}
		fmt.Fprintln(&output)
	}
	return []byte(output.String())
}

func renderOpenCodeCommand(resolved ResolvedWorkflow) []byte {
	var output strings.Builder
	fmt.Fprintln(&output, "---")
	fmt.Fprintf(&output, "description: Execute %s using %s\n", resolved.Workflow.ID, resolved.Profile)
	fmt.Fprintln(&output, "---")
	fmt.Fprintf(&output, "Execute `%s` phase-by-phase in deterministic dependency order.\n", resolved.Workflow.ID)
	fmt.Fprintln(&output, openCodeExecutionRule(resolved.Profile))
	return []byte(output.String())
}

var canonicalOpenCodeSkills = map[ir.SemanticID]ir.SemanticID{
	"role/bootstrap": "skill/bootstrap", "role/investigate": "skill/investigate", "role/draft-proposal": "skill/draft-proposal",
	"role/write-specs": "skill/write-specs", "role/architect": "skill/architect", "role/decompose": "skill/decompose",
	"role/implement": "skill/implement", "role/validate": "skill/validate", "role/finalize": "skill/finalize",
}

// renderOpenCodeCoreV2Agents is the sole Core V2 role producer. It lowers the
// nine composition bindings directly, rather than emitting objective-only role
// files alongside the composition stubs.
func renderOpenCodeCoreV2Agents(resolved ResolvedWorkflow) ([]Asset, error) {
	if len(resolved.Workflow.Roles) != len(canonicalOpenCodeSkills) || len(resolved.Composition.SkillBindings) != len(canonicalOpenCodeSkills) {
		return nil, fmt.Errorf("OpenCode Core V2 requires exactly %d canonical role bindings", len(canonicalOpenCodeSkills))
	}
	roles := make(map[ir.SemanticID]ir.Role, len(resolved.Workflow.Roles))
	for _, role := range resolved.Workflow.Roles {
		if _, exists := canonicalOpenCodeSkills[role.ID]; !exists {
			return nil, fmt.Errorf("OpenCode Core V2 role %q is not canonical", role.ID)
		}
		roles[role.ID] = role
	}
	bindings := make(map[ir.SemanticID]SkillBinding, len(resolved.Composition.SkillBindings))
	for _, binding := range resolved.Composition.SkillBindings {
		if want, ok := canonicalOpenCodeSkills[binding.Role]; !ok || binding.Skill != want || binding.Path == "" {
			return nil, fmt.Errorf("OpenCode Core V2 role %q has an invalid canonical skill binding", binding.Role)
		}
		if _, exists := bindings[binding.Role]; exists {
			return nil, fmt.Errorf("OpenCode Core V2 role %q has duplicate skill bindings", binding.Role)
		}
		bindings[binding.Role] = binding
	}

	roleIDs := make([]ir.SemanticID, 0, len(canonicalOpenCodeSkills))
	for roleID := range canonicalOpenCodeSkills {
		if _, ok := roles[roleID]; !ok {
			return nil, fmt.Errorf("OpenCode Core V2 is missing canonical role %q", roleID)
		}
		if _, ok := bindings[roleID]; !ok {
			return nil, fmt.Errorf("OpenCode Core V2 is missing canonical skill binding for %q", roleID)
		}
		roleIDs = append(roleIDs, roleID)
	}
	slices.Sort(roleIDs)
	agents := make([]Asset, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role := roles[roleID]
		name := openCodeSemanticName(roleID)
		permissions := make([]string, 0, len(role.AllowedEffects))
		for _, effect := range role.AllowedEffects {
			permissions = append(permissions, string(effect))
		}
		agents = append(agents, Asset{
			Path: "agents/" + name + ".md", SemanticID: ir.SemanticID("asset/opencode/agent/" + name), Kind: AssetAgent,
			Content: renderOpenCodeCoreV2Agent(role, bindings[roleID]), Mode: 0o644, Permissions: permissions,
		})
	}
	return agents, nil
}

func hasCanonicalOpenCodeCoreV2Composition(resolved ResolvedWorkflow) bool {
	return len(resolved.Workflow.Roles) == len(canonicalOpenCodeSkills) && len(resolved.Composition.SkillBindings) == len(canonicalOpenCodeSkills)
}

func renderOpenCodeCoreV2Agent(role ir.Role, binding SkillBinding) []byte {
	var output strings.Builder
	fmt.Fprintln(&output, "---")
	fmt.Fprintf(&output, "description: %s\n", strconv.Quote(role.Objective))
	fmt.Fprintln(&output, "mode: subagent")
	if role.ID == "role/bootstrap" {
		fmt.Fprintln(&output, "tools:")
		fmt.Fprintln(&output, "  question: true")
		fmt.Fprintln(&output, "permission:")
		fmt.Fprintln(&output, "  question: allow")
	}
	fmt.Fprintln(&output, "---")
	fmt.Fprintln(&output)
	name := openCodeSemanticName(role.ID)
	fmt.Fprintf(&output, "# %s\n\n## First action\n\nLoad the mapped skill `%s` before any phase work.\n\n%s\n", name, binding.Skill, role.Objective)
	return []byte(output.String())
}

// renderOpenCodeCompatibilityAgent retains support for incomplete historical
// fixtures. A complete canonical composition always takes the Core V2 path.
func renderOpenCodeCompatibilityAgent(role ir.Role) []byte {
	var output strings.Builder
	fmt.Fprintln(&output, "---")
	fmt.Fprintf(&output, "description: %s\n", strconv.Quote(role.Objective))
	fmt.Fprintln(&output, "mode: subagent")
	fmt.Fprintln(&output, "---")
	fmt.Fprintln(&output)
	name := openCodeSemanticName(role.ID)
	fmt.Fprintf(&output, "# %s\n\n%s\n", name, role.Objective)
	return []byte(output.String())
}

func openCodeExecutionRule(profile string) string {
	switch profile {
	case "portable-sequential":
		return "Execute phases sequentially. Do not delegate. Complete each phase before the next."
	case "portable-flat":
		return "Delegate each ready phase directly to its role agent. Do not request nested delegation."
	default:
		return "Use only qualified OpenCode native delegation. Nested delegation is enabled by explicit operator opt-in."
	}
}

func openCodeCommandExtensions(profile string) []ir.SemanticID {
	if profile == "native-advanced" {
		return []ir.SemanticID{openCodeNativeExtension}
	}
	return nil
}

func openCodeSemanticName(id ir.SemanticID) string {
	value := string(id)
	return value[strings.LastIndexByte(value, '/')+1:]
}

func openCodeSemanticStrings(ids []ir.SemanticID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = string(id)
	}
	return result
}
