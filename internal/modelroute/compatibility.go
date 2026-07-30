package modelroute

import (
	"fmt"
	"strings"
)

// CompatibilityDecoder is deliberately small and ingress-only. Callers must
// discard the input spelling and persist the returned canonical request.
type CompatibilityDecoder struct {
	version string
	aliases map[string]RouteID
}

func NewCompatibilityDecoder(version string, aliases map[string]RouteID) *CompatibilityDecoder {
	copyAliases := make(map[string]RouteID, len(aliases))
	for alias, route := range aliases {
		copyAliases[alias] = route
	}
	return &CompatibilityDecoder{version: version, aliases: copyAliases}
}

func (d *CompatibilityDecoder) Decode(value string, input CompatibilityInput) (RouteRequest, ResolutionEvidence, error) {
	if d == nil || input.Version == "" || input.Version != d.version {
		return RouteRequest{}, ResolutionEvidence{}, fmt.Errorf("%s: unsupported compatibility version", ReasonInvalidRoute)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return RouteRequest{}, ResolutionEvidence{}, fmt.Errorf("%s: empty compatibility value", ReasonInvalidRoute)
	}
	if route, err := NewRouteID(value); err == nil {
		return RouteRequest{RouteID: route}, ResolutionEvidence{ID: "compat:" + d.version, Source: SourceCompatibility, Route: route, ReasonID: "compatibility.canonical"}, nil
	}
	route, ok := d.aliases[value]
	if !ok {
		return RouteRequest{}, ResolutionEvidence{}, fmt.Errorf("%s: unsupported compatibility alias %q", ReasonInvalidRoute, value)
	}
	if err := route.Validate(); err != nil {
		return RouteRequest{}, ResolutionEvidence{}, err
	}
	return RouteRequest{RouteID: route}, ResolutionEvidence{ID: "compat:" + d.version + ":" + value, Source: SourceCompatibility, Route: route, ReasonID: "compatibility.translated"}, nil
}

var _ interface {
	Decode(string, CompatibilityInput) (RouteRequest, ResolutionEvidence, error)
} = (*CompatibilityDecoder)(nil)
