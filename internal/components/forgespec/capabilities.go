package forgespec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type ProbeStatus string

const (
	ProbeQualified   ProbeStatus = "qualified"
	ProbeUnavailable ProbeStatus = "unavailable"
)

type NegotiatedCapability struct {
	ID              ir.SemanticID               `json:"id"`
	Version         ir.Version                  `json:"version"`
	Provider        string                      `json:"provider"`
	ProviderVersion ir.Version                  `json:"provider_version"`
	Interval        ir.VersionRange             `json:"interval"`
	EvidenceClass   capability.EvidenceClass    `json:"evidence_class"`
	EvidenceRef     string                      `json:"evidence_ref"`
	ObservedAt      time.Time                   `json:"observed_at"`
	FreshUntil      time.Time                   `json:"fresh_until"`
	Confidence      capability.Confidence       `json:"confidence"`
	Experimental    bool                        `json:"experimental"`
	ProbeID         ir.SemanticID               `json:"probe_id"`
	Enforcement     capability.EnforcementClass `json:"enforcement"`
}

type CapabilitySnapshot struct {
	SchemaVersion   ir.Version             `json:"schema_version"`
	ServerVersion   ir.Version             `json:"server_version"`
	ProtocolVersion ir.Version             `json:"protocol_version"`
	ProbeStatus     ProbeStatus            `json:"probe_status"`
	Capabilities    []NegotiatedCapability `json:"capabilities"`
	Limits          map[string]int         `json:"limits,omitempty"`
}

type CapabilityRequirement struct {
	ID       ir.SemanticID   `json:"id"`
	Versions ir.VersionRange `json:"versions"`
}

type CoordinationMode string

const (
	CoordinationDirectV1         CoordinationMode = "direct-v1"
	CoordinationLegacySequential CoordinationMode = "legacy-sequential"
	CoordinationBlocked          CoordinationMode = "blocked"
)

type ExecutionMode string

const (
	ExecutionDirectQualified             ExecutionMode = "direct-qualified"
	ExecutionSequentialNoConcurrentWrite ExecutionMode = "sequential-no-concurrent-write"
)

type WorkflowRequirements struct {
	RequireDirectV1         bool `json:"require_direct_v1"`
	RequireApproval         bool `json:"require_approval"`
	RequireConcurrentWrites bool `json:"require_concurrent_writes"`
}

type Degradation struct {
	CapabilityID ir.SemanticID `json:"capability_id"`
	Reason       string        `json:"reason"`
}

type ForgeSpecResolution struct {
	Mode                  CoordinationMode        `json:"mode"`
	Execution             ExecutionMode           `json:"execution"`
	Snapshot              CapabilitySnapshot      `json:"snapshot"`
	Missing               []CapabilityRequirement `json:"missing"`
	Incompatible          []CapabilityRequirement `json:"incompatible"`
	Degradations          []Degradation           `json:"degradations"`
	UnsupportedGuarantees []string                `json:"unsupported_guarantees"`
}

var (
	capabilityVersion = ir.VersionRange{Minimum: ir.MustParseVersion("1.0.0"), MaximumTested: ir.MustParseVersion("1.4.0")}
	p0Capabilities    = []CapabilityRequirement{
		{ID: "forgespec/capabilities", Versions: capabilityVersion},
		{ID: "forgespec/task-cas", Versions: capabilityVersion},
		{ID: "forgespec/idempotency", Versions: capabilityVersion},
		{ID: "forgespec/task-attempt-lease", Versions: capabilityVersion},
		{ID: "forgespec/claim-recovery", Versions: capabilityVersion},
		{ID: "forgespec/dependency-transitions", Versions: capabilityVersion},
		{ID: "forgespec/audit-events", Versions: capabilityVersion},
		{ID: "forgespec/sdd-contract-revisions", Versions: capabilityVersion},
	}
	p1Capabilities = []CapabilityRequirement{
		{ID: "forgespec/approval-gates", Versions: capabilityVersion},
		{ID: "forgespec/batch-status", Versions: capabilityVersion},
		{ID: "forgespec/query-cursors", Versions: capabilityVersion},
		{ID: "forgespec/file-lease", Versions: capabilityVersion},
		{ID: "forgespec/structured-evidence-links", Versions: capabilityVersion},
	}
)

func RequiredP0Capabilities() []CapabilityRequirement { return slices.Clone(p0Capabilities) }

func RequiredP1Capabilities() []CapabilityRequirement { return slices.Clone(p1Capabilities) }

