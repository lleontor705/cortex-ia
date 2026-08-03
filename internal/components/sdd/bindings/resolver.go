// Package bindings resolves runtime-neutral semantic tool requirements to
// provider bindings. Resolution is pure and deterministic; it performs no tool
// invocation and grants no permissions.
package bindings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/tools"
)

type ErrorCode string

const (
	ErrorMissingRequired      ErrorCode = "missing_required_binding"
	ErrorIncompatibleRequired ErrorCode = "incompatible_required_binding"
	ErrorAmbiguousRequired    ErrorCode = "ambiguous_required_binding"
	ErrorPermissionWidening   ErrorCode = "permission_widening"
)

type Status string

const (
	StatusSelected    Status = "selected"
	StatusUnsupported Status = "unsupported"
)

type Selection string

const (
	SelectionExplicit   Selection = "explicit"
	SelectionPrecedence Selection = "precedence"
)

// SchemaRequirement describes the compatible contract interval required by
// canonical IR. A provider contract is compatible only when kind and reference
// match and its version falls inside Versions.
type SchemaRequirement struct {
	Kind     tools.ContractKind `json:"kind"`
	Ref      string             `json:"ref"`
	Versions ir.VersionRange    `json:"versions"`
}

type Request struct {
	SemanticID          ir.SemanticID         `json:"semantic_id"`
	Required            bool                  `json:"required"`
	ExplicitBinding     ir.SemanticID         `json:"explicit_binding,omitempty"`
	ProviderVersion     ir.Version            `json:"provider_version"`
	Input               SchemaRequirement     `json:"input"`
	Output              SchemaRequirement     `json:"output"`
	CanonicalPermission tools.PermissionScope `json:"canonical_permission"`
	RenderedPermission  tools.PermissionScope `json:"rendered_permission"`
}

type PermissionReport struct {
	Canonical      tools.PermissionScope `json:"canonical"`
	Rendered       tools.PermissionScope `json:"rendered"`
	AddedEffects   []tools.Effect        `json:"added_effects"`
	AddedResources []string              `json:"added_resources"`
}

// Result is the complete evidence-bearing contract consumed by renderers and
// manifests. Unsupported optional requirements return this structure without
// an error so degradation remains explicit.
type Result struct {
	SemanticID    ir.SemanticID        `json:"semantic_id"`
	Status        Status               `json:"status"`
	Selection     Selection            `json:"selection,omitempty"`
	Binding       tools.Binding        `json:"binding"`
	Input         tools.ContractRef    `json:"input"`
	Output        tools.ContractRef    `json:"output"`
	ErrorMappings []tools.ErrorMapping `json:"error_mappings"`
	Evidence      []tools.Evidence     `json:"evidence"`
	Permission    PermissionReport     `json:"permission"`
}

// BlockedError preserves stable diagnostics for machine and human manifests.
type BlockedError struct {
	Code       ErrorCode        `json:"code"`
	SemanticID ir.SemanticID    `json:"semantic_id"`
	Candidates []ir.SemanticID  `json:"candidates"`
	Reason     string           `json:"reason"`
	Permission PermissionReport `json:"permission"`
	Blocking   bool             `json:"blocking"`
}

func (e *BlockedError) Error() string {
	message := fmt.Sprintf("semantic binding %q blocked (%s): %s", e.SemanticID, e.Code, e.Reason)
	if len(e.Candidates) > 0 {
		message += fmt.Sprintf("; candidates=%v", e.Candidates)
	}
	if e.Code == ErrorPermissionWidening {
		message += fmt.Sprintf("; canonical=%v/%v rendered=%v/%v", e.Permission.Canonical.Effects, e.Permission.Canonical.Resources, e.Permission.Rendered.Effects, e.Permission.Rendered.Resources)
	}
	return message
}

type candidate struct {
	binding tools.Binding
	reason  string
}

