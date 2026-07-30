// Package tools defines canonical semantic tool-binding contracts. It contains
// no provider implementation or runtime execution behavior.
package tools

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var (
	ErrBindingNotFound  = errors.New("semantic tool binding not found")
	ErrAmbiguousBinding = errors.New("semantic tool binding precedence is ambiguous")
)

type ContractKind string

const (
	ContractSchema     ContractKind = "schema"
	ContractDocumented ContractKind = "documented"
)

type EnforcementClass string

const (
	EnforcementRuntime EnforcementClass = "runtime"
	EnforcementHook    EnforcementClass = "hook"
	EnforcementMCP     EnforcementClass = "mcp"
	EnforcementPrompt  EnforcementClass = "prompt"
	EnforcementNone    EnforcementClass = "none"
)

type Effect = ir.Effect

const (
	EffectRead    Effect = "effect/read"
	EffectWrite   Effect = "effect/write"
	EffectProcess Effect = "effect/process"
	EffectNetwork Effect = "effect/network"
)

type VersionInterval = ir.VersionRange

type ContractRef struct {
	Kind    ContractKind `json:"kind"`
	Ref     string       `json:"ref"`
	Version ir.Version   `json:"version"`
}

type PermissionScope struct {
	Effects   []Effect `json:"effects"`
	Resources []string `json:"resources"`
}

type ErrorMapping struct {
	ProviderCode string `json:"provider_code"`
	SemanticCode string `json:"semantic_code"`
}

type Evidence struct {
	Kind   string      `json:"kind"`
	Schema ContractRef `json:"schema"`
}

// Binding maps one stable semantic tool capability to a provider tool. Lower
// precedence values win; equal winning values are rejected as ambiguous.
type Binding struct {
	SchemaVersion ir.Version       `json:"schema_version"`
	ID            ir.SemanticID    `json:"id"`
	SemanticID    ir.SemanticID    `json:"semantic_id"`
	Provider      string           `json:"provider"`
	TargetTool    string           `json:"target_tool"`
	Precedence    uint16           `json:"precedence"`
	Input         ContractRef      `json:"input"`
	Output        ContractRef      `json:"output"`
	Permission    PermissionScope  `json:"permission"`
	Enforcement   EnforcementClass `json:"enforcement"`
	Versions      VersionInterval  `json:"versions"`
	Errors        []ErrorMapping   `json:"errors"`
	Evidence      []Evidence       `json:"evidence"`
}

type Requirement struct {
	SemanticID      ir.SemanticID `json:"semantic_id"`
	ExplicitBinding ir.SemanticID `json:"explicit_binding,omitempty"`
}

func (b Binding) Validate() error {
	if b.SchemaVersion.Major == 0 ||
		strings.TrimSpace(b.Provider) == "" ||
		strings.TrimSpace(b.TargetTool) == "" || b.Precedence == 0 {
		return errors.New("binding identity and precedence are required")
	}
	if err := ir.ValidateSemanticID(b.ID); err != nil {
		return fmt.Errorf("binding ID: %w", err)
	}
	if err := ir.ValidateSemanticID(b.SemanticID); err != nil {
		return fmt.Errorf("semantic tool ID: %w", err)
	}
	if err := validateContractRef("input", b.Input); err != nil {
		return err
	}
	if err := validateContractRef("output", b.Output); err != nil {
		return err
	}
	if len(b.Permission.Effects) == 0 || len(b.Permission.Resources) == 0 {
		return errors.New("permission effects and resources are required")
	}
	if !validEnforcement(b.Enforcement) {
		return fmt.Errorf("invalid enforcement class %q", b.Enforcement)
	}
	if !validVersionInterval(b.Versions) {
		return errors.New("provider version interval is required")
	}
	if len(b.Errors) == 0 {
		return errors.New("error mapping is required")
	}
	for _, mapping := range b.Errors {
		if strings.TrimSpace(mapping.ProviderCode) == "" || strings.TrimSpace(mapping.SemanticCode) == "" {
			return errors.New("error mapping codes are required")
		}
	}
	if len(b.Evidence) == 0 {
		return errors.New("produced evidence contract is required")
	}
	for _, evidence := range b.Evidence {
		if strings.TrimSpace(evidence.Kind) == "" {
			return errors.New("evidence kind is required")
		}
		if err := validateContractRef("evidence", evidence.Schema); err != nil {
			return err
		}
	}
	return nil
}

// ValidateProviderSurface rejects provider bindings that recreate retired
// built-in coordination tools or permissions. Binding consumers opt into this
// gate when producing current assets; bounded legacy decoders may still parse
// historical records without making them selectable.
func ValidateProviderSurface(binding Binding) error {
	if field, value := retiredCoordinationSurface(binding); field != "" {
		return fmt.Errorf("%s %q uses a retired coordination provider surface", field, value)
	}
	return nil
}

