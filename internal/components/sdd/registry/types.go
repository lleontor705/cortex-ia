// Package registry is the declarative skill overlay ingress boundary. It
// verifies local provenance of configured custom skills, normalizes
// declarations, and merges them additively onto the embedded asset catalog.
// The package is the single policy owner for overlay disable policy and
// overlay diagnostics; config stays transport-only and adapters stay
// host-only, so neither re-implements anything declared here.
//
// This file declares the typed contracts only; loader, normalize, merge, and
// receipt behavior live in their own files.
package registry

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// DisableClass classifies whether a component may be disabled by user
// selection. Classification is positive and fail-closed: only components
// explicitly classified Optional may be disabled, and a component missing
// from the policy map is protected, never Optional.
type DisableClass uint8

const (
	// Optional marks an explicitly declared optional component; it is the
	// only class eligible for DisabledComponents selection.
	Optional DisableClass = iota
	// ProtectedAuthority marks authority components that must never be
	// disabled.
	ProtectedAuthority
	// ProtectedWorkflow marks the SDD workflow component that must never be
	// disabled.
	ProtectedWorkflow
	// ProtectedRequired marks transitive dependencies of retained
	// selections; they stay protected while anything requiring them is
	// retained.
	ProtectedRequired
)

// Policy is the declarative disable policy snapshot applied during resolve.
type Policy struct {
	// SchemaVersion identifies the policy schema.
	SchemaVersion string
	// PolicyVersion identifies the policy revision.
	PolicyVersion string
	// ComponentClasses maps every classified component to its disable
	// class. Lookups must fail closed: absence means protected.
	ComponentClasses map[model.ComponentID]DisableClass
}

// The declarative skill core contracts below are declared canonically in the
// skillcore leaf package and re-exported here as aliases and constants so
// existing registry consumers keep compiling unchanged (design D8,
// typed_contracts.skillcore). Never declare new skill-core members here.

// OriginKind distinguishes where a skill's bytes come from.
type OriginKind = skillcore.OriginKind

const (
	// OriginEmbedded marks a skill from the embedded canonical baseline.
	OriginEmbedded = skillcore.OriginEmbedded
	// OriginCustom marks a skill declared by the local overlay config.
	OriginCustom = skillcore.OriginCustom
)

// Skill is one verified, normalized skill record. It intentionally carries
// only identity and content: agents, tools, permissions, and bindings are
// never fields of a declarative skill and are rejected as unsupported.
type Skill = skillcore.Skill

// SkillSet is the deterministically ordered effective skill collection.
type SkillSet = skillcore.SkillSet

// EvidenceKind identifies which kind of declared source an Evidence record
// verifies.
type EvidenceKind uint8

const (
	// EvidenceConfigSource marks verification of the declaring config file.
	EvidenceConfigSource EvidenceKind = iota
	// EvidenceSkillSource marks verification of one custom skill source.
	EvidenceSkillSource
)

// Evidence records the outcome of verifying one declared local source. It is
// recorded fact, not authorization: metadata can never self-authorize.
type Evidence struct {
	// Kind identifies the verified source kind.
	Kind EvidenceKind
	// Verified reports whether canonical filesystem verification succeeded.
	Verified bool
	// ConfigRelativePath is the source location relative to the config file.
	ConfigRelativePath string
	// ContentSHA256 is the SHA-256 digest of the content read from the
	// verified handle.
	ContentSHA256 string
}

// Request is the registry ingress input. It carries transport-only intent
// from configuration; every referenced path is re-verified against the local
// filesystem before any of it is accepted.
type Request struct {
	// Selection is the registry intent loaded from config. It is populated
	// by config and is never an authority source.
	Selection model.RegistrySelection
	// RetainedComponents is the retained component selection handed over
	// explicitly by the install pipeline (design D4): the resolved selected
	// components with declared disables already removed. It is the only
	// source of receipt EffectiveComponents; Policy.ComponentClasses stays
	// an authorization input and never defines the effective selection.
	RetainedComponents []model.ComponentID
}

// Receipt is the versioned canonical evidence of one effective install
// input. It carries only stable semantic digests and relative outputs;
// timestamps, absolute paths, inode/mtime facts, backup IDs, changed flags,
// and execution order are excluded.
type Receipt struct {
	// SchemaVersion identifies the receipt schema.
	SchemaVersion string
	// PolicyDigest fingerprints the applied Policy.
	PolicyDigest string
	// BaselineDigest fingerprints the embedded baseline catalog.
	BaselineDigest string
	// EffectiveComponents lists the effective component IDs, sorted.
	EffectiveComponents []model.ComponentID
	// EffectiveSkills is the ordered effective skill set and its digest.
	EffectiveSkills SkillSet
	// HostOutputs lists planned host outputs as adapter-relative paths.
	HostOutputs []string
	// Fingerprint is the SHA-256 of the canonical JSON encoding of this
	// receipt, computed without the Fingerprint field itself.
	Fingerprint string
}

