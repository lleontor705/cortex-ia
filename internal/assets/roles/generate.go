// Package roles mechanically renders runtime-neutral role and orchestrator
// contracts from canonical workflow semantic IDs.
package roles

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Profile is the portable execution shape used to lower role assets.
type Profile string

const (
	ProfilePortableSequential Profile = "portable-sequential"
	ProfilePortableFlat       Profile = "portable-flat"
	ProfileNativeAdvanced     Profile = "native-advanced"
)

// Asset is one deterministic generated prompt asset.
type Asset struct {
	Path       string
	SemanticID ir.SemanticID
	Content    []byte
}

// Generate emits a thin orchestrator plus one contract for every phase role.
// It derives filenames, references, and contract fields from canonical IDs;
// it does not schedule work or duplicate ForgeSpec authority. Every profile
// routes directly; the retired team-lead coordinator is not representable.
func Generate(workflow ir.WorkflowIR, profile Profile) ([]Asset, error) {
	if err := validateProfile(profile); err != nil {
		return nil, err
	}
	rolesByID, err := indexRoles(workflow.Roles)
	if err != nil {
		return nil, err
	}
	phases, phaseRoleIDs, err := normalizePhases(workflow.Phases, rolesByID)
	if err != nil {
		return nil, err
	}

	orchestratorID := ir.SemanticID("role/orchestrator")
	orchestrator, ok := rolesByID[orchestratorID]
	if !ok {
		return nil, fmt.Errorf("role asset generation requires %q", orchestratorID)
	}
	assets := []Asset{{
		Path:       "orchestrator.md",
		SemanticID: "asset/orchestrator",
		Content:    renderOrchestrator(workflow, orchestrator, phases, profile),
	}}

	slices.Sort(phaseRoleIDs)
	phaseRoleIDs = slices.Compact(phaseRoleIDs)
	for _, id := range phaseRoleIDs {
		if id == orchestratorID {
			continue
		}
		role := rolesByID[id]
		assets = append(assets, Asset{
			Path:       path.Join("roles", semanticName(id)+".md"),
			SemanticID: ir.SemanticID("asset/" + string(id)),
			Content:    renderRole(role, phasesForRole(phases, id), profile),
		})
	}
	slices.SortFunc(assets, func(left, right Asset) int {
		return strings.Compare(string(left.SemanticID), string(right.SemanticID))
	})
	return assets, nil
}

func validateProfile(profile Profile) error {
	switch profile {
	case ProfilePortableSequential, ProfilePortableFlat, ProfileNativeAdvanced:
		return nil
	default:
		return fmt.Errorf("unsupported role asset profile %q", profile)
	}
}

func indexRoles(input []ir.Role) (map[ir.SemanticID]ir.Role, error) {
	result := make(map[ir.SemanticID]ir.Role, len(input))
	for _, role := range input {
		if err := ir.ValidateSemanticID(role.ID); err != nil {
			return nil, fmt.Errorf("role: %w", err)
		}
		if role.ID == "role/team-lead" {
			return nil, fmt.Errorf("role %q is retired and must not appear in canonical input", role.ID)
		}
		if _, duplicate := result[role.ID]; duplicate {
			return nil, fmt.Errorf("duplicate role semantic ID %q", role.ID)
		}
		if strings.TrimSpace(role.Objective) == "" {
			return nil, fmt.Errorf("role %q objective is required", role.ID)
		}
		result[role.ID] = role
	}
	return result, nil
}

