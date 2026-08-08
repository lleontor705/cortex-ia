package config_test

// Integration tests for the project YAML boundary. Supported fields may reach
// installation; retired routing fields must fail during YAML loading, before
// any installer mutation is possible.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/config"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

const yamlSample = `
preset: custom
persona: professional
agents:
  - opencode
`

func TestSupportedYAML_FlowsThroughInstall(t *testing.T) {
	homeDir := t.TempDir()

	// Write .cortex-ia.yaml in a separate project directory and load it.
	projectDir := t.TempDir()
	yamlPath := filepath.Join(projectDir, config.FileName)
	if err := os.WriteFile(yamlPath, []byte(yamlSample), 0o644); err != nil {
		t.Fatalf("WriteFile yaml: %v", err)
	}
	cfg, err := config.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Apply supported YAML fields to the selection. Components are supplied by
	// the caller because project YAML does not select them.
	sel := model.Selection{Components: []model.ComponentID{model.ComponentCortex}}
	if err := config.ApplyToSelection(cfg, &sel); err != nil {
		t.Fatalf("ApplyToSelection: %v", err)
	}
	if len(sel.Agents) != 1 || sel.Agents[0] != model.AgentOpenCode {
		t.Fatalf("agents not propagated: %v", sel.Agents)
	}

	// Run install with a tiny registry containing only OpenCode.
	reg := agents.NewRegistry()
	reg.Register(opencode.NewAdapter())
	if _, err := pipeline.Install(homeDir, reg, sel, "test-v1", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := os.Stat(opencode.GlobalConfigPath(homeDir)); err != nil {
		t.Fatalf("supported YAML install did not write OpenCode configuration: %v", err)
	}

	// State records the supported selection, not a retired profile.
	st, err := state.Load(homeDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if len(st.InstalledAgents) != 1 || st.InstalledAgents[0] != model.AgentOpenCode {
		t.Errorf("state.InstalledAgents = %v, want [opencode]", st.InstalledAgents)
	}
	if st.LastProfile != "" {
		t.Errorf("state.LastProfile = %q, want empty", st.LastProfile)
	}
}

func TestRetiredYAMLRouting_FailsClosedWithoutInstallMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{name: "profile", yaml: "preset: custom\nprofile: cheap\nagents:\n  - opencode\n"},
		{name: "model preset", yaml: "preset: custom\nmodel-preset: economy\nagents:\n  - opencode\n"},
		{name: "profile driven model assignment", yaml: "preset: custom\nmodel-assignment:\n  implement: provider/model\nagents:\n  - opencode\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			yamlPath := filepath.Join(t.TempDir(), config.FileName)
			if err := os.WriteFile(yamlPath, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := config.LoadFile(yamlPath)
			var retired *config.RetiredProjectFieldError
			if !errors.As(err, &retired) {
				t.Fatalf("LoadFile error = %v, want RetiredProjectFieldError", err)
			}
			assertNoInstallMutation(t, homeDir)
		})
	}
}

func assertNoInstallMutation(t *testing.T, homeDir string) {
	t.Helper()
	entries, err := os.ReadDir(homeDir)
	if err != nil {
		t.Fatalf("ReadDir home: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("retired YAML mutated home; found entries: %v", entries)
	}
	for _, path := range []string{
		opencode.GlobalConfigPath(homeDir),
		state.StatePath(homeDir),
		state.LockPath(homeDir),
		state.InstallStatusPath(homeDir),
		filepath.Join(state.BaseDir(homeDir), "backups"),
		filepath.Join(state.BaseDir(homeDir), "workflow-receipt.json"),
		filepath.Join(state.BaseDir(homeDir), "ownership.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("retired YAML created managed path %q: %v", path, err)
		}
	}
}
