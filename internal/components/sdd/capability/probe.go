package capability

import (
	"context"
	"fmt"
	"reflect"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Prober is the volatile boundary for inspecting an installed runtime. It
// returns evidence and a proposed fact refinement; it owns no runtime state and
// has no authority to launch, schedule, or manage agents.
type Prober interface {
	Probe(context.Context, ProbeRequest) (ProbeResult, error)
}

// ProbeAuthority is the complete set of bounds a probe may refine. Anything
// not listed remains immutable, and Excluded is an unconditional deny.
type ProbeAuthority struct {
	CapabilityID      CapabilityID       `json:"capability_id"`
	RuntimeVersions   ir.VersionRange    `json:"runtime_versions"`
	Modes             []CapabilityValue  `json:"modes"`
	Cardinalities     []Cardinality      `json:"cardinalities"`
	Enforcement       []EnforcementClass `json:"enforcement"`
	TrustClasses      []ir.TrustClass    `json:"trust_classes,omitempty"`
	Permissions       []string           `json:"permissions,omitempty"`
	ExperimentalOptIn bool               `json:"experimental_opt_in"`
	Excluded          bool               `json:"excluded"`
}

type ProbeRequest struct {
	Base      CapabilityFact `json:"base"`
	Authority ProbeAuthority `json:"authority"`
}

type ProbeResult struct {
	Record       ProbeRecord     `json:"record"`
	Refined      CapabilityFact  `json:"refined"`
	TrustClasses []ir.TrustClass `json:"trust_classes,omitempty"`
	Permissions  []string        `json:"permissions,omitempty"`
}

type ProbeErrorCode string

const (
	ProbeErrorInvalidEvidence    ProbeErrorCode = "invalid_evidence"
	ProbeErrorExcluded           ProbeErrorCode = "excluded"
	ProbeErrorOutsideAuthority   ProbeErrorCode = "outside_authority"
	ProbeErrorPermissionWidening ProbeErrorCode = "permission_widening"
	ProbeErrorTrustWidening      ProbeErrorCode = "trust_widening"
	ProbeErrorExperimentalOptIn  ProbeErrorCode = "experimental_opt_in_required"
)

type ProbeValidationError struct {
	Code   ProbeErrorCode
	Field  string
	Reason string
}

func (e *ProbeValidationError) Error() string {
	return fmt.Sprintf("probe refinement rejected at %s: %s", e.Field, e.Reason)
}

// ApplyProbeResult validates a probe's proposed refinement against its exact
// authority and attaches the redacted evidence record to the accepted fact.
func ApplyProbeResult(request ProbeRequest, result ProbeResult) (CapabilityFact, error) {
	if err := ValidateProbeRefinement(request, result); err != nil {
		return CapabilityFact{}, err
	}
	refined := result.Refined
	refined.EvidenceClass = EvidenceExecutableProbe
	refined.EvidenceRef = result.Record.EvidenceDigest
	refined.ObservedAt = result.Record.Timestamp
	refined.Probe = &result.Record
	return refined, nil
}

func ValidateProbeRefinement(request ProbeRequest, result ProbeResult) error {
	if err := validateProbe(&result.Record); err != nil {
		return probeError(ProbeErrorInvalidEvidence, "record", "protocol or command, result, timestamp, and redacted digest are required")
	}
	if request.Authority.Excluded {
		return probeError(ProbeErrorExcluded, "authority.excluded", "hard exclusions cannot be overridden by probe evidence")
	}
	if request.Authority.CapabilityID != request.Base.ID || result.Refined.ID != request.Base.ID {
		return probeError(ProbeErrorOutsideAuthority, "refined.id", "probe capability identity is outside declared authority")
	}
	if result.Refined.Target != request.Base.Target || result.Refined.RuntimeID != request.Base.RuntimeID || result.Refined.AdapterID != request.Base.AdapterID {
		return probeError(ProbeErrorOutsideAuthority, "refined.identity", "target, runtime, and adapter identities are immutable")
	}
	if result.Refined.EvidenceClass != request.Base.EvidenceClass ||
		result.Refined.EvidenceRef != request.Base.EvidenceRef ||
		!result.Refined.ObservedAt.Equal(request.Base.ObservedAt) ||
		!result.Refined.FreshUntil.Equal(request.Base.FreshUntil) ||
		result.Refined.Confidence != request.Base.Confidence ||
		result.Refined.Current != request.Base.Current ||
		!reflect.DeepEqual(result.Refined.Probe, request.Base.Probe) {
		return probeError(ProbeErrorOutsideAuthority, "refined.metadata", "probe evidence metadata is recorded by the port and cannot be proposed as a refinement")
	}
	if !contains(request.Authority.Modes, result.Refined.Mode) {
		return probeError(ProbeErrorOutsideAuthority, "refined.mode", "capability mode is not declared by probe authority")
	}
	if !contains(request.Authority.Cardinalities, result.Refined.Cardinality) {
		return probeError(ProbeErrorOutsideAuthority, "refined.cardinality", "cardinality is not declared by probe authority")
	}
	if !contains(request.Authority.Enforcement, result.Refined.Enforcement) {
		return probeError(ProbeErrorOutsideAuthority, "refined.enforcement", "enforcement class is not declared by probe authority")
	}
	if !rangeContains(request.Authority.RuntimeVersions, result.Refined.RuntimeVersions) {
		return probeError(ProbeErrorOutsideAuthority, "refined.runtime_versions", "runtime interval exceeds probe authority")
	}
	if result.Refined.Experimental && !request.Authority.ExperimentalOptIn {
		return probeError(ProbeErrorExperimentalOptIn, "refined.experimental", "experimental native capability requires explicit opt-in")
	}
	if !subset(result.Permissions, request.Authority.Permissions) {
		return probeError(ProbeErrorPermissionWidening, "permissions", "probe result cannot widen permission scope")
	}
	if !subset(result.TrustClasses, request.Authority.TrustClasses) {
		return probeError(ProbeErrorTrustWidening, "trust_classes", "probe result cannot widen trusted inputs")
	}

	refined := result.Refined
	refined.Probe = &result.Record
	if err := validateFact(refined, 0, result.Record.Timestamp); err != nil {
		return probeError(ProbeErrorOutsideAuthority, "refined", err.Error())
	}
	return nil
}

func rangeContains(outer, inner ir.VersionRange) bool {
	return compareVersion(outer.Minimum, inner.Minimum) <= 0 && compareVersion(inner.MaximumTested, outer.MaximumTested) <= 0
}

func contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func subset[T comparable](values, allowed []T) bool {
	for _, value := range values {
		if !contains(allowed, value) {
			return false
		}
	}
	return true
}

func probeError(code ProbeErrorCode, field, reason string) *ProbeValidationError {
	return &ProbeValidationError{Code: code, Field: field, Reason: reason}
}