// Resolve selects an explicit compatible binding first, otherwise the unique
// compatible binding with the lowest numeric precedence. Catalog input order
// never breaks ties: equal winning precedence is a blocking ambiguity.
func Resolve(request Request, catalog []tools.Binding) (Result, error) {
	base := Result{SemanticID: request.SemanticID, Status: StatusUnsupported}
	if err := validateRequest(request); err != nil {
		return base, blocked(request.SemanticID, ErrorIncompatibleRequired, nil, err.Error(), PermissionReport{})
	}

	relevant := make([]candidate, 0, len(catalog))
	compatible := make([]tools.Binding, 0, len(catalog))
	for _, binding := range catalog {
		if binding.SemanticID != request.SemanticID {
			continue
		}
		if request.ExplicitBinding != "" && binding.ID != request.ExplicitBinding {
			continue
		}
		reason := compatibilityError(request, binding)
		relevant = append(relevant, candidate{binding: binding, reason: reason})
		if reason == "" {
			compatible = append(compatible, binding.Normalize())
		}
	}

	if len(compatible) == 0 {
		if !request.Required {
			return base, nil
		}
		code, reason := ErrorMissingRequired, "no binding is declared for the required semantic tool"
		if len(relevant) > 0 {
			code = ErrorIncompatibleRequired
			reason = incompatibleSummary(relevant)
		} else if request.ExplicitBinding != "" {
			reason = fmt.Sprintf("explicit binding %q is not declared for the required semantic tool", request.ExplicitBinding)
		}
		return base, blocked(request.SemanticID, code, candidateIDs(relevant), reason, PermissionReport{})
	}

	selection := SelectionPrecedence
	if request.ExplicitBinding != "" {
		selection = SelectionExplicit
		if len(compatible) > 1 {
			return base, blocked(request.SemanticID, ErrorAmbiguousRequired, bindingIDs(compatible), "explicit binding ID is duplicated", PermissionReport{})
		}
	} else {
		slices.SortFunc(compatible, compareBinding)
		if len(compatible) > 1 && compatible[0].Precedence == compatible[1].Precedence {
			winning := make([]tools.Binding, 0, len(compatible))
			for _, binding := range compatible {
				if binding.Precedence != compatible[0].Precedence {
					break
				}
				winning = append(winning, binding)
			}
			return base, blocked(request.SemanticID, ErrorAmbiguousRequired, bindingIDs(winning), fmt.Sprintf("%d is not a unique winning precedence", compatible[0].Precedence), PermissionReport{})
		}
	}

	selected := compatible[0]
	permission := comparePermissions(request.CanonicalPermission, mergePermissions(request.RenderedPermission, selected.Permission))
	result := Result{
		SemanticID:    request.SemanticID,
		Status:        StatusSelected,
		Selection:     selection,
		Binding:       selected,
		Input:         selected.Input,
		Output:        selected.Output,
		ErrorMappings: slices.Clone(selected.Errors),
		Evidence:      slices.Clone(selected.Evidence),
		Permission:    permission,
	}
	if len(permission.AddedEffects) > 0 || len(permission.AddedResources) > 0 {
		return result, blocked(request.SemanticID, ErrorPermissionWidening, []ir.SemanticID{selected.ID}, "rendered or provider binding scope exceeds canonical permission", permission)
	}
	return result, nil
}

func validateRequest(request Request) error {
	if err := ir.ValidateSemanticID(request.SemanticID); err != nil {
		return fmt.Errorf("semantic ID: %w", err)
	}
	if request.ExplicitBinding != "" {
		if err := ir.ValidateSemanticID(request.ExplicitBinding); err != nil {
			return fmt.Errorf("explicit binding: %w", err)
		}
	}
	if request.ProviderVersion.Major == 0 {
		return fmt.Errorf("provider version is required")
	}
	if err := validateSchemaRequirement("input", request.Input); err != nil {
		return err
	}
	if err := validateSchemaRequirement("output", request.Output); err != nil {
		return err
	}
	if len(request.CanonicalPermission.Effects) == 0 || len(request.CanonicalPermission.Resources) == 0 {
		return fmt.Errorf("canonical permission effects and resources are required")
	}
	return nil
}

func validateSchemaRequirement(name string, requirement SchemaRequirement) error {
	if requirement.Kind != tools.ContractSchema && requirement.Kind != tools.ContractDocumented {
		return fmt.Errorf("%s contract kind %q is invalid", name, requirement.Kind)
	}
	if strings.TrimSpace(requirement.Ref) == "" || !validRange(requirement.Versions) {
		return fmt.Errorf("%s contract reference and compatible version interval are required", name)
	}
	return nil
}

