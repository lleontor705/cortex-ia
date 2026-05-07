package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func newTestRegistry(t *testing.T) *agents.Registry {
	t.Helper()
	r := agents.NewRegistry()
	r.Register(claude.NewAdapter())
	r.Register(opencode.NewAdapter())
	return r
}

func writeStateWithAgents(t *testing.T, home string, ids ...model.AgentID) {
	t.Helper()
	if err := state.EnsureDir(home); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := state.Save(home, state.State{
		InstalledAgents: ids,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentPersona, model.ComponentSDD},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestService_Apply_RemovesPersonaSection(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	prompt := filepath.Join(home, ".claude", "CLAUDE.md")
	_ = os.MkdirAll(filepath.Dir(prompt), 0o755)
	original := "# CLAUDE\n\n<!-- cortex-ia:cortex-persona -->\nManaged tone\n<!-- /cortex-ia:cortex-persona -->\n\nMy own notes.\n"
	if err := os.WriteFile(prompt, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	res, err := svc.Apply(Selection{Components: []model.ComponentID{model.ComponentPersona}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.ChangedFiles) == 0 {
		t.Errorf("expected at least one changed file, got %+v", res)
	}

	got, _ := os.ReadFile(prompt)
	if strings.Contains(string(got), "cortex-persona") {
		t.Errorf("persona marker still present:\n%s", string(got))
	}
	if !strings.Contains(string(got), "My own notes.") {
		t.Errorf("user content was wiped:\n%s", string(got))
	}
}

func TestService_Apply_RemovesCortexMCP_Claude(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	mcpFile := filepath.Join(home, ".claude", "mcp", "cortex.json")
	_ = os.MkdirAll(filepath.Dir(mcpFile), 0o755)
	_ = os.WriteFile(mcpFile, []byte(`{"command":"cortex","args":["mcp"]}`), 0o644)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	res, err := svc.Apply(Selection{Components: []model.ComponentID{model.ComponentCortex}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(mcpFile); !os.IsNotExist(err) {
		t.Errorf("cortex.json should have been removed: %v", err)
	}
	if len(res.RemovedFiles) == 0 {
		t.Errorf("expected RemovedFiles to be populated, got %+v", res)
	}
}

func TestService_Apply_RemovesCortexMCP_OpenCode(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentOpenCode)

	settings := filepath.Join(home, ".config", "opencode", "opencode.json")
	_ = os.MkdirAll(filepath.Dir(settings), 0o755)
	original := map[string]any{
		"mcp": map[string]any{
			"cortex":  map[string]any{"type": "local", "command": []string{"cortex", "mcp"}, "enabled": true},
			"context": map[string]any{"type": "local", "command": []string{"ctx"}},
		},
		"theme": "dark",
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	_ = os.WriteFile(settings, data, 0o644)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	if _, err := svc.Apply(Selection{Components: []model.ComponentID{model.ComponentCortex}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, _ := os.ReadFile(settings)
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("settings became invalid JSON: %v\n%s", err, string(got))
	}
	mcp, ok := parsed["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp key disappeared, but only the cortex sub-key should have been removed")
	}
	if _, has := mcp["cortex"]; has {
		t.Errorf("cortex key still present in opencode settings")
	}
	if _, has := mcp["context"]; !has {
		t.Errorf("sibling 'context' key was incorrectly removed")
	}
	if parsed["theme"] != "dark" {
		t.Errorf("user setting 'theme' was changed: %v", parsed["theme"])
	}
}

func TestService_Apply_DryRun(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	mcpFile := filepath.Join(home, ".claude", "mcp", "cortex.json")
	_ = os.MkdirAll(filepath.Dir(mcpFile), 0o755)
	_ = os.WriteFile(mcpFile, []byte(`{}`), 0o644)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	res, err := svc.Apply(Selection{Components: []model.ComponentID{model.ComponentCortex}, DryRun: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(mcpFile); err != nil {
		t.Errorf("dry-run should not have removed the file: %v", err)
	}
	if len(res.ChangedFiles) == 0 {
		t.Errorf("dry-run should report planned files, got %+v", res)
	}
}

func TestService_PathsToBackup(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	prompt := filepath.Join(home, ".claude", "CLAUDE.md")
	mcp := filepath.Join(home, ".claude", "mcp", "cortex.json")
	_ = os.MkdirAll(filepath.Dir(mcp), 0o755)
	_ = os.WriteFile(prompt, []byte("x"), 0o644)
	_ = os.WriteFile(mcp, []byte("{}"), 0o644)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	paths, err := svc.PathsToBackup(Selection{
		Components: []model.ComponentID{model.ComponentPersona, model.ComponentCortex},
	})
	if err != nil {
		t.Fatalf("PathsToBackup: %v", err)
	}
	wantSubstrings := []string{"CLAUDE.md", "cortex.json"}
	for _, want := range wantSubstrings {
		found := false
		for _, p := range paths {
			if strings.Contains(p, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PathsToBackup missing %q in %v", want, paths)
		}
	}
}

func TestService_Apply_AllRemovesState(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	if _, err := svc.Apply(Selection{All: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	st, err := state.Load(home)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if len(st.InstalledAgents) != 0 {
		t.Errorf("state should be empty after All-uninstall, got %v", st.InstalledAgents)
	}
}

func TestService_Apply_UnknownAgentRejected(t *testing.T) {
	home := t.TempDir()
	writeStateWithAgents(t, home, model.AgentClaudeCode)

	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	_, err := svc.Apply(Selection{Agents: []model.AgentID{"made-up-agent"}})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error should mention unknown agent, got %v", err)
	}
}

func TestService_Apply_NoStateNoAgents(t *testing.T) {
	home := t.TempDir()
	svc := NewServiceWithRegistry(home, newTestRegistry(t))
	_, err := svc.Apply(Selection{Components: []model.ComponentID{model.ComponentPersona}})
	if err == nil {
		t.Fatal("expected error when no state and no explicit agents")
	}
}
