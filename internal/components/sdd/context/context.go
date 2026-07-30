// Package context compiles trust-bounded prompt context without granting data
// layers authority over workflow controls.
package context

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Controls are the authority and safety decisions that only trusted policy may
// define. Other trust classes are data, even when their content looks like an
// instruction.
type Controls struct {
	Authority      []string `json:"authority,omitempty"`
	Permissions    []string `json:"permissions,omitempty"`
	Approvals      []string `json:"approvals,omitempty"`
	Destinations   []string `json:"destinations,omitempty"`
	StopConditions []string `json:"stop_conditions,omitempty"`
}

// SecretReference identifies where a runtime may obtain a secret. It never
// carries the secret value.
type SecretReference struct {
	ID       ir.SemanticID `json:"id"`
	Provider string        `json:"provider"`
}

// Section is one bounded root-context layer.
type Section struct {
	ID         ir.SemanticID    `json:"id"`
	Class      ir.TrustClass    `json:"class"`
	Content    string           `json:"content,omitempty"`
	Controls   Controls         `json:"controls,omitempty"`
	References []ir.SemanticID  `json:"references,omitempty"`
	Secret     *SecretReference `json:"secret,omitempty"`
	Mandatory  bool             `json:"mandatory,omitempty"`
}

// Reference is a detailed procedure loaded through progressive disclosure.
type Reference struct {
	ID         ir.SemanticID   `json:"id"`
	Content    string          `json:"content"`
	References []ir.SemanticID `json:"references,omitempty"`
}

type OverflowAction string

const (
	OverflowFail    OverflowAction = "fail"
	OverflowDegrade OverflowAction = "degrade"
)

// Limits are verified target instruction bounds expressed in bytes.
type Limits struct {
	MaxBytes        int            `json:"max_bytes"`
	MaxSectionBytes int            `json:"max_section_bytes"`
	OnOverflow      OverflowAction `json:"on_overflow"`
}

type Input struct {
	Sections           []Section       `json:"sections"`
	References         []Reference     `json:"references,omitempty"`
	RelevantReferences []ir.SemanticID `json:"relevant_references,omitempty"`
	Limits             Limits          `json:"limits"`
}

type Degradation struct {
	SectionID ir.SemanticID `json:"section_id"`
	Reason    string        `json:"reason"`
}

type Result struct {
	Sections     []Section     `json:"sections"`
	References   []Reference   `json:"references"`
	Controls     Controls      `json:"controls"`
	Prompt       []byte        `json:"prompt"`
	Bytes        int           `json:"bytes"`
	Degradations []Degradation `json:"degradations"`
}

type ErrorCode string

const (
	ErrorInvalidInput        ErrorCode = "invalid_input"
	ErrorAuthorityBoundary   ErrorCode = "authority_boundary"
	ErrorUnresolvedReference ErrorCode = "unresolved_reference"
	ErrorCyclicReference     ErrorCode = "cyclic_reference"
	ErrorSizeLimit           ErrorCode = "size_limit"
	ErrorOpaqueSecret        ErrorCode = "opaque_secret"
)

type ValidationError struct {
	Code        ErrorCode
	Path        string
	Observed    string
	Remediation string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("context validation failed at %s: %s; %s", e.Path, e.Observed, e.Remediation)
}

