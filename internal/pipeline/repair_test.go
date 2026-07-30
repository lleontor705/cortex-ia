package pipeline

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestSelectionFromMetadataRejectsRetiredStateComponent(t *testing.T) {
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

	_, err := selectionFromMetadata(s, lock)
	if err == nil {
		t.Fatal("selectionFromMetadata() accepted retired state component")
	}
}

func TestBuildInjectorsHasNoMailboxProvider(t *testing.T) {
	entries := buildInjectors("", nil, model.Selection{}, func() ([]string, error) { return nil, nil })
	for _, entry := range entries {
		if entry.id == model.ComponentMailbox {
			t.Fatal("pipeline still exposes a live Mailbox injector")
		}
	}
}

func TestSelectionFromMetadataRequiresAgents(t *testing.T) {
	_, err := selectionFromMetadata(state.State{}, state.Lockfile{})
	if err == nil {
		t.Fatal("selectionFromMetadata() expected error, got nil")
	}
}