func compatibilityError(request Request, binding tools.Binding) string {
	if err := binding.Validate(); err != nil {
		return err.Error()
	}
	if err := tools.ValidateProviderSurface(binding); err != nil {
		return err.Error()
	}
	if !contains(binding.Versions, request.ProviderVersion) {
		return fmt.Sprintf("provider version %s is outside binding interval %s", request.ProviderVersion, binding.Versions)
	}
	if reason := contractCompatibilityError("input", request.Input, binding.Input); reason != "" {
		return reason
	}
	if reason := contractCompatibilityError("output", request.Output, binding.Output); reason != "" {
		return reason
	}
	seenProviderCodes := make(map[string]struct{}, len(binding.Errors))
	for _, mapping := range binding.Errors {
		if _, duplicate := seenProviderCodes[mapping.ProviderCode]; duplicate {
			return fmt.Sprintf("provider error code %q has multiple semantic mappings", mapping.ProviderCode)
		}
		seenProviderCodes[mapping.ProviderCode] = struct{}{}
	}
	return ""
}

func contractCompatibilityError(name string, required SchemaRequirement, provided tools.ContractRef) string {
	if provided.Kind != required.Kind || provided.Ref != required.Ref {
		return fmt.Sprintf("%s contract %q (%s) does not match required %q (%s)", name, provided.Ref, provided.Kind, required.Ref, required.Kind)
	}
	if !contains(required.Versions, provided.Version) {
		return fmt.Sprintf("%s contract version %s is outside required interval %s", name, provided.Version, required.Versions)
	}
	return ""
}

func compareBinding(left, right tools.Binding) int {
	if left.Precedence < right.Precedence {
		return -1
	}
	if left.Precedence > right.Precedence {
		return 1
	}
	return strings.Compare(string(left.ID), string(right.ID))
}

func comparePermissions(canonical, rendered tools.PermissionScope) PermissionReport {
	canonical = normalizePermission(canonical)
	rendered = normalizePermission(rendered)
	return PermissionReport{
		Canonical:      canonical,
		Rendered:       rendered,
		AddedEffects:   difference(rendered.Effects, canonical.Effects),
		AddedResources: difference(rendered.Resources, canonical.Resources),
	}
}

func mergePermissions(left, right tools.PermissionScope) tools.PermissionScope {
	return tools.PermissionScope{
		Effects:   append(slices.Clone(left.Effects), right.Effects...),
		Resources: append(slices.Clone(left.Resources), right.Resources...),
	}
}

func normalizePermission(permission tools.PermissionScope) tools.PermissionScope {
	return tools.PermissionScope{
		Effects:   sortedUnique(permission.Effects),
		Resources: sortedUnique(permission.Resources),
	}
}

func difference[T ~string](values, allowed []T) []T {
	allowedSet := make(map[T]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	result := make([]T, 0)
	for _, value := range values {
		if _, found := allowedSet[value]; !found {
			result = append(result, value)
		}
	}
	return result
}

func sortedUnique[T ~string](values []T) []T {
	result := slices.Clone(values)
	if result == nil {
		result = []T{}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func validRange(interval ir.VersionRange) bool {
	return interval.Minimum.Major > 0 && interval.Minimum.Major == interval.MaximumTested.Major && compareVersion(interval.Minimum, interval.MaximumTested) <= 0
}

func contains(interval ir.VersionRange, version ir.Version) bool {
	return validRange(interval) && version.Major == interval.Minimum.Major && compareVersion(version, interval.Minimum) >= 0 && compareVersion(version, interval.MaximumTested) <= 0
}

func compareVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}

func blocked(id ir.SemanticID, code ErrorCode, candidates []ir.SemanticID, reason string, permission PermissionReport) *BlockedError {
	return &BlockedError{Code: code, SemanticID: id, Candidates: sortedUnique(candidates), Reason: reason, Permission: permission, Blocking: true}
}

func bindingIDs(bindings []tools.Binding) []ir.SemanticID {
	ids := make([]ir.SemanticID, len(bindings))
	for index := range bindings {
		ids[index] = bindings[index].ID
	}
	return sortedUnique(ids)
}

func candidateIDs(candidates []candidate) []ir.SemanticID {
	ids := make([]ir.SemanticID, len(candidates))
	for index := range candidates {
		ids[index] = candidates[index].binding.ID
	}
	return sortedUnique(ids)
}

func incompatibleSummary(candidates []candidate) string {
	ordered := slices.Clone(candidates)
	slices.SortFunc(ordered, func(left, right candidate) int {
		return strings.Compare(string(left.binding.ID), string(right.binding.ID))
	})
	parts := make([]string, len(ordered))
	for index, item := range ordered {
		parts[index] = fmt.Sprintf("%s: %s", item.binding.ID, item.reason)
	}
	return strings.Join(parts, "; ")
}