func (snapshot CapabilitySnapshot) Validate() error {
	if snapshot.SchemaVersion.Major != 1 || snapshot.ServerVersion.Major == 0 {
		return fmt.Errorf("snapshot schema and server versions are required and compatible")
	}
	if snapshot.ProbeStatus == ProbeUnavailable {
		if len(snapshot.Capabilities) != 0 {
			return fmt.Errorf("unavailable probe cannot advertise capabilities")
		}
		return nil
	}
	if snapshot.ProbeStatus != ProbeQualified || snapshot.ProtocolVersion.Major != 1 {
		return fmt.Errorf("qualified probe and compatible protocol version are required")
	}
	seen := make(map[ir.SemanticID]struct{}, len(snapshot.Capabilities))
	for _, item := range snapshot.Capabilities {
		if err := validateCapability(item); err != nil {
			return fmt.Errorf("capability %q: %w", item.ID, err)
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return fmt.Errorf("duplicate capability %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func validateCapability(item NegotiatedCapability) error {
	if err := ir.ValidateSemanticID(item.ID); err != nil {
		return err
	}
	if item.Version.Major == 0 || strings.TrimSpace(item.Provider) == "" || item.ProviderVersion.Major == 0 || !containsVersion(item.Interval, item.Version) {
		return fmt.Errorf("independent capability and provider versions are incomplete")
	}
	if item.EvidenceClass != capability.EvidenceExecutableProbe && item.EvidenceClass != capability.EvidenceRuntimeObserved && item.EvidenceClass != capability.EvidenceInstalledSchema {
		return fmt.Errorf("runtime qualification evidence is required")
	}
	if strings.TrimSpace(item.EvidenceRef) == "" || item.ObservedAt.IsZero() || item.FreshUntil.Before(item.ObservedAt) || item.Confidence <= 0 || item.Confidence > 1 {
		return fmt.Errorf("evidence, freshness, and confidence are incomplete")
	}
	if err := ir.ValidateSemanticID(item.ProbeID); err != nil {
		return fmt.Errorf("probe ID: %w", err)
	}
	if item.Enforcement != capability.EnforcementRuntime && item.Enforcement != capability.EnforcementHook && item.Enforcement != capability.EnforcementMCP {
		return fmt.Errorf("qualified enforcement is required")
	}
	return nil
}

func (snapshot CapabilitySnapshot) Digest() (string, error) {
	if err := snapshot.Validate(); err != nil {
		return "", err
	}
	normalized := snapshot
	normalized.Capabilities = slices.Clone(snapshot.Capabilities)
	slices.SortFunc(normalized.Capabilities, func(left, right NegotiatedCapability) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func ResolveCapabilities(snapshot CapabilitySnapshot, requirements WorkflowRequirements, now time.Time) ForgeSpecResolution {
	result := ForgeSpecResolution{Mode: CoordinationBlocked, Snapshot: snapshot}
	if err := snapshot.Validate(); err != nil {
		result.Incompatible = append(result.Incompatible, CapabilityRequirement{ID: "forgespec/capability-snapshot", Versions: capabilityVersion})
		return result
	}
	if snapshot.ProbeStatus == ProbeUnavailable {
		if snapshot.ServerVersion.Major == 1 && snapshot.ServerVersion.Minor == 2 && !requirements.RequireDirectV1 && !requirements.RequireApproval && !requirements.RequireConcurrentWrites {
			result.Mode = CoordinationLegacySequential
			result.Execution = ExecutionSequentialNoConcurrentWrite
			result.UnsupportedGuarantees = []string{"parallelism", "takeover", "automatic-recovery", "enforced-approval", "atomic-file-reservations", "immutable-attempts", "full-sdd-revisions"}
		}
		return result
	}

	index := make(map[ir.SemanticID]NegotiatedCapability, len(snapshot.Capabilities))
	for _, item := range snapshot.Capabilities {
		index[item.ID] = item
	}
	for _, required := range p0Capabilities {
		item, found := index[required.ID]
		if !found {
			result.Missing = append(result.Missing, required)
			continue
		}
		if !containsVersion(required.Versions, item.Version) || item.ObservedAt.After(now) || !item.FreshUntil.After(now) {
			result.Incompatible = append(result.Incompatible, required)
		}
	}
	if len(result.Missing) != 0 || len(result.Incompatible) != 0 {
		return result
	}

	result.Mode = CoordinationDirectV1
	result.Execution = ExecutionDirectQualified
	for _, optional := range p1Capabilities {
		item, found := index[optional.ID]
		qualified := found && containsVersion(optional.Versions, item.Version) && !item.ObservedAt.After(now) && item.FreshUntil.After(now)
		if qualified {
			continue
		}
		if optional.ID == "forgespec/approval-gates" && requirements.RequireApproval || optional.ID == "forgespec/file-lease" && requirements.RequireConcurrentWrites {
			result.Mode = CoordinationBlocked
			result.Missing = append(result.Missing, optional)
			continue
		}
		result.Degradations = append(result.Degradations, Degradation{CapabilityID: optional.ID, Reason: "optional P1 capability is not qualified"})
		if optional.ID == "forgespec/file-lease" {
			result.Execution = ExecutionSequentialNoConcurrentWrite
		}
	}
	return result
}

func containsVersion(interval ir.VersionRange, version ir.Version) bool {
	return version.Major == interval.Minimum.Major && compareVersion(version, interval.Minimum) >= 0 && compareVersion(version, interval.MaximumTested) <= 0
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
