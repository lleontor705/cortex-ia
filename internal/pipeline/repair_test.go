package pipeline

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestSelectionFromMetadataPrefersLockComponents(t *testing.T) {
	s := state.State{
		InstalledAgents: []model.AgentID{model.AgentCodex},
		Preset:          model.PresetFull,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentMailbox},
	}
	lock := state.Lockfile{
		InstalledAgents: []model.AgentID{model.AgentCodex, model.AgentCodex},
		Preset:          model.PresetMinimal,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
	}

	selection, err := selectionFromMetadata(s, lock)
	if err != nil {
		t.Fatalf("selectionFromMetadata() error = %v", err)
	}

	if selection.Preset != model.PresetMinimal {
		t.Fatalf("selectionFromMetadata() preset = %q, want %q", selection.Preset, model.PresetMinimal)
	}

	if len(selection.Agents) != 1 || selection.Agents[0] != model.AgentCodex {
		t.Fatalf("selectionFromMetadata() agents = %v, want [%s]", selection.Agents, model.AgentCodex)
	}

	if len(selection.Components) != 3 {
		t.Fatalf("selectionFromMetadata() components = %v, want 3 entries", selection.Components)
	}

	if selection.Components[0] != model.ComponentCortex || selection.Components[1] != model.ComponentSDD || selection.Components[2] != model.ComponentMailbox {
		t.Fatalf("selectionFromMetadata() component order = %v, want lock components first then state-only components", selection.Components)
	}
}

func TestSelectionFromMetadataRequiresAgents(t *testing.T) {
	_, err := selectionFromMetadata(state.State{}, state.Lockfile{})
	if err == nil {
		t.Fatal("selectionFromMetadata() expected error, got nil")
	}
}