// Assemble validates trust and reference boundaries, applies fixed precedence,
// and fits optional data into the verified target limit. Mandatory policy and
// schema layers are never silently removed.
func Assemble(input Input) (Result, error) {
	if err := validateLimits(input.Limits); err != nil {
		return Result{}, err
	}

	sections, controls, degradations, err := validateSections(input.Sections, input.Limits)
	if err != nil {
		return Result{}, err
	}
	references, err := validateReferences(input.Sections, input.References, input.RelevantReferences, input.Limits)
	if err != nil {
		return Result{}, err
	}

	result := Result{Sections: sections, References: references, Controls: controls, Degradations: degradations}
	compilePrompt(&result)
	if result.Bytes <= input.Limits.MaxBytes {
		return result, nil
	}
	if input.Limits.OnOverflow != OverflowDegrade {
		return Result{}, sizeError("$.limits.max_bytes", result.Bytes, input.Limits.MaxBytes)
	}

	for index := len(result.Sections) - 1; index >= 0 && result.Bytes > input.Limits.MaxBytes; index-- {
		section := result.Sections[index]
		if mandatory(section) {
			continue
		}
		result.Sections = slices.Delete(result.Sections, index, index+1)
		result.Degradations = append(result.Degradations, Degradation{
			SectionID: section.ID,
			Reason:    fmt.Sprintf("optional context omitted to fit verified %d-byte instruction limit", input.Limits.MaxBytes),
		})
		compilePrompt(&result)
	}
	if result.Bytes > input.Limits.MaxBytes {
		return Result{}, sizeError("$.limits.max_bytes", result.Bytes, input.Limits.MaxBytes)
	}
	return result, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxBytes <= 0 || limits.MaxSectionBytes <= 0 {
		return invalid("$.limits", "invalid byte limits", "set positive section and total byte limits")
	}
	if limits.OnOverflow != "" && limits.OnOverflow != OverflowFail && limits.OnOverflow != OverflowDegrade {
		return invalid("$.limits.on_overflow", string(limits.OnOverflow), "use fail or degrade")
	}
	return nil
}

