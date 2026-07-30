package renderers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

const (
	windsurfTarget                TargetID = "windsurf"
	windsurfManifestSchemaVersion string   = "1.0.0"
)

// WindsurfRenderer lowers a resolved workflow to project-scoped Windsurf rules
// and a machine-readable disclosure. It is pure; bundle validation remains the
// responsibility of Render.
type WindsurfRenderer struct{}

func NewWindsurfRenderer() WindsurfRenderer { return WindsurfRenderer{} }

func (WindsurfRenderer) Target() TargetID { return windsurfTarget }

func (WindsurfRenderer) Render(ctx context.Context, resolved ResolvedWorkflow) (Bundle, error) {
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}
	if resolved.Profile != "portable-sequential" {
		return Bundle{}, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", "$.profile", resolved.Profile, "portable-sequential for the conservative Windsurf renderer")
	}
	if !windsurfValidFingerprint(resolved.GenerationFingerprint) {
		return Bundle{}, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", "$.generation_fingerprint", resolved.GenerationFingerprint, "a lowercase SHA-256 generation fingerprint")
	}

	permissions := sortedUnique(resolved.AllowedPermissions)
	capabilities, err := windsurfCapabilities(resolved.Capabilities, permissions)
	if err != nil {
		return Bundle{}, err
	}

	workflow := windsurfNormalizeWorkflow(resolved.Workflow)
	disclosure := windsurfDisclosure{
		SchemaVersion:         windsurfManifestSchemaVersion,
		WorkflowID:            resolved.Workflow.ID,
		WorkflowVersion:       resolved.Workflow.Version,
		Target:                resolved.Target,
		Profile:               resolved.Profile,
		GenerationFingerprint: resolved.GenerationFingerprint,
		PortableSemanticIDs:   windsurfPortableSemanticIDs(resolved.Workflow),
		Roles:                 workflow.Roles,
		Phases:                workflow.Phases,
		Tools:                 workflow.Tools,
		Context:               workflow.Context.Classes,
		Services:              workflow.Services,
		Profiles:              workflow.Profiles,
		RequestedPermissions:  permissions,
		EffectivePermissions:  slices.Clone(permissions),
		Capabilities:          capabilities,
	}
	machine, err := json.Marshal(disclosure)
	if err != nil {
		return Bundle{}, fmt.Errorf("marshal Windsurf disclosure: %w", err)
	}
	machine = append(machine, '\n')

	return Bundle{Assets: []Asset{
		{
			Path:       ".windsurf/cortex-ia-manifest.json",
			SemanticID: "manifest/windsurf/disclosure",
			Kind:       AssetFixture,
			Content:    machine,
			Mode:       0o644,
		},
		{
			Path:        ".windsurf/rules/cortex-ia.md",
			SemanticID:  resolved.Workflow.ID,
			Kind:        AssetRule,
			Content:     windsurfRules(resolved, permissions, capabilities),
			Mode:        0o644,
			Permissions: slices.Clone(permissions),
		},
	}}, nil
}

