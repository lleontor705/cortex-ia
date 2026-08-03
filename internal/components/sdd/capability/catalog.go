// Package capability defines the evidence-backed catalog used to qualify
// runtime capabilities. It validates catalog evidence but does not probe or
// execute external runtimes.
package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type CapabilityID = ir.SemanticID
type TargetID string
type Confidence float64

type CapabilityValue string

const (
	CapabilityAbsent    CapabilityValue = "absent"
	CapabilityAvailable CapabilityValue = "available"
)

type Cardinality string

const (
	CardinalityNone Cardinality = "none"
	CardinalityOne  Cardinality = "one"
	CardinalityMany Cardinality = "many"
)

type EvidenceClass string

const (
	EvidenceDocumentation   EvidenceClass = "documentation"
	EvidenceInstalledSchema EvidenceClass = "installed-schema"
	EvidenceExecutableProbe EvidenceClass = "executable-probe"
	EvidenceRuntimeObserved EvidenceClass = "runtime-observed"
)

type EnforcementClass string

const (
	EnforcementRuntime EnforcementClass = "runtime"
	EnforcementHook    EnforcementClass = "hook"
	EnforcementMCP     EnforcementClass = "mcp"
	EnforcementPrompt  EnforcementClass = "prompt"
	EnforcementNone    EnforcementClass = "none"
)

type ProbeMethod string

const (
	ProbeCommand  ProbeMethod = "command"
	ProbeProtocol ProbeMethod = "protocol"
)

// ProbeRecord is redacted qualification evidence. EvidenceDigest must identify
// the retained redacted evidence, never a secret-bearing raw transcript.
type ProbeRecord struct {
	ID             ir.SemanticID `json:"id"`
	Method         ProbeMethod   `json:"method"`
	Command        string        `json:"command,omitempty"`
	Protocol       string        `json:"protocol,omitempty"`
	Result         string        `json:"result"`
	Timestamp      time.Time     `json:"timestamp"`
	EvidenceDigest string        `json:"evidence_digest"`
}

// CapabilityFact describes one capability over one inclusive runtime version
// interval.
type CapabilityFact struct {
	ID              CapabilityID     `json:"id"`
	Mode            CapabilityValue  `json:"mode"`
	Cardinality     Cardinality      `json:"cardinality"`
	Target          TargetID         `json:"target"`
	RuntimeID       string           `json:"runtime_id"`
	AdapterID       string           `json:"adapter_id"`
	RuntimeVersions ir.VersionRange  `json:"runtime_versions"`
	EvidenceClass   EvidenceClass    `json:"evidence_class,omitempty"`
	EvidenceRef     string           `json:"evidence_ref,omitempty"`
	ObservedAt      time.Time        `json:"observed_at,omitempty"`
	FreshUntil      time.Time        `json:"fresh_until,omitempty"`
	Confidence      Confidence       `json:"confidence,omitempty"`
	Experimental    bool             `json:"experimental"`
	Current         bool             `json:"current"`
	Probe           *ProbeRecord     `json:"probe,omitempty"`
	Enforcement     EnforcementClass `json:"enforcement"`
}

// Fact is retained as a concise source-compatible name for catalog fixtures.
type Fact = CapabilityFact

// Catalog is a versioned snapshot. Validation is deterministic for the caller-
// supplied time, so builds never depend on the process clock implicitly.
type Catalog struct {
	SchemaVersion ir.Version       `json:"schema_version"`
	Version       ir.Version       `json:"version"`
	Facts         []CapabilityFact `json:"facts"`
}

var CatalogSchema = ir.SchemaContract{
	ID:      "schema/capability-catalog",
	Current: ir.MustParseVersion("1.0.0"),
	Supported: ir.VersionRange{
		Minimum:       ir.MustParseVersion("1.0.0"),
		MaximumTested: ir.MustParseVersion("1.1.0"),
	},
}

type ErrorCode string

