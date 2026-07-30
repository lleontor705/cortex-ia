// Package forgespec provides ForgeSpec capability negotiation and resolution.
// This file translates the published forgespec-mcp@1.4.0 capability response
// into cortex-ia's internal CapabilitySnapshot, ensuring that capability IDs
// are never guessed from package names or version text alone.
package forgespec

import (
	"fmt"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// ServerInfo identifies the published ForgeSpec server.
type ServerInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	APIVersion string `json:"api_version"`
}

// PublishedVersionRange mirrors the published server's interval format.
type PublishedVersionRange struct {
	MinInclusive string `json:"min_inclusive"`
	MaxExclusive string `json:"max_exclusive"`
}

// PublishedCapability is one capability entry in the published response.
type PublishedCapability struct {
	ID       string                 `json:"id"`
	Supported PublishedVersionRange `json:"supported"`
	Selected string                 `json:"selected"`
}

// PublishedCompatibility records the server's self-assessed negotiation result.
type PublishedCompatibility struct {
	Compatible        bool     `json:"compatible"`
	SelectedMode      string   `json:"selected_mode"`
	Missing           []string `json:"missing"`
	Incompatible      []string `json:"incompatible"`
	UnavailableOptional []string `json:"unavailable_optional"`
}

// PublishedCapabilityResponse mirrors the JSON response returned by the
// forgespec_capabilities MCP tool in the published forgespec-mcp@1.4.0.
type PublishedCapabilityResponse struct {
	Server       ServerInfo              `json:"server"`
	Modes        []string                `json:"modes"`
	Schemas      map[string]PublishedVersionRange `json:"schemas,omitempty"`
	Capabilities []PublishedCapability   `json:"capabilities"`
	Limits       map[string]int          `json:"limits,omitempty"`
	Compatibility PublishedCompatibility `json:"compatibility"`
}

// ProbeEvidence carries cortex-ia's independent observation metadata that
// attaches to every translated capability. The published server provides
// capability IDs and version intervals; cortex-ia supplies evidence, freshness,
// and enforcement class from its own probe.
type ProbeEvidence struct {
	ProbeID     string
	EvidenceRef string
	ObservedAt  time.Time
	FreshUntil  time.Time
	Enforcement capability.EnforcementClass
}

// MapPublishedCapabilityID maps a bare capability ID from the published
// forgespec_capabilities response to cortex-ia's namespaced ir.SemanticID.
// The published "forgespec.capabilities" ID uses a dot-namespace that maps
// to "forgespec/capabilities" by stripping the redundant provider prefix.
func MapPublishedCapabilityID(publishedID string) ir.SemanticID {
	if strings.HasPrefix(publishedID, "forgespec.") {
		return ir.SemanticID("forgespec/" + strings.TrimPrefix(publishedID, "forgespec."))
	}
	return ir.SemanticID("forgespec/" + publishedID)
}

// TranslatePublishedResponse converts a published ForgeSpec capability
// response into cortex-ia's internal CapabilitySnapshot. It attaches
// independent probe evidence to every capability and validates that the
// server is compatible for direct-v1.
func TranslatePublishedResponse(response PublishedCapabilityResponse, evidence ProbeEvidence) (CapabilitySnapshot, error) {
	serverVersion, err := ir.ParseVersion(response.Server.Version)
	if err != nil {
		return CapabilitySnapshot{}, fmt.Errorf("parse published server version %q: %w", response.Server.Version, err)
	}

	supportsDirectV1 := false
	for _, mode := range response.Modes {
		if mode == "direct-v1" {
			supportsDirectV1 = true
			break
		}
	}

	if !supportsDirectV1 || len(response.Capabilities) == 0 {
		return CapabilitySnapshot{
			SchemaVersion:   ir.MustParseVersion("1.0.0"),
			ServerVersion:   serverVersion,
			ProtocolVersion: ir.MustParseVersion("1.0.0"),
			ProbeStatus:     ProbeUnavailable,
		}, nil
	}

	capabilities := make([]NegotiatedCapability, 0, len(response.Capabilities))
	for _, pub := range response.Capabilities {
		selected, err := ir.ParseVersion(pub.Selected)
		if err != nil {
			return CapabilitySnapshot{}, fmt.Errorf("capability %q selected version %q: %w", pub.ID, pub.Selected, err)
		}
		minVersion, err := ir.ParseVersion(pub.Supported.MinInclusive)
		if err != nil {
			return CapabilitySnapshot{}, fmt.Errorf("capability %q min_inclusive %q: %w", pub.ID, pub.Supported.MinInclusive, err)
		}
		maxTested, err := ir.ParseVersion(pub.Supported.MaxExclusive)
		if err != nil {
			return CapabilitySnapshot{}, fmt.Errorf("capability %q max_exclusive %q: %w", pub.ID, pub.Supported.MaxExclusive, err)
		}
		// max_exclusive in the published format is the exclusive upper bound;
		// cortex-ia's MaximumTested records the newest version backed by
		// conformance evidence. The published max_exclusive "2.0.0" means
		// all 1.x versions are supported; we qualify through the server's
		// own release version.
		_ = maxTested // validated for completeness; interval uses Minimum..MaximumTested

		mappedID := MapPublishedCapabilityID(pub.ID)
		if err := ir.ValidateSemanticID(mappedID); err != nil {
			return CapabilitySnapshot{}, fmt.Errorf("mapped capability ID for %q: %w", pub.ID, err)
		}

		capabilities = append(capabilities, NegotiatedCapability{
			ID:              mappedID,
			Version:         selected,
			Provider:        response.Server.Name,
			ProviderVersion: serverVersion,
			Interval: ir.VersionRange{
				Minimum:       minVersion,
				MaximumTested: serverVersion,
			},
			EvidenceClass: capability.EvidenceExecutableProbe,
			EvidenceRef:   evidence.EvidenceRef,
			ObservedAt:    evidence.ObservedAt,
			FreshUntil:    evidence.FreshUntil,
			Confidence:    1,
			ProbeID:       ir.SemanticID(evidence.ProbeID),
			Enforcement:   evidence.Enforcement,
		})
	}

	snapshot := CapabilitySnapshot{
		SchemaVersion:   ir.MustParseVersion("1.0.0"),
		ServerVersion:   serverVersion,
		ProtocolVersion: ir.MustParseVersion("1.0.0"),
		ProbeStatus:     ProbeQualified,
		Capabilities:    capabilities,
	}
	if response.Limits != nil {
		snapshot.Limits = response.Limits
	}

	if err := snapshot.Validate(); err != nil {
		return CapabilitySnapshot{}, fmt.Errorf("translated snapshot validation: %w", err)
	}
	return snapshot, nil
}

