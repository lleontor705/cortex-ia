package modelroute

import (
	"strings"
	"testing"
	"time"
)

func TestRouteIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"versioned semantic route", "route/v1/architecture", false},
		{"empty", "", true},
		{"unversioned", "architecture", true},
		{"provider model", "anthropic/claude", true},
		{"fallback provider model", "provider/v1/model", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := NewRouteID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRouteID(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if err == nil && route.String() != tt.value {
				t.Fatalf("route = %q, want %q", route, tt.value)
			}
		})
	}
}

func TestRouteRequestPreservesNoFallbackPolicy(t *testing.T) {
	route, err := NewRouteID("route/v1/security")
	if err != nil {
		t.Fatal(err)
	}
	request := RouteRequest{RouteID: route, Constraints: []CapabilityConstraint{{ID: "tool-call", Required: true}}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	if request.FallbackRouteID != "" || request.AllowFallback {
		t.Fatalf("request unexpectedly permits fallback: %+v", request)
	}
}

func TestResolveConfiguredRouteRecordsFreshEvidence(t *testing.T) {
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	route, _ := NewRouteID("route/v1/architecture")
	ref := RouteRef{Provider: "provider-x", Model: "model-y"}
	input := ResolverInput{
		Now:             now,
		Requests:        map[string]RouteRequest{"architecture": {RouteID: route, Constraints: []CapabilityConstraint{{ID: "tool-call", Required: true}}}},
		ProviderConfigs: []ProviderConfig{{Provider: "provider-x", Routes: map[RouteID]RouteRef{route: ref}, Capabilities: map[RouteID][]CapabilityConstraint{route: {{ID: "tool-call", Required: true}}}, Evidence: []ResolutionEvidence{{ID: "ev-1", Source: SourceProviderConfig, Provider: "provider-x", Route: route, ObservedAt: now.Add(-time.Minute), FreshUntil: now.Add(time.Hour), Digest: "digest", Qualified: true}}}},
	}
	resolved, _, err := NewResolver().Resolve(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	got := resolved["architecture"]
	if got.Primary != ref || got.Evidence[0].Digest != "digest" || got.Evidence[0].Source != SourceProviderConfig {
		t.Fatalf("resolution lost configured evidence: %+v", got)
	}
}

func TestResolveFailsClosedForStaleAmbiguousAndInventedValues(t *testing.T) {
	route, _ := NewRouteID("route/v1/architecture")
	now := time.Now().UTC()
	base := ResolverInput{Now: now, Requests: map[string]RouteRequest{"architecture": {RouteID: route}}}
	for _, tt := range []struct {
		name   string
		input  ResolverInput
		reason string
	}{
		{"stale", ResolverInput{Now: now, Requests: base.Requests, ProviderConfigs: []ProviderConfig{{Provider: "p", Routes: map[RouteID]RouteRef{route: {Provider: "p", Model: "m"}}, Evidence: []ResolutionEvidence{{ID: "stale", Source: SourceProviderConfig, Provider: "p", Route: route, FreshUntil: now.Add(-time.Minute), Qualified: true}}}}}, ReasonStaleEvidence},
		{"ambiguous", ResolverInput{Now: now, Requests: base.Requests, ProviderConfigs: []ProviderConfig{{Provider: "p", Routes: map[RouteID]RouteRef{route: {Provider: "p", Model: "m"}}, Evidence: []ResolutionEvidence{{ID: "a", Source: SourceProviderConfig, Provider: "p", Route: route, FreshUntil: now.Add(time.Hour), Qualified: true}}}, {Provider: "q", Routes: map[RouteID]RouteRef{route: {Provider: "q", Model: "n"}}, Evidence: []ResolutionEvidence{{ID: "b", Source: SourceProviderConfig, Provider: "q", Route: route, FreshUntil: now.Add(time.Hour), Qualified: true}}}}}, ReasonAmbiguousRoute},
		{"invented", ResolverInput{Now: now, Requests: base.Requests, ProviderConfigs: []ProviderConfig{{Provider: "p", Routes: map[RouteID]RouteRef{route: {Provider: "p", Model: ""}}, Evidence: []ResolutionEvidence{{ID: "x", Source: SourceProviderConfig, Provider: "p", Route: route, FreshUntil: now.Add(time.Hour), Qualified: true}}}}}, ReasonUnresolvedRoute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := NewResolver().Resolve(t.Context(), tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("error = %v, want reason %q", err, tt.reason)
			}
		})
	}
}

func TestCompatibilityDecoderIsVersionedAndIngressOnly(t *testing.T) {
	route, _ := NewRouteID("route/v1/architecture")
	decoder := NewCompatibilityDecoder("v1", map[string]RouteID{"legacy-architecture": route})
	request, evidence, err := decoder.Decode("legacy-architecture", CompatibilityInput{Version: "v1"})
	if err != nil || request.RouteID != route || evidence.Source != SourceCompatibility {
		t.Fatalf("decode = %#v, %#v, %v", request, evidence, err)
	}
	if _, _, err := decoder.Decode("legacy-architecture", CompatibilityInput{Version: "v2"}); err == nil {
		t.Fatal("unsupported decoder version unexpectedly succeeded")
	}
}
