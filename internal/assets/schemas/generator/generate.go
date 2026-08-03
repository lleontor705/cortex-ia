// Package phaseassets mechanically derives phase skills, commands, contract
// schemas, and fixtures from canonical workflow semantic IDs.
package phaseassets

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets/roles"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Options controls profile-specific generation. Every profile routes
// directly; the retired team-lead coordinator is not representable.
type Options struct {
	Profile roles.Profile
}

// Asset is one deterministic generated phase asset.
type Asset struct {
	Path       string
	SemanticID ir.SemanticID
	Content    []byte
}

// Generate derives all phase assets from WorkflowIR. Commands intentionally
// reference generated skills instead of repeating their semantic contracts.
func Generate(workflow ir.WorkflowIR, options Options) ([]Asset, error) {
	if err := validateProfile(options.Profile); err != nil {
		return nil, err
	}
	if err := ir.ValidateSemanticID(workflow.ID); err != nil {
		return nil, fmt.Errorf("workflow: %w", err)
	}

	rolesByID, err := canonicalRoles(workflow.Roles)
	if err != nil {
		return nil, err
	}
	phases, err := canonicalPhases(workflow.Phases, rolesByID)
	if err != nil {
		return nil, err
	}

	assets := make([]Asset, 0, len(phases)*4)
	paths := make(map[string]ir.SemanticID, len(phases)*4)
	schemas := make(map[ir.SemanticID]ir.Contract)
	for _, phase := range phases {
		role := rolesByID[phase.Role]
		name := semanticName(phase.ID)
		phaseAssets := []Asset{
			{
				Path:       path.Join("skills", name, "SKILL.md"),
				SemanticID: ir.SemanticID("asset/skill/" + name),
				Content:    renderSkill(workflow, phase, role, options.Profile),
			},
			{
				Path:       path.Join("opencode", "commands", name+".md"),
				SemanticID: ir.SemanticID("asset/command/" + name),
				Content:    renderCommand(workflow, phase, options.Profile),
			},
			{
				Path:       path.Join("schemas", "fixtures", name+".json"),
				SemanticID: ir.SemanticID("asset/fixture/" + name),
				Content:    renderFixture(workflow, phase, role, options.Profile),
			},
		}
		for _, asset := range phaseAssets {
			if err := appendUniqueAsset(&assets, paths, asset); err != nil {
				return nil, err
			}
		}
		for _, contract := range role.Outputs {
			if previous, exists := schemas[contract.ID]; exists && previous.SchemaVersion != contract.SchemaVersion {
				return nil, fmt.Errorf("output contract %q has conflicting schema versions %q and %q", contract.ID, previous.SchemaVersion, contract.SchemaVersion)
			}
			schemas[contract.ID] = contract
		}
	}

	contractIDs := make([]ir.SemanticID, 0, len(schemas))
	for id := range schemas {
		contractIDs = append(contractIDs, id)
	}
	slices.Sort(contractIDs)
	for _, id := range contractIDs {
		name := semanticName(id)
		asset := Asset{
			Path:       path.Join("schemas", name+".schema.json"),
			SemanticID: ir.SemanticID("asset/schema/" + name),
			Content:    renderSchema(workflow, schemas[id]),
		}
		if err := appendUniqueAsset(&assets, paths, asset); err != nil {
			return nil, err
		}
	}

	slices.SortFunc(assets, func(left, right Asset) int {
		return strings.Compare(string(left.SemanticID), string(right.SemanticID))
	})
	return assets, nil
}

func validateProfile(profile roles.Profile) error {
	switch profile {
	case roles.ProfilePortableSequential, roles.ProfilePortableFlat, roles.ProfileNativeAdvanced:
		return nil
	default:
		return fmt.Errorf("unsupported phase asset profile %q", profile)
	}
}

func canonicalRoles(input []ir.Role) (map[ir.SemanticID]ir.Role, error) {
	result := make(map[ir.SemanticID]ir.Role, len(input))
	for _, role := range input {
		if err := ir.ValidateSemanticID(role.ID); err != nil {
			return nil, fmt.Errorf("role: %w", err)
		}
		if role.ID == "role/team-lead" {
			return nil, fmt.Errorf("role %q is retired and must not appear in canonical input", role.ID)
		}
		if _, exists := result[role.ID]; exists {
			return nil, fmt.Errorf("duplicate role semantic ID %q", role.ID)
		}
		if strings.TrimSpace(role.Objective) == "" {
			return nil, fmt.Errorf("role %q objective is required", role.ID)
		}
		for _, contract := range append(slices.Clone(role.Inputs), role.Outputs...) {
			if err := ir.ValidateSemanticID(contract.ID); err != nil {
				return nil, fmt.Errorf("role %q contract: %w", role.ID, err)
			}
		}
		result[role.ID] = role
	}
	return result, nil
}