const (
	ErrorMissingRequired      ErrorCode = "missing_required"
	ErrorInvalidValue         ErrorCode = "invalid_value"
	ErrorUnsupportedVersion   ErrorCode = "unsupported_version"
	ErrorUnsupportedExtension ErrorCode = "unsupported_extension"
	ErrorUnknownField         ErrorCode = "unknown_field"
	ErrorContradictoryOverlap ErrorCode = "contradictory_overlap"
	ErrorExpiredCurrent       ErrorCode = "expired_current"
)

// ValidationError provides a stable field path and an actionable repair.
type ValidationError struct {
	Code        ErrorCode
	Path        string
	Observed    string
	Supported   string
	Remediation string
	Cause       error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s at %s: observed %s; supported %s; %s", CatalogSchema.ID, e.Path, e.Observed, e.Supported, e.Remediation)
}

func (e *ValidationError) Unwrap() error { return e.Cause }

type Degradation = ir.Degradation

type DecodeResult struct {
	Catalog      Catalog
	Degradations []Degradation
}

type extensionDeclaration struct {
	Optional bool            `json:"optional"`
	Value    json.RawMessage `json:"value,omitempty"`
}

var catalogFields = map[string]struct{}{
	"schema_version": {}, "version": {}, "facts": {},
}

// DecodeCatalog applies schema compatibility and catalog validation before the
// snapshot can participate in profile selection.
func DecodeCatalog(data []byte, now time.Time) (DecodeResult, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil {
		return DecodeResult{}, validationError(ErrorInvalidValue, "$", string(data), "valid JSON object", "provide a valid capability catalog object", err)
	}
	for _, required := range []string{"schema_version", "version", "facts"} {
		if raw, ok := fields[required]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return DecodeResult{}, validationError(ErrorMissingRequired, "$."+required, "<missing>", CatalogSchema.Supported.String(), "add the required field", nil)
		}
	}

	var schemaVersion ir.Version
	if err := json.Unmarshal(fields["schema_version"], &schemaVersion); err != nil {
		return DecodeResult{}, validationError(ErrorInvalidValue, "$.schema_version", string(fields["schema_version"]), CatalogSchema.Supported.String(), "use a major.minor.patch schema version", err)
	}
	if schemaVersion.Major != CatalogSchema.Supported.Minimum.Major {
		return DecodeResult{}, validationError(ErrorUnsupportedVersion, "$.schema_version", schemaVersion.String(), CatalogSchema.Supported.String(), "regenerate the catalog with a supported schema major", nil)
	}

	unknown := make([]string, 0)
	for name := range fields {
		if _, ok := catalogFields[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)

	result := DecodeResult{}
	for _, name := range unknown {
		if !strings.Contains(name, "/") {
			return DecodeResult{}, validationError(ErrorUnknownField, "$."+name, name, CatalogSchema.Supported.String(), "remove the field or use a declared namespaced extension", nil)
		}
		var extension extensionDeclaration
		if err := json.Unmarshal(fields[name], &extension); err != nil {
			return DecodeResult{}, validationError(ErrorInvalidValue, "$."+name, string(fields[name]), "optional namespaced extension", "declare the extension as an object with optional=true", err)
		}
		if !extension.Optional {
			return DecodeResult{}, validationError(ErrorUnsupportedExtension, "$."+name, name, "optional namespaced extension", "remove the extension or mark it optional after confirming degradation is safe", nil)
		}
		result.Degradations = append(result.Degradations, Degradation{
			SemanticID: ir.SemanticID(name),
			Reason:     "optional namespaced extension is unsupported and was ignored",
		})
		delete(fields, name)
	}

	canonical, err := json.Marshal(fields)
	if err != nil {
		return DecodeResult{}, err
	}
	if err := json.Unmarshal(canonical, &result.Catalog); err != nil {
		return DecodeResult{}, validationError(ErrorInvalidValue, "$", "invalid catalog value", CatalogSchema.Supported.String(), "correct the field type reported by the decoder", err)
	}
	if err := result.Catalog.Validate(now); err != nil {
		return DecodeResult{}, err
	}
	return result, nil
}

