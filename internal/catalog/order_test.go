package catalog

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestTopoSort_FullPreset(t *testing.T) {
	components := ComponentsForPreset(model.PresetFull)
	groups, err := TopoSort(components)
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) < 2 {
		t.Errorf("expected at least 2 levels, got %d", len(groups))
	}

	// Level 0 should contain components with no deps.
	level0Set := make(map[model.ComponentID]bool)
	for _, id := range groups[0] {
		level0Set[id] = true
	}
	for _, id := range []model.ComponentID{model.ComponentCortex, model.ComponentCLIOrch, model.ComponentMailbox, model.ComponentForgeSpec, model.ComponentContext7, model.ComponentSkills} {
		if !level0Set[id] {
			t.Errorf("expected %s in level 0", id)
		}
	}

	// SDD should be in a later level (depends on cortex, forgespec, mailbox).
	flat := groups.Flatten()
	sddIdx := -1
	cortexIdx := -1
	for i, id := range flat {
		if id == model.ComponentSDD {
			sddIdx = i
		}
		if id == model.ComponentCortex {
			cortexIdx = i
		}
	}
	if sddIdx <= cortexIdx {
		t.Error("SDD should come after Cortex in topological order")
	}
}

func TestTopoSort_MinimalPreset(t *testing.T) {
	components := ComponentsForPreset(model.PresetMinimal)
	groups, err := TopoSort(components)
	if err != nil {
		t.Fatal(err)
	}

	flat := groups.Flatten()
	if len(flat) < 5 {
		t.Errorf("expected at least 5 components (with deps), got %d", len(flat))
	}
}

func TestTopoSort_SingleComponent(t *testing.T) {
	groups, err := TopoSort([]model.ComponentID{model.ComponentCortex})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0]) != 1 {
		t.Errorf("expected 1 group with 1 component, got %v", groups)
	}
}

func TestTopoSort_Empty(t *testing.T) {
	groups, err := TopoSort(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestParallelGroups_Flatten(t *testing.T) {
	groups := ParallelGroups{
		{model.ComponentCortex, model.ComponentMailbox},
		{model.ComponentConventions},
		{model.ComponentSDD},
	}
	flat := groups.Flatten()
	if len(flat) != 4 {
		t.Errorf("expected 4 items, got %d", len(flat))
	}
}
