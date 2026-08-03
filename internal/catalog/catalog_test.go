package catalog

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestComponentsForPresetFull(t *testing.T) {
	ids := ComponentsForPreset(model.PresetFull)
	if len(ids) != 7 {
		t.Errorf("expected 7 current components for full preset, got %d", len(ids))
	}
	assertComponentPresent(t, ids, model.ComponentMailbox)
}

func TestComponentsForPresetMinimal(t *testing.T) {
	ids := ComponentsForPreset(model.PresetMinimal)
	if len(ids) < 4 {
		t.Errorf("expected at least 4 components for minimal preset, got %d", len(ids))
	}
}

func TestResolveDeps_SDDPullsDeps(t *testing.T) {
	resolved := ResolveDeps([]model.ComponentID{model.ComponentSDD})

	has := make(map[model.ComponentID]bool)
	for _, id := range resolved {
		has[id] = true
	}

	if !has[model.ComponentCortex] {
		t.Error("SDD should pull cortex as dependency")
	}
	if !has[model.ComponentForgeSpec] {
		t.Error("SDD should pull forgespec as dependency")
	}
	if !has[model.ComponentMailbox] {
		t.Error("SDD should pull agent-mailbox as dependency")
	}
	if !has[model.ComponentSDD] {
		t.Error("SDD should be in resolved list")
	}
}

func TestResolveDeps_Order(t *testing.T) {
	resolved := ResolveDeps([]model.ComponentID{model.ComponentSDD})

	sddIdx := -1
	cortexIdx := -1
	for i, id := range resolved {
		if id == model.ComponentSDD {
			sddIdx = i
		}
		if id == model.ComponentCortex {
			cortexIdx = i
		}
	}

	if cortexIdx > sddIdx {
		t.Error("cortex should appear before SDD in dependency order")
	}
}

func TestResolveDeps_NoDuplicates(t *testing.T) {
	input := []model.ComponentID{
		model.ComponentCortex, model.ComponentSDD, model.ComponentCortex,
	}
	resolved := ResolveDeps(input)

	seen := make(map[model.ComponentID]bool)
	for _, id := range resolved {
		if seen[id] {
			t.Errorf("duplicate component: %s", id)
		}
		seen[id] = true
	}
}

func TestResolveDeps_MinimalPreset(t *testing.T) {
	minimal := ComponentsForPreset(model.PresetMinimal)
	resolved := ResolveDeps(minimal)

	has := make(map[model.ComponentID]bool)
	for _, id := range resolved {
		has[id] = true
	}

	if !has[model.ComponentMailbox] {
		t.Error("minimal preset should contain Mailbox through SDD dependencies")
	}
}

func assertComponentPresent(t *testing.T, components []model.ComponentID, wanted model.ComponentID) {
	t.Helper()
	for _, component := range components {
		if component == wanted {
			return
		}
	}
	t.Fatalf("current component %q is missing", wanted)
}