func retiredCoordinationSurface(binding Binding) (string, string) {
	values := []struct {
		field string
		value string
	}{
		{field: "binding ID", value: string(binding.ID)},
		{field: "semantic tool ID", value: string(binding.SemanticID)},
		{field: "provider", value: binding.Provider},
		{field: "target tool", value: binding.TargetTool},
	}
	for _, resource := range binding.Permission.Resources {
		values = append(values, struct{ field, value string }{field: "permission resource", value: resource})
	}
	for _, evidence := range binding.Evidence {
		values = append(values, struct{ field, value string }{field: "evidence kind", value: evidence.Kind})
	}
	for _, candidate := range values {
		value := strings.ToLower(candidate.value)
		for _, retired := range []string{"agent-mailbox", "mailbox", "msg_", "msg-", "a2a", "resource_", "resource-", "dlq"} {
			if strings.Contains(value, retired) {
				return candidate.field, candidate.value
			}
		}
	}
	return "", ""
}

func (b Binding) Normalize() Binding {
	normalized := b
	normalized.Permission.Effects = sortedUnique(b.Permission.Effects)
	normalized.Permission.Resources = sortedUnique(b.Permission.Resources)
	normalized.Errors = slices.Clone(b.Errors)
	slices.SortFunc(normalized.Errors, func(a, z ErrorMapping) int {
		return strings.Compare(a.ProviderCode+"\x00"+a.SemanticCode, z.ProviderCode+"\x00"+z.SemanticCode)
	})
	normalized.Evidence = slices.Clone(b.Evidence)
	slices.SortFunc(normalized.Evidence, func(a, z Evidence) int {
		return strings.Compare(a.Kind+"\x00"+a.Schema.Ref, z.Kind+"\x00"+z.Schema.Ref)
	})
	return normalized
}

// SelectBinding applies the canonical precedence: an explicit binding ID wins;
// otherwise the unique lowest numeric precedence wins. Input order never affects
// the result, and tied winners are an error rather than an implicit choice.
func SelectBinding(requirement Requirement, bindings []Binding) (Binding, error) {
	if err := ir.ValidateSemanticID(requirement.SemanticID); err != nil {
		return Binding{}, fmt.Errorf("%w: semantic ID is required", ErrBindingNotFound)
	}
	candidates := make([]Binding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.SemanticID != requirement.SemanticID {
			continue
		}
		if err := binding.Validate(); err != nil {
			return Binding{}, fmt.Errorf("binding %q: %w", binding.ID, err)
		}
		if requirement.ExplicitBinding != "" && binding.ID == requirement.ExplicitBinding {
			return binding.Normalize(), nil
		}
		candidates = append(candidates, binding)
	}
	if requirement.ExplicitBinding != "" {
		return Binding{}, fmt.Errorf("%w: explicit binding %q for %q", ErrBindingNotFound, requirement.ExplicitBinding, requirement.SemanticID)
	}
	if len(candidates) == 0 {
		return Binding{}, fmt.Errorf("%w: %q", ErrBindingNotFound, requirement.SemanticID)
	}
	slices.SortFunc(candidates, func(a, z Binding) int {
		if a.Precedence < z.Precedence {
			return -1
		}
		if a.Precedence > z.Precedence {
			return 1
		}
		return strings.Compare(string(a.ID), string(z.ID))
	})
	if len(candidates) > 1 && candidates[0].Precedence == candidates[1].Precedence {
		return Binding{}, fmt.Errorf("%w: %q precedence %d", ErrAmbiguousBinding, requirement.SemanticID, candidates[0].Precedence)
	}
	return candidates[0].Normalize(), nil
}

func validateContractRef(name string, ref ContractRef) error {
	if ref.Kind != ContractSchema && ref.Kind != ContractDocumented {
		return fmt.Errorf("%s contract kind %q is invalid", name, ref.Kind)
	}
	if strings.TrimSpace(ref.Ref) == "" || ref.Version.Major == 0 {
		return fmt.Errorf("%s contract reference and version are required", name)
	}
	return nil
}

func validVersionInterval(interval VersionInterval) bool {
	minimum, maximum := interval.Minimum, interval.MaximumTested
	if minimum.Major == 0 || maximum.Major != minimum.Major {
		return false
	}
	if maximum.Minor < minimum.Minor {
		return false
	}
	return maximum.Minor != minimum.Minor || maximum.Patch >= minimum.Patch
}

func validEnforcement(class EnforcementClass) bool {
	return class == EnforcementRuntime || class == EnforcementHook || class == EnforcementMCP ||
		class == EnforcementPrompt || class == EnforcementNone
}

func sortedUnique[T ~string](values []T) []T {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}