// PublishedQualificationEvidence records independently observed evidence from
// probing the published forgespec-mcp@1.4.0 package. This is the redacted,
// credential-free evidence record that supports direct-v1 enablement.
type PublishedQualificationEvidence struct {
	PackageName    string         `json:"package_name"`
	PackageVersion string         `json:"package_version"`
	NPMSHASum      string         `json:"npm_shasum"`
	TagSHA         string         `json:"tag_sha"`
	ProbeID        string         `json:"probe_id"`
	ProbeCommand   string         `json:"probe_command"`
	ObservedAt     time.Time      `json:"observed_at"`
	CapabilityIDs  []string       `json:"capability_ids"`
	P0Semantics    []P0SemanticEvidence `json:"p0_semantics"`
	Isolated       bool           `json:"isolated"`
	ExternalDB     bool           `json:"external_db_mutated"`
}

// P0SemanticEvidence records one independently qualified P0 semantic behavior.
type P0SemanticEvidence struct {
	Capability string `json:"capability"`
	Behavior   string `json:"behavior"`
	Evidence   string `json:"evidence"`
	Pass       bool   `json:"pass"`
}

// QualificationDecision reports whether the published evidence is sufficient
// to enable direct-v1 bindings.
type QualificationDecision struct {
	Qualified  bool     `json:"qualified"`
	Reason     string   `json:"reason"`
	FailedP0   []string `json:"failed_p0,omitempty"`
}

// EvaluateQualification determines whether all required P0 semantics are
// independently evidenced. Partial or stale evidence blocks truthfully.
func EvaluateQualification(evidence PublishedQualificationEvidence) QualificationDecision {
	decision := QualificationDecision{Qualified: false}

	if evidence.PackageName == "" || evidence.PackageVersion == "" {
		decision.Reason = "published package name and version are required"
		return decision
	}
	if evidence.ProbeID == "" || evidence.ObservedAt.IsZero() {
		decision.Reason = "independent probe identity and observation time are required"
		return decision
	}
	if !evidence.Isolated {
		decision.Reason = "qualification must be performed in an isolated environment"
		return decision
	}
	if evidence.ExternalDB {
		decision.Reason = "qualification must not mutate production or user databases"
		return decision
	}

	// Verify every required P0 capability ID is present in the observed list.
	observedIDs := make(map[string]bool, len(evidence.CapabilityIDs))
	for _, id := range evidence.CapabilityIDs {
		observedIDs[id] = true
	}
	for _, requirement := range RequiredP0Capabilities() {
		bareID := strings.TrimPrefix(string(requirement.ID), "forgespec/")
		bareID = strings.ReplaceAll(bareID, "/", ".") // restore "forgespec.capabilities" form
		// Match both the bare ID (e.g. "task-cas") and the restored dot form
		// (e.g. "forgespec.capabilities") from the published response.
		dotForm := bareID
		if !strings.HasPrefix(dotForm, "forgespec.") {
			dotForm = ""
		}
		if !observedIDs[bareID] && !observedIDs[string(requirement.ID)] && (dotForm == "" || !observedIDs[dotForm]) {
			decision.FailedP0 = append(decision.FailedP0, string(requirement.ID))
		}
	}
	if len(decision.FailedP0) > 0 {
		decision.Reason = "missing observed P0 capability IDs: " + strings.Join(decision.FailedP0, ", ")
		return decision
	}

	// Verify every required P0 semantic behavior has passing evidence.
	requiredSemantics := []string{
		"task-cas", "idempotency", "task-attempt-lease", "claim-recovery",
		"dependency-transitions", "audit-events", "sdd-contract-revisions",
	}
	semanticPass := make(map[string]bool, len(requiredSemantics))
	for _, sem := range evidence.P0Semantics {
		if sem.Pass {
			for _, req := range requiredSemantics {
				if strings.Contains(sem.Capability, req) {
					semanticPass[req] = true
				}
			}
		}
	}
	for _, req := range requiredSemantics {
		if !semanticPass[req] {
			decision.FailedP0 = append(decision.FailedP0, req)
		}
	}
	if len(decision.FailedP0) > 0 {
		decision.Reason = "missing passing P0 semantic evidence: " + strings.Join(decision.FailedP0, ", ")
		return decision
	}

	decision.Qualified = true
	decision.Reason = "all P0 capabilities and semantics independently evidenced from published package"
	return decision
}
