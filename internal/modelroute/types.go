// Package modelroute owns provider-neutral route contracts and their explicit
// configuration-backed resolution. Concrete provider/model values only cross
// this boundary through qualified configuration or discovery evidence.
package modelroute

import (
	"fmt"
	"regexp"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type RouteID string
type FallbackRouteID string
type ProviderID string
type ModelID string

func (r RouteID) String() string         { return string(r) }
func (r FallbackRouteID) String() string { return string(r) }

var routePattern = regexp.MustCompile(`^route/v[0-9]+/[a-z0-9][a-z0-9._-]*$`)

func NewRouteID(value string) (RouteID, error) {
	r := RouteID(value)
	if err := r.Validate(); err != nil {
		return "", err
	}
	return r, nil
}
func ParseRouteID(value string) (RouteID, error) { return NewRouteID(value) }
func NewFallbackRouteID(value string) (FallbackRouteID, error) {
	r := FallbackRouteID(value)
	if err := r.Validate(); err != nil {
		return "", err
	}
	return r, nil
}
func ParseFallbackRouteID(value string) (FallbackRouteID, error) { return NewFallbackRouteID(value) }
func (r RouteID) Validate() error {
	if !routePattern.MatchString(string(r)) {
		return fmt.Errorf("%s: invalid route id %q", ReasonInvalidRoute, r)
	}
	return nil
}
func (r FallbackRouteID) Validate() error {
	if !routePattern.MatchString(string(r)) {
		return fmt.Errorf("%s: invalid fallback route id %q", ReasonInvalidRoute, r)
	}
	return nil
}

type RouteRef struct {
	Provider ProviderID `json:"provider"`
	Model    ModelID    `json:"model"`
}

type CapabilityConstraint struct {
	ID            string    `json:"id"`
	Required      bool      `json:"required"`
	MinQuality    string    `json:"min_quality,omitempty"`
	ToolCall      bool      `json:"tool_call,omitempty"`
	Reasoning     string    `json:"reasoning,omitempty"`
	MinContext    int       `json:"min_context,omitempty"`
	LatencyPolicy string    `json:"latency_policy,omitempty"`
	CostPolicy    string    `json:"cost_policy,omitempty"`
	Trust         string    `json:"trust,omitempty"`
	Isolation     string    `json:"isolation,omitempty"`
	FreshUntil    time.Time `json:"fresh_until,omitempty"`
	Evidence      []string  `json:"evidence,omitempty"`
}
type CapabilityRequirement = CapabilityConstraint
type CapabilityRequirements []CapabilityRequirement

type RouteRequest struct {
	RouteID         RouteID                `json:"route_id"`
	FallbackRouteID FallbackRouteID        `json:"fallback_route_id,omitempty"`
	AllowFallback   bool                   `json:"allow_fallback,omitempty"`
	Constraints     []CapabilityConstraint `json:"constraints,omitempty"`
}

func (r RouteRequest) Validate() error {
	if err := r.RouteID.Validate(); err != nil {
		return err
	}
	if r.FallbackRouteID != "" {
		if err := r.FallbackRouteID.Validate(); err != nil {
			return err
		}
		if string(r.FallbackRouteID) == string(r.RouteID) {
			return fmt.Errorf("%s: fallback equals primary", ReasonInvalidRoute)
		}
		if !r.AllowFallback {
			return fmt.Errorf("%s: fallback not permitted", ReasonInvalidRoute)
		}
	}
	seen := map[string]bool{}
	for _, c := range r.Constraints {
		if c.ID == "" || seen[c.ID] {
			return fmt.Errorf("%s: invalid capability constraint", ReasonInvalidCapability)
		}
		seen[c.ID] = true
	}
	return nil
}

type RouteResolutionSource string

const (
	SourceUserConfig       RouteResolutionSource = "user-config"
	SourceProviderConfig   RouteResolutionSource = "provider-config"
	SourceRuntimeDiscovery RouteResolutionSource = "runtime-discovery"
	SourceGeneratedPreset  RouteResolutionSource = "generated-preset"
	SourceCompatibility    RouteResolutionSource = "compatibility-boundary"
)

type ResolutionEvidence struct {
	ID         string                `json:"id"`
	Source     RouteResolutionSource `json:"source"`
	Provider   ProviderID            `json:"provider"`
	Route      RouteID               `json:"route"`
	ObservedAt time.Time             `json:"observed_at"`
	FreshUntil time.Time             `json:"fresh_until"`
	Digest     string                `json:"digest"`
	Qualified  bool                  `json:"qualified"`
	ReasonID   string                `json:"reason_id"`
}
type ResolvedRoute struct {
	Role        ir.SemanticID          `json:"role,omitempty"`
	Requested   RouteRequest           `json:"requested"`
	PrimaryID   RouteID                `json:"primary_id"`
	Primary     RouteRef               `json:"primary"`
	FallbackID  FallbackRouteID        `json:"fallback_id,omitempty"`
	Fallback    *RouteRef              `json:"fallback,omitempty"`
	Constraints []CapabilityConstraint `json:"constraints,omitempty"`
	Evidence    []ResolutionEvidence   `json:"evidence"`
	Degradation string                 `json:"degradation,omitempty"`
}
type Profile struct {
	Name   string                  `json:"name"`
	Routes map[string]RouteRequest `json:"routes"`
}
type ProviderConfig struct {
	Provider     ProviderID                         `json:"provider"`
	Routes       map[RouteID]RouteRef               `json:"routes"`
	Capabilities map[RouteID][]CapabilityConstraint `json:"capabilities,omitempty"`
	Evidence     []ResolutionEvidence               `json:"evidence"`
}
type DiscoveredRoute struct {
	ID          RouteID                `json:"id"`
	Ref         RouteRef               `json:"ref"`
	Constraints []CapabilityConstraint `json:"constraints,omitempty"`
	Evidence    ResolutionEvidence     `json:"evidence"`
}
type CompatibilityInput struct {
	Version        string             `json:"version"`
	Aliases        map[string]RouteID `json:"aliases,omitempty"`
	HistoricalRead bool               `json:"historical_read,omitempty"`
}

const (
	ReasonInvalidRoute       = "route.invalid"
	ReasonInvalidCapability  = "capability.invalid"
	ReasonUnresolvedRoute    = "route.unresolved"
	ReasonStaleEvidence      = "route.stale-evidence"
	ReasonAmbiguousRoute     = "route.ambiguous"
	ReasonCapabilityMismatch = "route.capability-mismatch"
)