func validateSections(input []Section, limits Limits) ([]Section, Controls, []Degradation, error) {
	sections := cloneSections(input)
	seen := make(map[ir.SemanticID]struct{}, len(sections))
	controls := Controls{}
	degradations := make([]Degradation, 0)
	retained := sections[:0]
	for index := range sections {
		section := &sections[index]
		path := fmt.Sprintf("$.sections[%d]", index)
		if err := addID(seen, section.ID, path+".id"); err != nil {
			return nil, Controls{}, nil, err
		}
		if _, ok := trustPrecedence(section.Class); !ok {
			return nil, Controls{}, nil, invalid(path+".class", string(section.Class), "use one of the eight canonical trust classes")
		}
		section.References = sortedUnique(section.References)
		section.Controls = normalizeControls(section.Controls)
		if section.Class != ir.TrustTrustedPolicy && !emptyControls(section.Controls) {
			return nil, Controls{}, nil, &ValidationError{Code: ErrorAuthorityBoundary, Path: path + ".controls", Observed: string(section.Class) + " attempted to define workflow controls", Remediation: "move authority, permissions, approvals, destinations, and stop conditions to trusted_policy"}
		}
		if section.Class == ir.TrustTrustedPolicy {
			controls = mergeControls(controls, section.Controls)
		}
		if err := validateSecret(*section, path); err != nil {
			return nil, Controls{}, nil, err
		}
		if len([]byte(section.Content)) > limits.MaxSectionBytes {
			if mandatory(*section) || limits.OnOverflow != OverflowDegrade {
				return nil, Controls{}, nil, sizeError(path+".content", len([]byte(section.Content)), limits.MaxSectionBytes)
			}
			degradations = append(degradations, Degradation{SectionID: section.ID, Reason: fmt.Sprintf("optional context omitted to fit verified %d-byte section limit", limits.MaxSectionBytes)})
			continue
		}
		retained = append(retained, *section)
	}
	sections = retained
	slices.SortStableFunc(sections, func(left, right Section) int {
		leftRank, _ := trustPrecedence(left.Class)
		rightRank, _ := trustPrecedence(right.Class)
		if leftRank != rightRank {
			return leftRank - rightRank
		}
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return sections, normalizeControls(controls), degradations, nil
}

func validateSecret(section Section, path string) error {
	if section.Class != ir.TrustSecretReference {
		if section.Secret != nil {
			return invalid(path+".secret", string(section.Secret.ID), "use secret_reference trust class for opaque secret references")
		}
		return nil
	}
	if section.Secret == nil || section.Secret.ID == "" || strings.TrimSpace(section.Secret.Provider) == "" {
		return opaqueSecret(path, "missing opaque ID or provider")
	}
	if err := ir.ValidateSemanticID(section.Secret.ID); err != nil {
		return opaqueSecret(path+".secret.id", err.Error())
	}
	if section.Content != "" || containsSecretMaterial(section.Secret.Provider) {
		return opaqueSecret(path, "secret value or credential-bearing provider")
	}
	return nil
}

func validateReferences(sections []Section, input []Reference, relevant []ir.SemanticID, limits Limits) ([]Reference, error) {
	references := cloneReferences(input)
	byID := make(map[ir.SemanticID]Reference, len(references))
	for index := range references {
		reference := &references[index]
		path := fmt.Sprintf("$.references[%d]", index)
		if err := addIDReference(byID, *reference, path+".id"); err != nil {
			return nil, err
		}
		reference.References = sortedUnique(reference.References)
		if len([]byte(reference.Content)) > limits.MaxSectionBytes {
			return nil, sizeError(path+".content", len([]byte(reference.Content)), limits.MaxSectionBytes)
		}
	}
	for index, section := range sections {
		for refIndex, id := range section.References {
			if _, ok := byID[id]; !ok {
				return nil, unresolved(fmt.Sprintf("$.sections[%d].references[%d]", index, refIndex), id)
			}
		}
	}
	for index, reference := range references {
		for refIndex, id := range reference.References {
			if _, ok := byID[id]; !ok {
				return nil, unresolved(fmt.Sprintf("$.references[%d].references[%d]", index, refIndex), id)
			}
		}
	}
	if hasReferenceCycle(byID) {
		return nil, &ValidationError{Code: ErrorCyclicReference, Path: "$.references", Observed: "reference cycle", Remediation: "remove at least one reference edge from the cycle"}
	}

	selected := make(map[ir.SemanticID]struct{})
	var selectReference func(ir.SemanticID)
	selectReference = func(id ir.SemanticID) {
		if _, exists := selected[id]; exists {
			return
		}
		selected[id] = struct{}{}
		for _, child := range byID[id].References {
			selectReference(child)
		}
	}
	for index, id := range sortedUnique(relevant) {
		if _, ok := byID[id]; !ok {
			return nil, unresolved(fmt.Sprintf("$.relevant_references[%d]", index), id)
		}
		selectReference(id)
	}
	result := make([]Reference, 0, len(selected))
	for id := range selected {
		result = append(result, byID[id])
	}
	slices.SortFunc(result, func(left, right Reference) int { return strings.Compare(string(left.ID), string(right.ID)) })
	return result, nil
}

func hasReferenceCycle(references map[ir.SemanticID]Reference) bool {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[ir.SemanticID]int, len(references))
	var visit func(ir.SemanticID) bool
	visit = func(id ir.SemanticID) bool {
		if state[id] == visiting {
			return true
		}
		if state[id] == visited {
			return false
		}
		state[id] = visiting
		for _, child := range references[id].References {
			if visit(child) {
				return true
			}
		}
		state[id] = visited
		return false
	}
	ids := make([]ir.SemanticID, 0, len(references))
	for id := range references {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if visit(id) {
			return true
		}
	}
	return false
}

func trustPrecedence(class ir.TrustClass) (int, bool) {
	switch class {
	case ir.TrustTrustedPolicy:
		return 0, true
	case ir.TrustOperatorInput:
		return 1, true
	case ir.TrustTrustedSchema:
		return 2, true
	case ir.TrustRepositoryData:
		return 3, true
	case ir.TrustToolOutput:
		return 4, true
	case ir.TrustPeerMessage:
		return 5, true
	case ir.TrustRemoteUntrusted:
		return 6, true
	case ir.TrustSecretReference:
		return 7, true
	default:
		return 0, false
	}
}

func mandatory(section Section) bool {
	return section.Mandatory || section.Class == ir.TrustTrustedPolicy || section.Class == ir.TrustTrustedSchema
}

func compilePrompt(result *Result) {
	prompt, err := json.Marshal(struct {
		SchemaVersion string      `json:"schema_version"`
		Controls      Controls    `json:"controls"`
		Sections      []Section   `json:"sections"`
		References    []Reference `json:"references"`
	}{
		SchemaVersion: "1.0.0",
		Controls:      result.Controls,
		Sections:      result.Sections,
		References:    result.References,
	})
	if err != nil {
		panic(fmt.Sprintf("marshal validated prompt context: %v", err))
	}
	result.Prompt = prompt
	result.Bytes = len(prompt)
}

func cloneSections(input []Section) []Section {
	result := slices.Clone(input)
	for index := range result {
		result[index].References = slices.Clone(input[index].References)
		result[index].Controls = cloneControls(input[index].Controls)
		if input[index].Secret != nil {
			secret := *input[index].Secret
			result[index].Secret = &secret
		}
	}
	return result
}

func cloneReferences(input []Reference) []Reference {
	result := slices.Clone(input)
	for index := range result {
		result[index].References = slices.Clone(input[index].References)
	}
	return result
}

func cloneControls(input Controls) Controls {
	return Controls{
		Authority: slices.Clone(input.Authority), Permissions: slices.Clone(input.Permissions),
		Approvals: slices.Clone(input.Approvals), Destinations: slices.Clone(input.Destinations),
		StopConditions: slices.Clone(input.StopConditions),
	}
}

func normalizeControls(input Controls) Controls {
	return Controls{
		Authority: sortedUnique(input.Authority), Permissions: sortedUnique(input.Permissions),
		Approvals: sortedUnique(input.Approvals), Destinations: sortedUnique(input.Destinations),
		StopConditions: sortedUnique(input.StopConditions),
	}
}

func mergeControls(left, right Controls) Controls {
	return Controls{
		Authority: append(left.Authority, right.Authority...), Permissions: append(left.Permissions, right.Permissions...),
		Approvals: append(left.Approvals, right.Approvals...), Destinations: append(left.Destinations, right.Destinations...),
		StopConditions: append(left.StopConditions, right.StopConditions...),
	}
}

func emptyControls(controls Controls) bool {
	return len(controls.Authority) == 0 && len(controls.Permissions) == 0 && len(controls.Approvals) == 0 && len(controls.Destinations) == 0 && len(controls.StopConditions) == 0
}

func addID(seen map[ir.SemanticID]struct{}, id ir.SemanticID, path string) error {
	if err := ir.ValidateSemanticID(id); err != nil {
		return invalid(path, string(id), "use a canonical lower-case namespaced semantic ID")
	}
	if _, exists := seen[id]; exists {
		return invalid(path, string(id), "use each context section ID exactly once")
	}
	seen[id] = struct{}{}
	return nil
}

func addIDReference(seen map[ir.SemanticID]Reference, reference Reference, path string) error {
	if err := ir.ValidateSemanticID(reference.ID); err != nil {
		return invalid(path, string(reference.ID), "use a canonical lower-case namespaced semantic ID")
	}
	if _, exists := seen[reference.ID]; exists {
		return invalid(path, string(reference.ID), "use each progressive reference ID exactly once")
	}
	seen[reference.ID] = reference
	return nil
}

func containsSecretMaterial(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"token=", "password=", "secret=", "authorization:", "begin private key", "sk-"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.User != nil
}

func sortedUnique[T ~string](values []T) []T {
	result := slices.Clone(values)
	if result == nil {
		result = []T{}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func invalid(path, observed, remediation string) *ValidationError {
	return &ValidationError{Code: ErrorInvalidInput, Path: path, Observed: observed, Remediation: remediation}
}

func unresolved(path string, id ir.SemanticID) *ValidationError {
	return &ValidationError{Code: ErrorUnresolvedReference, Path: path, Observed: string(id), Remediation: "declare the detailed reference before context compilation"}
}

func opaqueSecret(path, observed string) *ValidationError {
	return &ValidationError{Code: ErrorOpaqueSecret, Path: path, Observed: observed, Remediation: "provide only a semantic secret ID and non-credential-bearing provider reference"}
}

func sizeError(path string, observed, limit int) *ValidationError {
	return &ValidationError{Code: ErrorSizeLimit, Path: path, Observed: fmt.Sprintf("%d bytes exceeds %d", observed, limit), Remediation: "increase the verified target limit or declare optional context degradation"}
}
