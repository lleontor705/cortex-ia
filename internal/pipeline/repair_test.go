package pipeline

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestSelectionFromMetadataAcceptsCurrentMailboxComponent(t *testing.T) {
	s := state.State{
		InstalledAgents: []model.AgentID{model.AgentOpenCode},
		Preset:          model.PresetFull,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentMailbox},
	}
	lock := state.Lockfile{
		InstalledAgents: []model.AgentID{model.AgentOpenCode},
		Preset:          model.PresetMinimal,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
	}

	selection, err := selectionFromMetadata(s, lock)
	if err != nil {
		t.Fatalf("selectionFromMetadata() rejected current Mailbox component: %v", err)
	}
	found := false
	for _, component := range selection.Components {
		found = found || component == model.ComponentMailbox
	}
	if !found {
		t.Fatal("selectionFromMetadata() dropped current Mailbox component")
	}
}

func TestBuildInjectorsIncludesMailboxProvider(t *testing.T) {
	entries := buildInjectors("", nil, model.Selection{}, func() ([]string, error) { return nil, nil })
	for _, entry := range entries {
		if entry.id == model.ComponentMailbox {
			return
		}
	}
	t.Fatal("pipeline omitted the current Mailbox injector")
}

func TestSelectionFromMetadataRequiresAgents(t *testing.T) {
	_, err := selectionFromMetadata(state.State{}, state.Lockfile{})
	if err == nil {
		t.Fatal("selectionFromMetadata() expected error, got nil")
	}
}
