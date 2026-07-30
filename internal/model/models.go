package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// RoutesForPreset returns provider-neutral route requests. Presets select
// semantics only; provider/model resolution remains an explicit configuration
// concern.
func RoutesForPreset(preset ModelPreset) RouteAssignments {
	if preset != ModelPresetBalanced && preset != ModelPresetPerformance && preset != ModelPresetEconomy {
		return nil
	}
	name := "workflow"
	switch preset {
	case ModelPresetPerformance:
		name = "performance"
	case ModelPresetEconomy:
		name = "economy"
	}
	route, err := modelroute.NewRouteID("route/v1/" + name)
	if err != nil {
		return nil
	}
	assignments := RouteAssignments{}
	for _, phase := range []string{"orchestrator", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize"} {
		assignments[phase] = modelroute.RouteRequest{RouteID: route}
	}
	return assignments
}

// ModelsForPreset returns semantic route names for compatibility with older
// callers. It never returns provider/model identifiers or a concrete default.
func ModelsForPreset(preset ModelPreset) ModelAssignments {
	routes := RoutesForPreset(preset)
	if routes == nil {
		return nil
	}
	assignments := ModelAssignments{}
	for phase, request := range routes {
		assignments[phase] = string(request.RouteID)
	}
	return assignments
}

// FormatModelAssignments returns a markdown table for prompt injection.
func FormatModelAssignments(m ModelAssignments) string {
	if len(m) == 0 {
		return "No route assignments configured."
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("| Phase | Model |\n|-------|-------|\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %s |\n", k, m[k])
	}
	return b.String()
}
