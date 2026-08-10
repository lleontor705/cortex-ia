package cortex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestInjectCortex_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := adapter.SettingsPath(tmpDir)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
}

func TestTemplates_CortexIsGoBinary(t *testing.T) {
	tmpl := Templates()
	if tmpl.Name != "cortex" {
		t.Errorf("Name = %s", tmpl.Name)
	}
	if tmpl.TOMLCommand != "cortex" {
		t.Errorf("TOMLCommand = %s, want cortex (Go binary, not npx)", tmpl.TOMLCommand)
	}
	if len(tmpl.TOMLArgs) != 2 || tmpl.TOMLArgs[0] != "mcp" || tmpl.TOMLArgs[1] != "--tools=agent" {
		t.Errorf("TOMLArgs = %v, want [mcp --tools=agent]", tmpl.TOMLArgs)
	}
}

func TestCortexCommandVectorPathRoundTripAllClients(t *testing.T) {
	const executable = `C:\Program Files\Cortex Tools\cortex.exe`
	for _, tc := range []struct {
		name    string
		adapter func() agents.Adapter
	}{
		{name: "opencode", adapter: func() agents.Adapter { return opencode.NewAdapter() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("APPDATA", filepath.Join(tmpDir, "App Data"))
			result, err := mcpinject.Inject(tmpDir, tc.adapter(), cortexTemplatesForExecutable(t, executable))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Files) != 1 {
				t.Fatalf("injected files = %v, want exactly one", result.Files)
			}
			content, err := os.ReadFile(result.Files[0])
			if err != nil {
				t.Fatal(err)
			}
			vector, err := decodedCortexCommandVector(tc.adapter().Agent(), content)
			if err != nil {
				t.Fatal(err)
			}
			if err := requireExactCortexCommandVector(vector, executable); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExactCortexCommandVectorRejectsDrift(t *testing.T) {
	const executable = `C:\Program Files\Cortex Tools\cortex.exe`
	for _, vector := range [][]string{
		{executable + " mcp --tools=agent"},
		{executable, "mcp", "--tools=agent", "--profile=default"},
		{executable, "mcp", "--tools=agent", "--model=gpt-5"},
		{"npx", "cortex", "mcp", "--tools=agent"},
		{"npm", "exec", "cortex", "mcp", "--tools=agent"},
	} {
		if err := requireExactCortexCommandVector(vector, executable); err == nil {
			t.Errorf("requireExactCortexCommandVector(%q) succeeded", vector)
		}
	}
}

func cortexTemplatesForExecutable(t *testing.T, executable string) mcpinject.ServerTemplates {
	t.Helper()
	template := Templates()
	encode := func(value any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	template.SeparateFileJSON = encode(map[string]any{"command": executable, "args": []string{"mcp", "--tools=agent"}})
	template.DefaultOverlayJSON = encode(map[string]any{"mcpServers": map[string]any{"cortex": map[string]any{"command": executable, "args": []string{"mcp", "--tools=agent"}}}})
	template.OpenCodeOverlayJSON = encode(map[string]any{"mcp": map[string]any{"cortex": map[string]any{"type": "local", "command": []string{executable, "mcp", "--tools=agent"}, "enabled": true}}})
	template.VSCodeOverlayJSON = encode(map[string]any{"servers": map[string]any{"cortex": map[string]any{"type": "stdio", "command": executable, "args": []string{"mcp", "--tools=agent"}}}})
	template.TOMLCommand = executable
	template.TOMLArgs = []string{"mcp", "--tools=agent"}
	return template
}

func decodedCortexCommandVector(agentID model.AgentID, content []byte) ([]string, error) {
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	server, ok := document["mcp"].(map[string]any)["cortex"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("cortex server config missing from settings")
	}
	if command, ok := server["command"].([]any); ok {
		return stringsFromAny(command)
	}
	args, err := stringsFromAny(server["args"].([]any))
	if err != nil {
		return nil, err
	}
	return append([]string{server["command"].(string)}, args...), nil
}

func stringsFromAny(values []any) ([]string, error) {
	result := make([]string, len(values))
	for i, value := range values {
		stringValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("command element %d = %T, want string", i, value)
		}
		result[i] = stringValue
	}
	return result, nil
}

func requireExactCortexCommandVector(vector []string, executable string) error {
	want := []string{executable, "mcp", "--tools=agent"}
	if len(vector) != len(want) {
		return fmt.Errorf("command vector = %q, want exactly %q", vector, want)
	}
	for i := range want {
		if vector[i] != want[i] {
			return fmt.Errorf("command vector = %q, want exactly %q", vector, want)
		}
	}
	return nil
}