func windsurfValidFingerprint(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type windsurfDisclosure struct {
	SchemaVersion         string                         `json:"schema_version"`
	WorkflowID            ir.SemanticID                  `json:"workflow_id"`
	WorkflowVersion       ir.Version                     `json:"workflow_version"`
	Target                TargetID                       `json:"target"`
	Profile               string                         `json:"profile"`
	GenerationFingerprint string                         `json:"generation_fingerprint"`
	PortableSemanticIDs   []ir.SemanticID                `json:"portable_semantic_ids"`
	Roles                 []ir.Role                      `json:"roles"`
	Phases                []ir.Phase                     `json:"phases"`
	Tools                 []ir.ToolRequirement           `json:"tools"`
	Context               []ir.TrustClass                `json:"context_trust_classes"`
	Services              []ir.ServiceRequirement        `json:"services"`
	Profiles              []ir.Profile                   `json:"profiles"`
	RequestedPermissions  []string                       `json:"requested_permissions"`
	EffectivePermissions  []string                       `json:"effective_permissions"`
	Capabilities          []windsurfCapabilityDisclosure `json:"capabilities"`
}

func windsurfNormalizeWorkflow(input ir.WorkflowIR) ir.WorkflowIR {
	result := input
	result.Roles = slices.Clone(input.Roles)
	for index := range result.Roles {
		role := &result.Roles[index]
		role.Inputs = slices.Clone(role.Inputs)
		role.Outputs = slices.Clone(role.Outputs)
		slices.SortFunc(role.Inputs, func(left, right ir.Contract) int { return strings.Compare(string(left.ID), string(right.ID)) })
		slices.SortFunc(role.Outputs, func(left, right ir.Contract) int { return strings.Compare(string(left.ID), string(right.ID)) })
		role.NonGoals = sortedUnique(role.NonGoals)
		role.AllowedEffects = sortedUnique(role.AllowedEffects)
		role.Evidence = sortedUnique(role.Evidence)
		role.TerminalStates = sortedUnique(role.TerminalStates)
	}
	slices.SortFunc(result.Roles, func(left, right ir.Role) int { return strings.Compare(string(left.ID), string(right.ID)) })

	result.Phases = slices.Clone(input.Phases)
	for index := range result.Phases {
		result.Phases[index].DependsOn = sortedUnique(result.Phases[index].DependsOn)
	}
	slices.SortFunc(result.Phases, func(left, right ir.Phase) int { return strings.Compare(string(left.ID), string(right.ID)) })
	result.Tools = slices.Clone(input.Tools)
	slices.SortFunc(result.Tools, func(left, right ir.ToolRequirement) int { return strings.Compare(string(left.ID), string(right.ID)) })
	result.Context.Classes = sortedUnique(input.Context.Classes)
	result.Services = slices.Clone(input.Services)
	slices.SortFunc(result.Services, func(left, right ir.ServiceRequirement) int { return strings.Compare(string(left.ID), string(right.ID)) })
	result.Profiles = slices.Clone(input.Profiles)
	slices.SortFunc(result.Profiles, func(left, right ir.Profile) int { return strings.Compare(string(left.ID), string(right.ID)) })
	return result
}

type windsurfCapabilityDisclosure struct {
	ID              ir.SemanticID               `json:"id"`
	State           resolution.State            `json:"state"`
	Substitution    ir.SemanticID               `json:"substitution"`
	Enforcement     capability.EnforcementClass `json:"enforcement"`
	Guarantee       resolution.GuaranteeLevel   `json:"guarantee"`
	PermissionDelta resolution.PermissionDelta  `json:"permission_delta"`
	Reason          string                      `json:"reason"`
}

func windsurfCapabilities(input []resolution.Resolution, allowedPermissions []string) ([]windsurfCapabilityDisclosure, error) {
	allowed := make(map[string]struct{}, len(allowedPermissions))
	for _, permission := range allowedPermissions {
		allowed[permission] = struct{}{}
	}

	result := make([]windsurfCapabilityDisclosure, 0, len(input))
	for _, item := range input {
		delta := resolution.PermissionDelta{
			Added:   sortedUnique(item.PermissionDelta.Added),
			Removed: sortedUnique(item.PermissionDelta.Removed),
		}
		for _, permission := range delta.Added {
			if _, ok := allowed[permission]; !ok {
				return nil, validationError(
					ErrorPermissionWidening,
					item.ID,
					"$.capabilities.permission_delta.added",
					permission,
					"a permission present in the canonical resolved scope",
				)
			}
		}

		enforcement := item.Binding.Enforcement
		if enforcement == "" {
			enforcement = capability.EnforcementNone
		}
		guarantee := item.Guarantee
		if guarantee == "" {
			guarantee = resolution.GuaranteeNone
		}
		result = append(result, windsurfCapabilityDisclosure{
			ID:              item.ID,
			State:           item.State,
			Substitution:    item.Substitution,
			Enforcement:     enforcement,
			Guarantee:       guarantee,
			PermissionDelta: delta,
			Reason:          item.Reason,
		})
	}
	slices.SortFunc(result, func(left, right windsurfCapabilityDisclosure) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return result, nil
}

func windsurfPortableSemanticIDs(workflow ir.WorkflowIR) []ir.SemanticID {
	ids := []ir.SemanticID{workflow.ID}
	for _, role := range workflow.Roles {
		ids = append(ids, role.ID)
		for _, contract := range role.Inputs {
			ids = append(ids, contract.ID)
		}
		for _, contract := range role.Outputs {
			ids = append(ids, contract.ID)
		}
		ids = append(ids, role.Evidence...)
		for _, effect := range role.AllowedEffects {
			ids = append(ids, ir.SemanticID(effect))
		}
	}
	for _, phase := range workflow.Phases {
		ids = append(ids, phase.ID)
	}
	for _, tool := range workflow.Tools {
		ids = append(ids, tool.ID)
	}
	for _, service := range workflow.Services {
		ids = append(ids, service.ID)
	}
	for _, profile := range workflow.Profiles {
		ids = append(ids, profile.ID)
	}
	return sortedUnique(ids)
}

func windsurfRules(resolved ResolvedWorkflow, permissions []string, capabilities []windsurfCapabilityDisclosure) []byte {
	roles := slices.Clone(resolved.Workflow.Roles)
	slices.SortFunc(roles, func(left, right ir.Role) int { return strings.Compare(string(left.ID), string(right.ID)) })
	phases := slices.Clone(resolved.Workflow.Phases)
	slices.SortFunc(phases, func(left, right ir.Phase) int { return strings.Compare(string(left.ID), string(right.ID)) })
	tools := slices.Clone(resolved.Workflow.Tools)
	slices.SortFunc(tools, func(left, right ir.ToolRequirement) int { return strings.Compare(string(left.ID), string(right.ID)) })
	trustClasses := sortedUnique(resolved.Workflow.Context.Classes)

	var output strings.Builder
	fmt.Fprintln(&output, "# cortex-ia workflow")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Target: `%s`\n", resolved.Target)
	fmt.Fprintf(&output, "- Profile: `%s`\n", resolved.Profile)
	fmt.Fprintf(&output, "- Workflow semantic ID: `%s`\n", resolved.Workflow.ID)

	fmt.Fprintln(&output, "\n## Portable roles")
	for _, role := range roles {
		fmt.Fprintf(&output, "- `%s`: %s\n", role.ID, role.Objective)
	}
	fmt.Fprintln(&output, "\n## Portable phases")
	for _, phase := range phases {
		dependencies := sortedUnique(phase.DependsOn)
		fmt.Fprintf(&output, "- `%s` -> `%s`; depends on: %s\n", phase.ID, phase.Role, windsurfMarkdownValues(dependencies))
	}
	fmt.Fprintln(&output, "\n## Semantic tools")
	for _, tool := range tools {
		fmt.Fprintf(&output, "- `%s` (required: %t)\n", tool.ID, tool.Required)
	}
	fmt.Fprintln(&output, "\n## Trust boundaries")
	for _, trustClass := range trustClasses {
		fmt.Fprintf(&output, "- `%s`\n", trustClass)
	}
	fmt.Fprintln(&output, "\n## Permission ceiling")
	for _, permission := range permissions {
		fmt.Fprintf(&output, "- `%s`\n", permission)
	}
	fmt.Fprintln(&output, "\n## Capability disclosures")
	for _, item := range capabilities {
		fmt.Fprintf(
			&output,
			"- `%s`: state=%s; substitution=%s; enforcement=%s; guarantee=%s; added=%s; removed=%s; reason=%s\n",
			item.ID,
			item.State,
			windsurfMarkdownValue(item.Substitution),
			item.Enforcement,
			item.Guarantee,
			windsurfMarkdownValues(item.PermissionDelta.Added),
			windsurfMarkdownValues(item.PermissionDelta.Removed),
			item.Reason,
		)
	}
	return []byte(output.String())
}

func windsurfMarkdownValue[T ~string](value T) string {
	if value == "" {
		return "none"
	}
	return "`" + string(value) + "`"
}

func windsurfMarkdownValues[T ~string](values []T) string {
	if len(values) == 0 {
		return "none"
	}
	formatted := make([]string, len(values))
	for index, value := range values {
		formatted[index] = windsurfMarkdownValue(value)
	}
	return strings.Join(formatted, ", ")
}
