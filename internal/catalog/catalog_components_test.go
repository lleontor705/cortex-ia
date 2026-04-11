package catalog

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestAllComponents_Count(t *testing.T) {
	components := AllComponents()
	if len(components) != 8 {
		t.Errorf("expected 8 components, got %d", len(components))
	}
}

func TestAllComponents_NoDuplicateIDs(t *testing.T) {
	components := AllComponents()
	seen := make(map[model.ComponentID]bool)
	for _, c := range components {
		if seen[c.ID] {
			t.Errorf("duplicate component ID: %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestComponentMap_AllPresent(t *testing.T) {
	components := AllComponents()
	cmap := ComponentMap()
	if len(cmap) != len(components) {
		t.Errorf("map has %d entries, want %d", len(cmap), len(components))
	}
	for _, c := range components {
		if _, ok := cmap[c.ID]; !ok {
			t.Errorf("component %s missing from map", c.ID)
		}
	}
}

func TestComponentMap_LookupByID(t *testing.T) {
	cmap := ComponentMap()
	ids := []model.ComponentID{
		model.ComponentCortex,
		model.ComponentMailbox,
		model.ComponentForgeSpec,
		model.ComponentSDD,
		model.ComponentSkills,
		model.ComponentContext7,
		model.ComponentConventions,
		model.ComponentGGA,
	}
	for _, id := range ids {
		info, ok := cmap[id]
		if !ok {
			t.Errorf("lookup failed for %s", id)
			continue
		}
		if info.ID != id {
			t.Errorf("expected ID %s, got %s", id, info.ID)
		}
		if info.Name == "" {
			t.Errorf("component %s has empty name", id)
		}
	}
}

func TestResolveDeps_SingleNoDeps(t *testing.T) {
	resolved := ResolveDeps([]model.ComponentID{model.ComponentCortex})
	if len(resolved) != 1 {
		t.Errorf("expected 1 component, got %d", len(resolved))
	}
	if len(resolved) > 0 && resolved[0] != model.ComponentCortex {
		t.Errorf("expected cortex, got %s", resolved[0])
	}
}

func TestResolveDeps_WithDeps(t *testing.T) {
	resolved := ResolveDeps([]model.ComponentID{model.ComponentSDD})
	has := make(map[model.ComponentID]bool)
	for _, id := range resolved {
		has[id] = true
	}
	expected := []model.ComponentID{
		model.ComponentCortex,
		model.ComponentForgeSpec,
		model.ComponentMailbox,
		model.ComponentSDD,
	}
	for _, id := range expected {
		if !has[id] {
			t.Errorf("expected %s in resolved deps", id)
		}
	}
}

func TestResolveDeps_NoDuplicates_WithRedundantInput(t *testing.T) {
	// Pass cortex twice plus SDD (which depends on cortex)
	input := []model.ComponentID{
		model.ComponentCortex,
		model.ComponentSDD,
		model.ComponentCortex,
	}
	resolved := ResolveDeps(input)
	seen := make(map[model.ComponentID]bool)
	for _, id := range resolved {
		if seen[id] {
			t.Errorf("duplicate in resolved: %s", id)
		}
		seen[id] = true
	}
}

func TestResolveDeps_Empty(t *testing.T) {
	resolved := ResolveDeps([]model.ComponentID{})
	if len(resolved) != 0 {
		t.Errorf("expected 0 components for empty input, got %d", len(resolved))
	}
}

func TestComponentsForPreset_Custom(t *testing.T) {
	ids := ComponentsForPreset(model.PresetCustom)
	if ids != nil {
		t.Errorf("expected nil for custom preset, got %v", ids)
	}
}
