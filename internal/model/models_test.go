package model

import (
	"strings"
	"testing"
)

func TestModelsForPreset_Balanced(t *testing.T) {
	m := ModelsForPreset(ModelPresetBalanced)
	if m["architect"] != ModelOpus {
		t.Errorf("balanced: architect = %q, want opus", m["architect"])
	}
	if m["implement"] != ModelSonnet {
		t.Errorf("balanced: implement = %q, want sonnet", m["implement"])
	}
	if m["finalize"] != ModelHaiku {
		t.Errorf("balanced: finalize = %q, want haiku", m["finalize"])
	}
}

func TestModelsForPreset_Performance(t *testing.T) {
	m := ModelsForPreset(ModelPresetPerformance)
	if m["architect"] != ModelOpus {
		t.Errorf("performance: architect = %q, want opus", m["architect"])
	}
	if m["validate"] != ModelOpus {
		t.Errorf("performance: validate = %q, want opus", m["validate"])
	}
}

func TestModelsForPreset_Economy(t *testing.T) {
	m := ModelsForPreset(ModelPresetEconomy)
	for phase, model := range m {
		if model == ModelOpus {
			t.Errorf("economy: %s = opus, expected sonnet or haiku", phase)
		}
	}
}

func TestModelsForPreset_Default(t *testing.T) {
	m := ModelsForPreset("unknown")
	if len(m) == 0 {
		t.Error("unknown preset should return balanced default")
	}
}

func TestFormatModelAssignments(t *testing.T) {
	m := ModelAssignments{"architect": ModelOpus, "implement": ModelSonnet}
	result := FormatModelAssignments(m)

	if !strings.Contains(result, "| architect | opus |") {
		t.Error("expected architect=opus row")
	}
	if !strings.Contains(result, "| implement | sonnet |") {
		t.Error("expected implement=sonnet row")
	}
	if !strings.Contains(result, "Phase") {
		t.Error("expected table header")
	}
}

func TestFormatModelAssignments_Empty(t *testing.T) {
	result := FormatModelAssignments(nil)
	if !strings.Contains(result, "No model") {
		t.Error("expected empty message")
	}
}
