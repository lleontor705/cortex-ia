package model

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestModelsForPreset_Balanced(t *testing.T) {
	m := ModelsForPreset(ModelPresetBalanced)
	if _, err := modelroute.NewRouteID(string(m["architect"])); err != nil {
		t.Fatal(err)
	}
}

func TestModelsForPreset_Performance(t *testing.T) {
	m := ModelsForPreset(ModelPresetPerformance)
	if _, err := modelroute.NewRouteID(string(m["architect"])); err != nil {
		t.Fatal(err)
	}
}

func TestModelsForPreset_Economy(t *testing.T) {
	m := ModelsForPreset(ModelPresetEconomy)
	for phase, route := range m {
		if _, err := modelroute.NewRouteID(string(route)); err != nil {
			t.Errorf("economy: %s: %v", phase, err)
		}
	}
}

func TestModelsForPreset_Default(t *testing.T) {
	m := ModelsForPreset("unknown")
	if len(m) != 0 {
		t.Error("unknown preset must fail closed without a default")
	}
}

func TestFormatModelAssignments(t *testing.T) {
	m := ModelAssignments{"architect": "route/v1/architecture", "implement": "route/v1/implementation"}
	result := FormatModelAssignments(m)

	if !strings.Contains(result, "| architect | route/v1/architecture |") {
		t.Error("expected architecture route row")
	}
	if !strings.Contains(result, "| implement | route/v1/implementation |") {
		t.Error("expected implementation route row")
	}
	if !strings.Contains(result, "Phase") {
		t.Error("expected table header")
	}
}

func TestFormatModelAssignments_Empty(t *testing.T) {
	result := FormatModelAssignments(nil)
	if !strings.Contains(result, "No route") {
		t.Error("expected empty message")
	}
}