func canonicalPhases(input []ir.Phase, rolesByID map[ir.SemanticID]ir.Role) ([]ir.Phase, error) {
	result := slices.Clone(input)
	slices.SortFunc(result, func(left, right ir.Phase) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	known := make(map[ir.SemanticID]struct{}, len(result))
	included := make(map[ir.SemanticID]struct{}, len(result))
	filtered := result[:0]
	for _, phase := range result {
		if err := ir.ValidateSemanticID(phase.ID); err != nil {
			return nil, fmt.Errorf("phase: %w", err)
		}
		if _, exists := known[phase.ID]; exists {
			return nil, fmt.Errorf("duplicate phase semantic ID %q", phase.ID)
		}
		known[phase.ID] = struct{}{}
		if phase.ID == "phase/native-coordinate" {
			return nil, fmt.Errorf("phase %q is retired and must not appear in canonical input", phase.ID)
		}
		if _, exists := rolesByID[phase.Role]; !exists {
			return nil, fmt.Errorf("phase %q references unknown role %q", phase.ID, phase.Role)
		}
		phase.DependsOn = sortedSemanticIDs(phase.DependsOn)
		filtered = append(filtered, phase)
		included[phase.ID] = struct{}{}
	}
	for _, phase := range filtered {
		for _, dependency := range phase.DependsOn {
			if _, exists := known[dependency]; !exists {
				return nil, fmt.Errorf("phase %q references unknown dependency %q", phase.ID, dependency)
			}
			if _, exists := included[dependency]; !exists {
				return nil, fmt.Errorf("phase %q depends on profile-excluded phase %q", phase.ID, dependency)
			}
		}
	}
	return filtered, nil
}

func appendUniqueAsset(assets *[]Asset, paths map[string]ir.SemanticID, asset Asset) error {
	if err := ir.ValidateSemanticID(asset.SemanticID); err != nil {
		return fmt.Errorf("generated asset: %w", err)
	}
	if previous, exists := paths[asset.Path]; exists {
		return fmt.Errorf("generated path %q collides for %q and %q", asset.Path, previous, asset.SemanticID)
	}
	paths[asset.Path] = asset.SemanticID
	*assets = append(*assets, asset)
	return nil
}

func renderSkill(workflow ir.WorkflowIR, phase ir.Phase, role ir.Role, profile roles.Profile) []byte {
	var output strings.Builder
	name := semanticName(phase.ID)
	fmt.Fprintf(&output, "---\nname: %s\ndescription: %q\n---\n\n", name, role.Objective)
	fmt.Fprintf(&output, "# Generated Phase Skill: %s\n\n", name)
	writeSource(&output, workflow, profile, phase.ID)
	fmt.Fprintf(&output, "## Objective\n\n%s\n\n", role.Objective)
	fmt.Fprintf(&output, "## Canonical Role\n\n- `%s`\n\n", role.ID)
	writeContracts(&output, "Inputs", role.Inputs)
	writeContracts(&output, "Outputs", role.Outputs)
	writeIDs(&output, "Dependencies", phase.DependsOn)
	writeIDs(&output, "Required Evidence", role.Evidence)
	writeTerminalStates(&output, role.TerminalStates)
	return []byte(output.String())
}

func renderCommand(workflow ir.WorkflowIR, phase ir.Phase, profile roles.Profile) []byte {
	name := semanticName(phase.ID)
	var output strings.Builder
	fmt.Fprintf(&output, "---\ndescription: Execute canonical phase %s\nagent: orchestrator\nsubtask: true\n---\n\n", phase.ID)
	writeSource(&output, workflow, profile, phase.ID)
	fmt.Fprintf(&output, "Load `{{HOME}}/.config/opencode/skills/%s/SKILL.md` and execute `%s` exactly as defined there. Return its generated contract without restating or replacing the skill contract.\n", name, phase.ID)
	return []byte(output.String())
}

func renderSchema(workflow ir.WorkflowIR, contract ir.Contract) []byte {
	document := struct {
		Schema            string         `json:"$schema"`
		ID                ir.SemanticID  `json:"$id"`
		Title             string         `json:"title"`
		Type              string         `json:"type"`
		Additional        bool           `json:"additionalProperties"`
		WorkflowID        ir.SemanticID  `json:"x-workflow-id"`
		WorkflowVersion   ir.Version     `json:"x-workflow-version"`
		IRVersion         ir.Version     `json:"x-ir-version"`
		ContractVersion   ir.Version     `json:"x-contract-version"`
		CanonicalContract ir.SemanticID  `json:"x-semantic-id"`
		Required          []string       `json:"required,omitempty"`
		Properties        map[string]any `json:"properties"`
	}{
		Schema:            "https://json-schema.org/draft/2020-12/schema",
		ID:                contract.ID,
		Title:             "Generated contract " + string(contract.ID),
		Type:              "object",
		Additional:        false,
		WorkflowID:        workflow.ID,
		WorkflowVersion:   workflow.Version,
		IRVersion:         workflow.SchemaVersion,
		ContractVersion:   contract.SchemaVersion,
		CanonicalContract: contract.ID,
		Properties: map[string]any{
			"semantic_id": map[string]any{"const": contract.ID},
			"status":      map[string]any{"enum": []string{"success", "partial", "failed", "blocked"}, "type": "string"},
		},
	}
	if contract.Required {
		document.Required = []string{"semantic_id", "status"}
	}
	return marshalJSON(document)
}

func renderFixture(workflow ir.WorkflowIR, phase ir.Phase, role ir.Role, profile roles.Profile) []byte {
	document := struct {
		WorkflowID      ir.SemanticID   `json:"workflow_id"`
		WorkflowVersion ir.Version      `json:"workflow_version"`
		IRVersion       ir.Version      `json:"ir_version"`
		Profile         roles.Profile   `json:"profile"`
		PhaseID         ir.SemanticID   `json:"phase_id"`
		RoleID          ir.SemanticID   `json:"role_id"`
		DependsOn       []ir.SemanticID `json:"depends_on"`
		Inputs          []ir.SemanticID `json:"inputs"`
		Outputs         []ir.SemanticID `json:"outputs"`
	}{
		WorkflowID:      workflow.ID,
		WorkflowVersion: workflow.Version,
		IRVersion:       workflow.SchemaVersion,
		Profile:         profile,
		PhaseID:         phase.ID,
		RoleID:          role.ID,
		DependsOn:       sortedSemanticIDs(phase.DependsOn),
		Inputs:          contractIDs(role.Inputs),
		Outputs:         contractIDs(role.Outputs),
	}
	return marshalJSON(document)
}

func writeSource(output *strings.Builder, workflow ir.WorkflowIR, profile roles.Profile, phaseID ir.SemanticID) {
	fmt.Fprintf(output, "- Semantic ID: `%s`\n- Workflow: `%s`@`%s`\n- IR version: `%s`\n- Profile: `%s`\n\n", phaseID, workflow.ID, workflow.Version, workflow.SchemaVersion, profile)
}

func writeContracts(output *strings.Builder, title string, contracts []ir.Contract) {
	fmt.Fprintf(output, "## %s\n\n", title)
	contracts = slices.Clone(contracts)
	slices.SortFunc(contracts, func(left, right ir.Contract) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	if len(contracts) == 0 {
		output.WriteString("- None.\n\n")
		return
	}
	for _, contract := range contracts {
		fmt.Fprintf(output, "- `%s` schema `%s` (required: %t)\n", contract.ID, contract.SchemaVersion, contract.Required)
	}
	output.WriteByte('\n')
}

func writeIDs(output *strings.Builder, title string, ids []ir.SemanticID) {
	fmt.Fprintf(output, "## %s\n\n", title)
	ids = sortedSemanticIDs(ids)
	if len(ids) == 0 {
		output.WriteString("- None.\n\n")
		return
	}
	for _, id := range ids {
		fmt.Fprintf(output, "- `%s`\n", id)
	}
	output.WriteByte('\n')
}

func writeTerminalStates(output *strings.Builder, states []ir.TerminalState) {
	values := make([]string, len(states))
	for index, state := range states {
		values[index] = string(state)
	}
	slices.Sort(values)
	output.WriteString("## Terminal States\n\n")
	for _, state := range slices.Compact(values) {
		fmt.Fprintf(output, "- `%s`\n", state)
	}
}

func contractIDs(contracts []ir.Contract) []ir.SemanticID {
	result := make([]ir.SemanticID, len(contracts))
	for index, contract := range contracts {
		result[index] = contract.ID
	}
	return sortedSemanticIDs(result)
}

func sortedSemanticIDs(ids []ir.SemanticID) []ir.SemanticID {
	result := slices.Clone(ids)
	slices.Sort(result)
	return slices.Compact(result)
}

func semanticName(id ir.SemanticID) string {
	parts := strings.Split(string(id), "/")
	return parts[len(parts)-1]
}

func marshalJSON(value any) []byte {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal generated phase asset: %v", err))
	}
	return append(content, '\n')
}