// Resolved is the complete registry output consumed by the compiler and
// composer. The catalog is the effective additive merge: the embedded
// catalog remains the base and custom skills are additions, never
// replacements.
type Resolved struct {
	// Catalog is the effective materialized catalog (embedded base plus
	// verified custom additions).
	Catalog assets.MaterializedCatalog
	// EffectiveSkills lists every effective skill in canonical order.
	EffectiveSkills []Skill
	// Disabled lists the optional components resolved as disabled.
	Disabled []model.ComponentID
	// Provenance records the verification evidence for every declared
	// local source.
	Provenance []Evidence
	// CanonicalReceipt is the canonical evidence of this effective input.
	CanonicalReceipt Receipt
}

// ErrorClass is the stable failure vocabulary of the registry and install
// pipeline. Classes are disjoint so a failure is never misreported and no
// success is claimed from a failure path.
type ErrorClass string

const (
	// ErrorUntrusted marks a source that failed local provenance
	// verification.
	ErrorUntrusted ErrorClass = "untrusted"
	// ErrorInvalid marks a syntactically or semantically invalid
	// declaration.
	ErrorInvalid ErrorClass = "invalid"
	// ErrorUnsupported marks an out-of-scope declaration such as agents,
	// tools, permissions, or bindings.
	ErrorUnsupported ErrorClass = "unsupported"
	// ErrorOverride marks an attempt to replace an embedded canonical skill
	// ID.
	ErrorOverride ErrorClass = "override"
	// ErrorCollision marks two effective custom declarations sharing one
	// ID, even with identical content.
	ErrorCollision ErrorClass = "collision"
	// ErrorProtectedDisable marks a disable attempt against a protected
	// component class.
	ErrorProtectedDisable ErrorClass = "protected_disable"
	// ErrorWrite marks a mutation failure after preflight.
	ErrorWrite ErrorClass = "write"
	// ErrorRollback marks an incomplete restoration after a failure.
	ErrorRollback ErrorClass = "rollback"
)

// Stage locates which pipeline boundary produced a diagnostic.
type Stage string

const (
	// StageLoad covers local provenance and containment verification.
	StageLoad Stage = "load"
	// StageNormalize covers ID, path, and content canonicalization.
	StageNormalize Stage = "normalize"
	// StageMerge covers additive merge, collision, and override checks.
	StageMerge Stage = "merge"
	// StagePlan covers the global preflight before any write.
	StagePlan Stage = "plan"
	// StageApply covers mutation writes.
	StageApply Stage = "apply"
	// StageVerify covers post-apply residual verification.
	StageVerify Stage = "verify"
	// StageRollback covers snapshot restoration.
	StageRollback Stage = "rollback"
)

// Diagnostic is one aggregated failure fact. Reports are ordered by Stage,
// Class, ID, Rule, then DeclarationIndex so repeated validation yields a
// deterministic primary cause.
type Diagnostic struct {
	// Class is the stable failure class.
	Class ErrorClass
	// Stage is the pipeline boundary that produced the diagnostic.
	Stage Stage
	// ID is the involved skill ID when one is parseable; nil otherwise. It
	// is never invented.
	ID *model.SkillID
	// Rule names the violated rule.
	Rule string
	// DeclarationIndex is the request-order index of the offending
	// declaration when one is identifiable.
	DeclarationIndex int
	// SafeRemediation describes a known-safe fix; it stays empty when none
	// is known instead of guessing.
	SafeRemediation string
	// Cause is the underlying error, if any.
	Cause error
}

// Diagnostics is the aggregated pure validation report. It records every
// defect observed without mutating anything; mutation failures use
// InstallError instead.
type Diagnostics []Diagnostic

// InstallError is the terminal no-false-success error. Primary is the
// deterministic cause, All carries every defect, and Rollback distinguishes
// an incomplete restoration once mutation has begun.
type InstallError struct {
	// Primary is the deterministic primary diagnostic.
	Primary Diagnostic
	// All lists every diagnostic in sorted order.
	All []Diagnostic
	// Rollback is non-nil when restoration after a write failure did not
	// fully converge; its Class is ErrorRollback.
	Rollback *Diagnostic
}

// Error renders only the deterministic primary cause. The complete report
// stays in All and Rollback so a summary can never mask a failure class or
// claim success.
func (e *InstallError) Error() string {
	summary := fmt.Sprintf("install %s failure at %s stage: %s", e.Primary.Class, e.Primary.Stage, e.Primary.Rule)
	if e.Primary.Cause != nil {
		summary += ": " + e.Primary.Cause.Error()
	}
	return summary
}

var _ error = (*InstallError)(nil)
