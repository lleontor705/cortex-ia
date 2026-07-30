package forgespec

import (
	"reflect"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// PublishedP0CapabilityIDs are the exact capability IDs returned by the
// published forgespec-mcp@1.4.0 forgespec_capabilities tool. These are NOT
// guessed from source — they are independently observed from the live
// published npm package.
var PublishedP0CapabilityIDs = []string{
	"forgespec/capabilities",
	"forgespec/task-cas",
	"forgespec/idempotency",
	"forgespec/task-attempt-lease",
	"forgespec/claim-recovery",
	"forgespec/dependency-transitions",
	"forgespec/audit-events",
	"forgespec/sdd-contract-revisions",
}

// PublishedP1CapabilityIDs are the exact optional capability IDs from the
// published forgespec-mcp@1.4.0.
var PublishedP1CapabilityIDs = []string{
	"forgespec/approval-gates",
	"forgespec/batch-status",
	"forgespec/query-cursors",
	"forgespec/file-lease",
	"forgespec/structured-evidence-links",
}

// PublishedServerVersion is the exact published server version.
const PublishedServerVersion = "1.4.0"

func TestRequiredP0CapabilityIDsMatchPublishedServer(t *testing.T) {
	got := make([]string, 0, len(RequiredP0Capabilities()))
	for _, requirement := range RequiredP0Capabilities() {
		got = append(got, string(requirement.ID))
	}
	if !reflect.DeepEqual(got, PublishedP0CapabilityIDs) {
		t.Fatalf("P0 IDs do not match published forgespec-mcp@1.4.0:\n  got:  %v\n  want: %v", got, PublishedP0CapabilityIDs)
	}
}

func TestRequiredP1CapabilityIDsMatchPublishedServer(t *testing.T) {
	got := make([]string, 0, len(RequiredP1Capabilities()))
	for _, requirement := range RequiredP1Capabilities() {
		got = append(got, string(requirement.ID))
	}
	if !reflect.DeepEqual(got, PublishedP1CapabilityIDs) {
		t.Fatalf("P1 IDs do not match published forgespec-mcp@1.4.0:\n  got:  %v\n  want: %v", got, PublishedP1CapabilityIDs)
	}
}

func TestCapabilityVersionIntervalMatchesPublishedServer(t *testing.T) {
	for _, requirement := range RequiredP0Capabilities() {
		if requirement.Versions.Minimum != ir.MustParseVersion("1.0.0") {
			t.Fatalf("P0 %s minimum = %s, want 1.0.0", requirement.ID, requirement.Versions.Minimum)
		}
		if requirement.Versions.MaximumTested != ir.MustParseVersion("1.4.0") {
			t.Fatalf("P0 %s maximum_tested = %s, want 1.4.0 (qualified through published release)", requirement.ID, requirement.Versions.MaximumTested)
		}
	}
	for _, requirement := range RequiredP1Capabilities() {
		if requirement.Versions.Minimum != ir.MustParseVersion("1.0.0") {
			t.Fatalf("P1 %s minimum = %s, want 1.0.0", requirement.ID, requirement.Versions.Minimum)
		}
		if requirement.Versions.MaximumTested != ir.MustParseVersion("1.4.0") {
			t.Fatalf("P1 %s maximum_tested = %s, want 1.4.0", requirement.ID, requirement.Versions.MaximumTested)
		}
	}
}

func TestMapPublishedCapabilityMapsBareIDToNamespacedSemanticID(t *testing.T) {
	tests := []struct {
		published string
		want      ir.SemanticID
	}{
		{published: "forgespec.capabilities", want: "forgespec/capabilities"},
		{published: "task-cas", want: "forgespec/task-cas"},
		{published: "idempotency", want: "forgespec/idempotency"},
		{published: "task-attempt-lease", want: "forgespec/task-attempt-lease"},
		{published: "claim-recovery", want: "forgespec/claim-recovery"},
		{published: "dependency-transitions", want: "forgespec/dependency-transitions"},
		{published: "audit-events", want: "forgespec/audit-events"},
		{published: "sdd-contract-revisions", want: "forgespec/sdd-contract-revisions"},
		{published: "approval-gates", want: "forgespec/approval-gates"},
		{published: "batch-status", want: "forgespec/batch-status"},
		{published: "query-cursors", want: "forgespec/query-cursors"},
		{published: "file-lease", want: "forgespec/file-lease"},
		{published: "structured-evidence-links", want: "forgespec/structured-evidence-links"},
	}
	for _, tt := range tests {
		t.Run(tt.published, func(t *testing.T) {
			got := MapPublishedCapabilityID(tt.published)
			if got != tt.want {
				t.Fatalf("MapPublishedCapabilityID(%q) = %q, want %q", tt.published, got, tt.want)
			}
			if err := ir.ValidateSemanticID(got); err != nil {
				t.Fatalf("mapped ID %q fails semantic ID validation: %v", got, err)
			}
		})
	}
}

func TestPublishedCapabilityResponseTranslatesToCapabilitySnapshot(t *testing.T) {
	now := time.Date(2026, time.July, 27, 16, 0, 0, 0, time.UTC)
	response := PublishedCapabilityResponse{
		Server:     ServerInfo{Name: "forgespec-mcp", Version: PublishedServerVersion, APIVersion: "1.0.0"},
		Modes:      []string{"legacy", "direct-v1"},
		Capabilities: []PublishedCapability{
			{ID: "forgespec.capabilities", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "task-cas", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "idempotency", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "task-attempt-lease", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "claim-recovery", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "dependency-transitions", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "audit-events", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "sdd-contract-revisions", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "approval-gates", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "batch-status", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "query-cursors", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "file-lease", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
			{ID: "structured-evidence-links", Supported: PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"}, Selected: "1.0.0"},
		},
		Compatibility: PublishedCompatibility{Compatible: true, SelectedMode: "direct-v1"},
	}

	evidence := ProbeEvidence{
		ProbeID:      "probe/forgespec/capabilities",
		EvidenceRef:  "npm:forgespec-mcp@1.4.0/forgespec_capabilities",
		ObservedAt:   now.Add(-time.Minute),
		FreshUntil:   now.Add(time.Hour),
		Enforcement:  capability.EnforcementMCP,
	}

	snapshot, err := TranslatePublishedResponse(response, evidence)
	if err != nil {
		t.Fatalf("TranslatePublishedResponse error: %v", err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("snapshot validation error: %v", err)
	}

	result := ResolveCapabilities(snapshot, WorkflowRequirements{}, now)
	if result.Mode != CoordinationDirectV1 {
		t.Fatalf("ResolveCapabilities mode = %q, want direct-v1; missing=%v incompatible=%v", result.Mode, result.Missing, result.Incompatible)
	}

	for _, cap := range snapshot.Capabilities {
		if cap.ID == "" || cap.Version.Major == 0 || cap.Provider == "" || cap.EvidenceRef == "" || cap.ProbeID == "" {
			t.Fatalf("capability lacks evidence contract: %+v", cap)
		}
	}
}

func TestTranslatePublishedResponseRejectsIncompatibleServer(t *testing.T) {
	response := PublishedCapabilityResponse{
		Server:        ServerInfo{Name: "forgespec-mcp", Version: "1.2.7", APIVersion: "1.0.0"},
		Modes:         []string{"legacy"},
		Capabilities:  nil,
		Compatibility: PublishedCompatibility{Compatible: true, SelectedMode: "legacy"},
	}
	evidence := ProbeEvidence{ProbeID: "probe/forgespec/capabilities", EvidenceRef: "npm:forgespec-mcp@1.2.7", ObservedAt: time.Now(), FreshUntil: time.Now().Add(time.Hour), Enforcement: capability.EnforcementMCP}

	snapshot, err := TranslatePublishedResponse(response, evidence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.ProbeStatus != ProbeUnavailable {
		t.Fatalf("1.2.x server should be unavailable for direct-v1: %+v", snapshot)
	}

	result := ResolveCapabilities(snapshot, WorkflowRequirements{}, time.Now())
	if result.Mode != CoordinationLegacySequential {
		t.Fatalf("1.2.x server resolved to %q, want legacy-sequential", result.Mode)
	}
}

func TestResolveCapabilitiesBlocksOnPartialP0FromPublishedIDs(t *testing.T) {
	now := time.Date(2026, time.July, 27, 16, 0, 0, 0, time.UTC)
	capabilities := make([]NegotiatedCapability, 0, len(RequiredP0Capabilities())-1)
	for i, requirement := range RequiredP0Capabilities() {
		if i == 0 {
			continue // skip first to simulate partial
		}
		capabilities = append(capabilities, NegotiatedCapability{
			ID: requirement.ID, Version: requirement.Versions.Minimum,
			Provider: "forgespec", ProviderVersion: ir.MustParseVersion(PublishedServerVersion),
			Interval: requirement.Versions, EvidenceClass: capability.EvidenceExecutableProbe,
			EvidenceRef: "probe://forgespec/" + string(requirement.ID), ObservedAt: now.Add(-time.Minute),
			FreshUntil: now.Add(time.Hour), Confidence: 1, ProbeID: "probe/forgespec/capabilities",
			Enforcement: capability.EnforcementMCP,
		})
	}
	snapshot := CapabilitySnapshot{
		SchemaVersion: ir.MustParseVersion("1.0.0"), ServerVersion: ir.MustParseVersion(PublishedServerVersion),
		ProtocolVersion: ir.MustParseVersion("1.0.0"), ProbeStatus: ProbeQualified,
		Capabilities: capabilities,
	}
	result := ResolveCapabilities(snapshot, WorkflowRequirements{}, now)
	if result.Mode != CoordinationBlocked {
		t.Fatalf("partial P0 resolved to %q, want blocked; missing=%v", result.Mode, result.Missing)
	}
	if len(result.Missing) != 1 {
		t.Fatalf("expected exactly 1 missing capability, got %d", len(result.Missing))
	}
}
