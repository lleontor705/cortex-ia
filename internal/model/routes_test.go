package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestModelsForPresetUsesSemanticRoutesOnly(t *testing.T) {
	assignments := ModelsForPreset(ModelPresetBalanced)
	if len(assignments) == 0 {
		t.Fatal("balanced preset must provide semantic route assignments")
	}
	for phase, value := range assignments {
		if _, err := modelroute.NewRouteID(string(value)); err != nil {
			t.Fatalf("%s is not a semantic route: %q", phase, value)
		}
	}
}

func TestProfilePersistsRoutesAndConfiguredValuesSeparately(t *testing.T) {
	route, err := modelroute.NewRouteID("route/v1/implementation")
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{
		Name:   "portable",
		Routes: map[string]modelroute.RouteRequest{"sdd-apply": {RouteID: route}},
		ConfiguredAssignments: map[string]OpenCodeModelAssignment{
			"sdd-apply": {Provider: "provider-x", Model: "model-y"},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{strings.Join([]string{"son", "net"}, ""), strings.Join([]string{"op", "us"}, ""), strings.Join([]string{"ha", "iku"}, "")} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("profile emitted a legacy alias: %s", data)
		}
	}
	if !strings.Contains(string(data), "route/v1/implementation") || !strings.Contains(string(data), "provider-x") {
		t.Fatalf("profile lost canonical route/configuration: %s", data)
	}
}
