package forgespec

import (
	"reflect"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestResolveCapabilitiesRequiresEveryFreshCompatibleP0Capability(t *testing.T) {
	now := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*CapabilitySnapshot)
		want   CoordinationMode
	}{
		{name: "complete P0 selects direct v1", want: CoordinationDirectV1},
		{name: "partial P0 blocks", mutate: func(snapshot *CapabilitySnapshot) { snapshot.Capabilities = snapshot.Capabilities[1:] }, want: CoordinationBlocked},
		{name: "stale P0 blocks", mutate: func(snapshot *CapabilitySnapshot) { snapshot.Capabilities[0].FreshUntil = now }, want: CoordinationBlocked},
		{name: "incompatible P0 blocks", mutate: func(snapshot *CapabilitySnapshot) { snapshot.Capabilities[0].Version = ir.MustParseVersion("2.0.0") }, want: CoordinationBlocked},
		{name: "package version alone is legacy only", mutate: func(snapshot *CapabilitySnapshot) {
			snapshot.Capabilities = nil
			snapshot.ServerVersion = ir.MustParseVersion("1.2.7")
			snapshot.ProbeStatus = ProbeUnavailable
		}, want: CoordinationLegacySequential},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validSnapshot(now)
			if tt.mutate != nil {
				tt.mutate(&snapshot)
			}
			result := ResolveCapabilities(snapshot, WorkflowRequirements{}, now)
			if result.Mode != tt.want {
				t.Fatalf("Mode = %q, want %q; missing=%v incompatible=%v", result.Mode, tt.want, result.Missing, result.Incompatible)
			}
		})
	}
}

func TestResolveCapabilitiesNegotiatesP1Independently(t *testing.T) {
	now := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	withoutP1 := validSnapshot(now)

	optional := ResolveCapabilities(withoutP1, WorkflowRequirements{}, now)
	if optional.Mode != CoordinationDirectV1 || optional.Execution != ExecutionSequentialNoConcurrentWrite {
		t.Fatalf("optional P1 resolution = %+v", optional)
	}
	if len(optional.Degradations) == 0 {
		t.Fatal("optional P1 omission has no visible degradation")
	}

	for _, requirements := range []WorkflowRequirements{
		{RequireApproval: true},
		{RequireConcurrentWrites: true},
	} {
		result := ResolveCapabilities(withoutP1, requirements, now)
		if result.Mode != CoordinationBlocked {
			t.Fatalf("requirements %+v resolved to %q, want blocked", requirements, result.Mode)
		}
	}
}

func TestCapabilitySnapshotDigestIsDeterministicAndEvidenceBacked(t *testing.T) {
	now := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	left := validSnapshot(now)
	right := validSnapshot(now)
	for i, j := 0, len(right.Capabilities)-1; i < j; i, j = i+1, j-1 {
		right.Capabilities[i], right.Capabilities[j] = right.Capabilities[j], right.Capabilities[i]
	}
	leftDigest, err := left.Digest()
	if err != nil {
		t.Fatal(err)
	}
	rightDigest, err := right.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("digest depends on capability order: %q != %q", leftDigest, rightDigest)
	}
	for _, item := range left.Capabilities {
		if item.Version == (ir.Version{}) || item.Provider == "" || item.ProviderVersion == (ir.Version{}) || item.EvidenceRef == "" || item.ProbeID == "" || item.FreshUntil.IsZero() || item.Enforcement == "" {
			t.Fatalf("capability lacks independent version/evidence contract: %+v", item)
		}
	}
}

func validSnapshot(now time.Time) CapabilitySnapshot {
	capabilities := make([]NegotiatedCapability, 0, len(RequiredP0Capabilities()))
	for _, requirement := range RequiredP0Capabilities() {
		capabilities = append(capabilities, NegotiatedCapability{
			ID: requirement.ID, Version: requirement.Versions.Minimum,
			Provider: "forgespec", ProviderVersion: ir.MustParseVersion("2.0.0"),
			Interval: requirement.Versions, EvidenceClass: capability.EvidenceExecutableProbe,
			EvidenceRef: "probe://forgespec/" + string(requirement.ID), ObservedAt: now.Add(-time.Minute),
			FreshUntil: now.Add(time.Hour), Confidence: 1, ProbeID: "probe/forgespec/capabilities",
			Enforcement: capability.EnforcementMCP,
		})
	}
	return CapabilitySnapshot{
		SchemaVersion: ir.MustParseVersion("1.0.0"), ServerVersion: ir.MustParseVersion("2.0.0"),
		ProtocolVersion: ir.MustParseVersion("1.0.0"), ProbeStatus: ProbeQualified,
		Capabilities: capabilities,
	}
}

func TestRequiredP0CapabilityIDsAreStable(t *testing.T) {
	got := make([]string, 0, len(RequiredP0Capabilities()))
	for _, requirement := range RequiredP0Capabilities() {
		got = append(got, string(requirement.ID))
	}
	want := PublishedP0CapabilityIDs
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("P0 IDs = %v, want %v", got, want)
	}
}

func TestRequiredP1CapabilityIDsAreStable(t *testing.T) {
	got := make([]string, 0, len(RequiredP1Capabilities()))
	for _, requirement := range RequiredP1Capabilities() {
		got = append(got, string(requirement.ID))
	}
	want := PublishedP1CapabilityIDs
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("P1 IDs = %v, want %v", got, want)
	}
}