// Validate rejects facts that cannot support a conservative, evidence-backed
// capability decision.
func (c Catalog) Validate(now time.Time) error {
	if c.SchemaVersion.Major != CatalogSchema.Supported.Minimum.Major {
		return validationError(ErrorUnsupportedVersion, "$.schema_version", c.SchemaVersion.String(), CatalogSchema.Supported.String(), "use a supported catalog schema major", nil)
	}
	if c.Version.Major == 0 {
		return validationError(ErrorInvalidValue, "$.version", c.Version.String(), "non-zero semantic version", "set the catalog content version", nil)
	}
	for index := range c.Facts {
		if err := validateFact(c.Facts[index], index, now); err != nil {
			return err
		}
		for previous := 0; previous < index; previous++ {
			if contradictory(c.Facts[previous], c.Facts[index]) {
				return validationError(ErrorContradictoryOverlap, factPath(index, "runtime_versions"), c.Facts[index].RuntimeVersions.String(), "non-overlapping intervals or identical capability claims", "split the version intervals at an exact boundary or reconcile the contradictory facts", nil)
			}
		}
	}
	return nil
}

func validateFact(fact CapabilityFact, index int, now time.Time) error {
	path := func(field string) string { return factPath(index, field) }
	if err := ir.ValidateSemanticID(fact.ID); err != nil {
		return validationError(ErrorInvalidValue, path("id"), string(fact.ID), "canonical semantic capability ID", "use a lower-case namespaced semantic ID", err)
	}
	if strings.TrimSpace(string(fact.Target)) == "" || strings.TrimSpace(fact.RuntimeID) == "" || strings.TrimSpace(fact.AdapterID) == "" {
		return validationError(ErrorMissingRequired, path("target"), "incomplete identity", "target, runtime_id, and adapter_id", "identify both the external runtime and cortex-ia adapter", nil)
	}
	if !validInterval(fact.RuntimeVersions) {
		return validationError(ErrorInvalidValue, path("runtime_versions"), fact.RuntimeVersions.String(), "ordered versions in one supported major", "set minimum and maximum_tested to compatible ordered semantic versions", nil)
	}
	if !validMode(fact.Mode) {
		return validationError(ErrorInvalidValue, path("mode"), string(fact.Mode), "absent|available", "select a declared capability mode", nil)
	}
	if !validCardinality(fact.Cardinality) || (fact.Mode == CapabilityAbsent) != (fact.Cardinality == CardinalityNone) {
		return validationError(ErrorInvalidValue, path("cardinality"), string(fact.Cardinality), "absent=none; available=one|many", "choose a cardinality consistent with the capability mode", nil)
	}
	if !validEnforcement(fact.Enforcement) {
		return validationError(ErrorInvalidValue, path("enforcement"), string(fact.Enforcement), "runtime|hook|mcp|prompt|none", "select a declared enforcement class", nil)
	}
	if fact.Mode == CapabilityAbsent {
		return nil
	}
	if !validEvidenceClass(fact.EvidenceClass) {
		return validationError(ErrorInvalidValue, path("evidence_class"), string(fact.EvidenceClass), "declared evidence class", "classify the evidence source accurately", nil)
	}
	if strings.TrimSpace(fact.EvidenceRef) == "" {
		return validationError(ErrorMissingRequired, path("evidence_ref"), "<missing>", "durable provenance reference", "add the evidence artifact or documentation reference", nil)
	}
	if fact.ObservedAt.IsZero() || fact.FreshUntil.IsZero() || fact.FreshUntil.Before(fact.ObservedAt) {
		return validationError(ErrorInvalidValue, path("fresh_until"), fact.FreshUntil.String(), "deadline on or after observed_at", "record observation and freshness timestamps from the evidence", nil)
	}
	if fact.Current && !fact.FreshUntil.After(now) {
		return validationError(ErrorExpiredCurrent, path("fresh_until"), fact.FreshUntil.Format(time.RFC3339), "deadline after validation time", "refresh the evidence or mark the fact non-current", nil)
	}
	if fact.Confidence <= 0 || fact.Confidence > 1 {
		return validationError(ErrorInvalidValue, path("confidence"), fmt.Sprint(fact.Confidence), "confidence in (0,1]", "record an evidence-backed confidence value", nil)
	}
	if fact.EvidenceClass == EvidenceDocumentation && fact.Enforcement != EnforcementPrompt && fact.Enforcement != EnforcementNone {
		return validationError(ErrorInvalidValue, path("evidence_class"), string(fact.EvidenceClass), "documentation cannot prove runtime enforcement", "use runtime-observed evidence or downgrade the enforcement class", nil)
	}
	if fact.EvidenceClass == EvidenceInstalledSchema || fact.EvidenceClass == EvidenceExecutableProbe {
		if err := validateProbe(fact.Probe); err != nil {
			return validationError(ErrorMissingRequired, path("probe"), "incomplete probe record", "command/protocol, result, timestamp, and redacted evidence digest", "attach the complete redacted probe record", err)
		}
	}
	return nil
}

