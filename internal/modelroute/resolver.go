package modelroute

import (
	"context"
	"fmt"
	"time"
)

type ResolverInput struct {
	UserProfiles    []Profile               `json:"user_profiles"`
	ActiveProfile   string                  `json:"active_profile"`
	ProviderConfigs []ProviderConfig        `json:"provider_configs"`
	Discovered      []DiscoveredRoute       `json:"discovered"`
	Requests        map[string]RouteRequest `json:"requests"`
	Compatibility   CompatibilityInput      `json:"compatibility,omitempty"`
	Now             time.Time               `json:"now"`
}

type Resolver struct{}

func NewResolver() *Resolver { return &Resolver{} }

type routeCandidate struct {
	ref         RouteRef
	evidence    ResolutionEvidence
	constraints []CapabilityConstraint
}

func (r *Resolver) Resolve(ctx context.Context, input ResolverInput) (map[string]ResolvedRoute, []ResolutionEvidence, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	requests := input.Requests
	if requests == nil {
		requests = map[string]RouteRequest{}
	}
	if len(requests) == 0 && input.ActiveProfile != "" {
		for _, profile := range input.UserProfiles {
			if profile.Name == input.ActiveProfile {
				requests = profile.Routes
				break
			}
		}
	}
	for name, request := range requests {
		if err := request.Validate(); err != nil {
			return nil, nil, err
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
		_ = name
	}
	byRoute := map[RouteID][]routeCandidate{}
	for _, config := range input.ProviderConfigs {
		for route, ref := range config.Routes {
			for _, evidence := range config.Evidence {
				if evidence.Route != route || evidence.Provider != config.Provider || evidence.Source == "" {
					continue
				}
				if !evidence.Qualified || ref.Provider == "" || ref.Model == "" || ref.Provider != config.Provider {
					continue
				}
				byRoute[route] = append(byRoute[route], routeCandidate{ref: ref, evidence: evidence, constraints: config.Capabilities[route]})
			}
		}
	}
	for _, discovered := range input.Discovered {
		evidence := discovered.Evidence
		if evidence.Route != discovered.ID || evidence.Provider != discovered.Ref.Provider || evidence.Source != SourceRuntimeDiscovery || !evidence.Qualified || discovered.Ref.Provider == "" || discovered.Ref.Model == "" {
			continue
		}
		byRoute[discovered.ID] = append(byRoute[discovered.ID], routeCandidate{ref: discovered.Ref, evidence: evidence, constraints: discovered.Constraints})
	}
	result := make(map[string]ResolvedRoute, len(requests))
	allEvidence := make([]ResolutionEvidence, 0)
	for name, request := range requests {
		primary, reason := selectCandidate(byRoute[request.RouteID], request.Constraints, now)
		if reason != "" && request.FallbackRouteID != "" && request.AllowFallback {
			fallback, fallbackReason := selectCandidate(byRoute[RouteID(request.FallbackRouteID)], request.Constraints, now)
			if fallbackReason == "" {
				primary = fallback
				primary.evidence.ReasonID = reason
				result[name] = ResolvedRoute{Requested: request, PrimaryID: request.RouteID, Primary: primary.ref, FallbackID: request.FallbackRouteID, Fallback: routeRefPtr(fallback.ref), Constraints: request.Constraints, Evidence: []ResolutionEvidence{primary.evidence, fallback.evidence}, Degradation: reason}
				allEvidence = append(allEvidence, primary.evidence, fallback.evidence)
				continue
			}
			_ = fallbackReason
		}
		if reason != "" {
			return nil, allEvidence, fmt.Errorf("%s: %s", reason, name)
		}
		resolved := ResolvedRoute{Requested: request, PrimaryID: request.RouteID, Primary: primary.ref, Constraints: request.Constraints, Evidence: []ResolutionEvidence{primary.evidence}}
		if request.FallbackRouteID != "" {
			resolved.FallbackID = request.FallbackRouteID
		}
		result[name] = resolved
		allEvidence = append(allEvidence, primary.evidence)
	}
	return result, allEvidence, nil
}

func routeRefPtr(ref RouteRef) *RouteRef { return &ref }

func selectCandidate(candidates []routeCandidate, required []CapabilityConstraint, now time.Time) (routeCandidate, string) {
	if len(candidates) == 0 {
		return routeCandidate{}, ReasonUnresolvedRoute
	}
	eligible := make([]routeCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !c.evidence.FreshUntil.After(now) || !c.evidence.Qualified {
			continue
		}
		if satisfies(required, c.constraints) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		for _, c := range candidates {
			if c.evidence.FreshUntil.Before(now) || !c.evidence.FreshUntil.After(now) {
				return routeCandidate{}, ReasonStaleEvidence
			}
		}
		return routeCandidate{}, ReasonCapabilityMismatch
	}
	if len(eligible) > 1 {
		return routeCandidate{}, ReasonAmbiguousRoute
	}
	return eligible[0], ""
}

func satisfies(required, available []CapabilityConstraint) bool {
	for _, want := range required {
		if !want.Required {
			continue
		}
		found := false
		for _, have := range available {
			if have.ID == want.ID && (!want.ToolCall || have.ToolCall) && (want.MinContext == 0 || have.MinContext >= want.MinContext) && (want.Reasoning == "" || have.Reasoning == want.Reasoning) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