func normalizePhases(input []ir.Phase, roles map[ir.SemanticID]ir.Role) ([]ir.Phase, []ir.SemanticID, error) {
	phases := slices.Clone(input)
	slices.SortFunc(phases, func(left, right ir.Phase) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	roleIDs := make([]ir.SemanticID, 0, len(phases))
	seen := make(map[ir.SemanticID]struct{}, len(phases))
	for index := range phases {
		phase := &phases[index]
		if err := ir.ValidateSemanticID(phase.ID); err != nil {
			return nil, nil, fmt.Errorf("phase: %w", err)
		}
		if _, duplicate := seen[phase.ID]; duplicate {
			return nil, nil, fmt.Errorf("duplicate phase semantic ID %q", phase.ID)
		}
		seen[phase.ID] = struct{}{}
		if phase.ID == "phase/native-coordinate" {
			return nil, nil, fmt.Errorf("phase %q is retired and must not appear in canonical input", phase.ID)
		}
		if _, ok := roles[phase.Role]; !ok {
			return nil, nil, fmt.Errorf("phase %q references unknown role %q", phase.ID, phase.Role)
		}
		phase.DependsOn = sortedSemanticIDs(phase.DependsOn)
		roleIDs = append(roleIDs, phase.Role)
	}
	for _, phase := range phases {
		for _, dependency := range phase.DependsOn {
			if _, ok := seen[dependency]; !ok {
				return nil, nil, fmt.Errorf("phase %q references unknown dependency %q", phase.ID, dependency)
			}
		}
	}
	return phases, roleIDs, nil
}

func renderOrchestrator(workflow ir.WorkflowIR, role ir.Role, phases []ir.Phase, profile Profile) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "# Thin SDD Orchestrator\n\n- Semantic ID: `%s`\n- Workflow: `%s`@`%s`\n- Profile: `%s`\n\n", role.ID, workflow.ID, workflow.Version, profile)
	output.WriteString("## Objective\n\n")
	output.WriteString(role.Objective)
	output.WriteString(" The orchestrator is a thin router: query ForgeSpec readiness, select only a ready phase or bounded work reference, invoke the profile-supported runtime route, validate the returned contract, and stop or hand off. ForgeSpec remains task authority; runtime state is non-authoritative.\n\n")
	output.WriteString("## Routing Mode\n\n")
	switch profile {
	case ProfilePortableSequential:
		output.WriteString("Execute one ready phase contract at a time without delegation.\n\n")
	case ProfilePortableFlat:
		output.WriteString("Delegate ready phase or work references only to direct child roles; nested delegation is forbidden.\n\n")
	case ProfileNativeAdvanced:
		output.WriteString("Use qualified native routing directly; dispatch one bounded ready work reference to implement without an intermediate coordinator.\n\n")
	}
	output.WriteString("## Phase References\n\n")
	for _, phase := range phases {
		fmt.Fprintf(&output, "- `%s` -> `%s`", phase.ID, phase.Role)
		if len(phase.DependsOn) > 0 {
			fmt.Fprintf(&output, "; depends on %s", inlineIDs(phase.DependsOn))
		}
		output.WriteByte('\n')
	}
	output.WriteString("\n## Non-Goals\n\n")
	writeBullets(&output, sortedStrings(role.NonGoals))
	output.WriteString("\n## Terminal States\n\n")
	writeTerminalStates(&output, role.TerminalStates)
	return []byte(output.String())
}

func renderRole(role ir.Role, phases []ir.Phase, profile Profile) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "# Phase Role: %s\n\n- Semantic ID: `%s`\n- Profile: `%s`\n- Objective: %s\n\n", semanticName(role.ID), role.ID, profile, role.Objective)
	output.WriteString("## Phase References\n\n")
	if len(phases) == 0 {
		output.WriteString("- Native coordinator only; no task phase ownership.\n")
	} else {
		for _, phase := range phases {
			fmt.Fprintf(&output, "- `%s`", phase.ID)
			if len(phase.DependsOn) > 0 {
				fmt.Fprintf(&output, "; depends on %s", inlineIDs(phase.DependsOn))
			}
			output.WriteByte('\n')
		}
	}
	output.WriteString("\n## Inputs\n\n")
	writeContracts(&output, role.Inputs)
	output.WriteString("\n## Outputs\n\n")
	writeContracts(&output, role.Outputs)
	output.WriteString("\n## Non-Goals\n\n")
	writeBullets(&output, sortedStrings(role.NonGoals))
	output.WriteString("\n## Allowed Effects\n\n")
	effects := make([]string, len(role.AllowedEffects))
	for index, effect := range role.AllowedEffects {
		effects[index] = string(effect)
	}
	writeCodeBullets(&output, sortedStrings(effects))
	output.WriteString("\n## Required Evidence\n\n")
	writeCodeBullets(&output, semanticStrings(role.Evidence))
	output.WriteString("\n## Terminal States\n\n")
	writeTerminalStates(&output, role.TerminalStates)
	return []byte(output.String())
}

func phasesForRole(phases []ir.Phase, roleID ir.SemanticID) []ir.Phase {
	result := make([]ir.Phase, 0, len(phases))
	for _, phase := range phases {
		if phase.Role == roleID {
			result = append(result, phase)
		}
	}
	return result
}

func writeContracts(output *strings.Builder, contracts []ir.Contract) {
	contracts = slices.Clone(contracts)
	slices.SortFunc(contracts, func(left, right ir.Contract) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	if len(contracts) == 0 {
		output.WriteString("- None.\n")
		return
	}
	for _, contract := range contracts {
		fmt.Fprintf(output, "- `%s` schema `%s` (required: %t)\n", contract.ID, contract.SchemaVersion, contract.Required)
	}
}

func writeBullets(output *strings.Builder, values []string) {
	if len(values) == 0 {
		output.WriteString("- None.\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(output, "- %s\n", value)
	}
}

func writeCodeBullets(output *strings.Builder, values []string) {
	if len(values) == 0 {
		output.WriteString("- None.\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(output, "- `%s`\n", value)
	}
}

func writeTerminalStates(output *strings.Builder, states []ir.TerminalState) {
	values := make([]string, len(states))
	for index, state := range states {
		values[index] = string(state)
	}
	writeCodeBullets(output, sortedStrings(values))
}

func semanticName(id ir.SemanticID) string {
	parts := strings.Split(string(id), "/")
	return parts[len(parts)-1]
}

func inlineIDs(ids []ir.SemanticID) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = "`" + string(id) + "`"
	}
	return strings.Join(values, ", ")
}

func sortedSemanticIDs(values []ir.SemanticID) []ir.SemanticID {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}

func semanticStrings(values []ir.SemanticID) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return sortedStrings(result)
}

func sortedStrings(values []string) []string {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}