func validateProbe(probe *ProbeRecord) error {
	if probe == nil {
		return fmt.Errorf("probe record is nil")
	}
	if err := ir.ValidateSemanticID(probe.ID); err != nil {
		return err
	}
	methodValid := probe.Method == ProbeCommand && strings.TrimSpace(probe.Command) != "" && strings.TrimSpace(probe.Protocol) == "" ||
		probe.Method == ProbeProtocol && strings.TrimSpace(probe.Protocol) != "" && strings.TrimSpace(probe.Command) == ""
	if !methodValid || strings.TrimSpace(probe.Result) == "" || probe.Timestamp.IsZero() || strings.TrimSpace(probe.EvidenceDigest) == "" {
		return fmt.Errorf("incomplete probe record")
	}
	return nil
}

func contradictory(left, right CapabilityFact) bool {
	if left.ID != right.ID || left.Target != right.Target || left.RuntimeID != right.RuntimeID || left.AdapterID != right.AdapterID {
		return false
	}
	if !intervalsOverlap(left.RuntimeVersions, right.RuntimeVersions) {
		return false
	}
	return left.Mode != right.Mode || left.Cardinality != right.Cardinality || left.Enforcement != right.Enforcement || left.Experimental != right.Experimental
}

func intervalsOverlap(left, right ir.VersionRange) bool {
	return compareVersion(left.Minimum, right.MaximumTested) <= 0 && compareVersion(right.Minimum, left.MaximumTested) <= 0
}

func validInterval(interval ir.VersionRange) bool {
	isDefault := compareVersion(interval.Minimum, ir.Version{}) == 0 && compareVersion(interval.MaximumTested, ir.Version{}) == 0
	return !isDefault && interval.Minimum.Major == interval.MaximumTested.Major && compareVersion(interval.Minimum, interval.MaximumTested) <= 0
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

func validMode(mode CapabilityValue) bool {
	return mode == CapabilityAbsent || mode == CapabilityAvailable
}

func validCardinality(cardinality Cardinality) bool {
	return cardinality == CardinalityNone || cardinality == CardinalityOne || cardinality == CardinalityMany
}

func validEvidenceClass(class EvidenceClass) bool {
	return class == EvidenceDocumentation || class == EvidenceInstalledSchema || class == EvidenceExecutableProbe || class == EvidenceRuntimeObserved
}

func validEnforcement(class EnforcementClass) bool {
	return class == EnforcementRuntime || class == EnforcementHook || class == EnforcementMCP || class == EnforcementPrompt || class == EnforcementNone
}

func factPath(index int, field string) string {
	return fmt.Sprintf("$.facts[%d].%s", index, field)
}

func validationError(code ErrorCode, path, observed, supported, remediation string, cause error) *ValidationError {
	return &ValidationError{Code: code, Path: path, Observed: observed, Supported: supported, Remediation: remediation, Cause: cause}
}
