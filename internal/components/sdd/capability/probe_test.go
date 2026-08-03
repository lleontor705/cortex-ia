package capability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestValidateProbeRefinementAcceptsDeclaredAuthority(t *testing.T) {
	now := time.Date(2026, time.July, 26, 20, 0, 0, 0, time.UTC)
	request := validProbeRequest(now)
	result := validProbeResult(now)

	refined, err := ApplyProbeResult(request, result)
	if err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
	if refined.Probe == nil || refined.Probe.Protocol != "mcp/capabilities@1" || refined.Probe.Result != "available:many" || refined.Probe.Timestamp != now || refined.Probe.EvidenceDigest != "sha256:redacted-evidence" {
		t.Fatalf("refined probe record = %+v", refined.Probe)
	}
	if refined.EvidenceClass != EvidenceExecutableProbe || refined.EvidenceRef != "sha256:redacted-evidence" || refined.ObservedAt != now {
		t.Fatalf("refined evidence = class %q ref %q observed %v", refined.EvidenceClass, refined.EvidenceRef, refined.ObservedAt)
	}
}

func TestValidateProbeRefinementRejectsAuthorityViolations(t *testing.T) {
	now := time.Date(2026, time.July, 26, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		mutate func(*ProbeRequest, *ProbeResult)
		code   ProbeErrorCode
	}{
		{
			name: "hard exclusion",
			mutate: func(request *ProbeRequest, _ *ProbeResult) {
				request.Authority.Excluded = true
			},
			code: ProbeErrorExcluded,
		},
		{
			name: "undeclared capability mode",
			mutate: func(_ *ProbeRequest, result *ProbeResult) {
				result.Refined.Mode = CapabilityAbsent
				result.Refined.Cardinality = CardinalityNone
			},
			code: ProbeErrorOutsideAuthority,
		},
		{
			name: "undeclared metadata mutation",
			mutate: func(_ *ProbeRequest, result *ProbeResult) {
				result.Refined.Current = false
			},
			code: ProbeErrorOutsideAuthority,
		},
		{
			name: "wider permission",
			mutate: func(_ *ProbeRequest, result *ProbeResult) {
				result.Permissions = append(result.Permissions, "network:write")
			},
			code: ProbeErrorPermissionWidening,
		},
		{
			name: "wider trust",
			mutate: func(_ *ProbeRequest, result *ProbeResult) {
				result.TrustClasses = append(result.TrustClasses, ir.TrustRemoteUntrusted)
			},
			code: ProbeErrorTrustWidening,
		},
		{
			name: "experimental without native opt in",
			mutate: func(_ *ProbeRequest, result *ProbeResult) {
				result.Refined.Experimental = true
			},
			code: ProbeErrorExperimentalOptIn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validProbeRequest(now)
			result := validProbeResult(now)
			tt.mutate(&request, &result)

			err := ValidateProbeRefinement(request, result)
			var probeErr *ProbeValidationError
			if !errors.As(err, &probeErr) {
				t.Fatalf("error = %v, want *ProbeValidationError", err)
			}
			if probeErr.Code != tt.code {
				t.Fatalf("code = %q, want %q", probeErr.Code, tt.code)
			}
		})
	}
}

func TestValidateProbeRefinementRequiresRedactedEvidenceRecord(t *testing.T) {
	now := time.Date(2026, time.July, 26, 20, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*ProbeRecord)
	}{
		{name: "protocol", mutate: func(record *ProbeRecord) { record.Protocol = "" }},
		{name: "result", mutate: func(record *ProbeRecord) { record.Result = "" }},
		{name: "timestamp", mutate: func(record *ProbeRecord) { record.Timestamp = time.Time{} }},
		{name: "redacted digest", mutate: func(record *ProbeRecord) { record.EvidenceDigest = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validProbeRequest(now)
			result := validProbeResult(now)
			tt.mutate(&result.Record)

			err := ValidateProbeRefinement(request, result)
			var probeErr *ProbeValidationError
			if !errors.As(err, &probeErr) || probeErr.Code != ProbeErrorInvalidEvidence {
				t.Fatalf("error = %v, want %q", err, ProbeErrorInvalidEvidence)
			}
		})
	}
}

func TestProberPortHasNoRuntimeControlSurface(t *testing.T) {
	var _ Prober = stubProber{}
}

type stubProber struct{}

func (stubProber) Probe(context.Context, ProbeRequest) (ProbeResult, error) {
	return ProbeResult{}, nil
}

func validProbeRequest(now time.Time) ProbeRequest {
	return ProbeRequest{
		Base: validCatalog(now).Facts[0],
		Authority: ProbeAuthority{
			CapabilityID:      "delegation/direct-child",
			RuntimeVersions:   versionRange("1.0.0", "1.9.0"),
			Modes:             []CapabilityValue{CapabilityAvailable},
			Cardinalities:     []Cardinality{CardinalityOne, CardinalityMany},
			Enforcement:       []EnforcementClass{EnforcementRuntime},
			TrustClasses:      []ir.TrustClass{ir.TrustToolOutput},
			Permissions:       []string{"runtime:inspect"},
			ExperimentalOptIn: false,
		},
	}
}

func validProbeResult(now time.Time) ProbeResult {
	refined := validCatalog(now).Facts[0]
	refined.RuntimeVersions = versionRange("1.0.0", "1.9.0")
	refined.Cardinality = CardinalityMany
	return ProbeResult{
		Record: ProbeRecord{
			ID:             "probe/direct-child",
			Method:         ProbeProtocol,
			Protocol:       "mcp/capabilities@1",
			Result:         "available:many",
			Timestamp:      now,
			EvidenceDigest: "sha256:redacted-evidence",
		},
		Refined:      refined,
		TrustClasses: []ir.TrustClass{ir.TrustToolOutput},
		Permissions:  []string{"runtime:inspect"},
	}
}
