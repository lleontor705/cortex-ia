package forgespec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestInjectForgeSpec_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := filepath.Join(tmpDir, ".claude", "mcp", "forgespec.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "npx" {
		t.Errorf("command = %v, want npx", m["command"])
	}
	args := m["args"].([]any)
	if len(args) != 2 || args[1] != "forgespec-mcp@1.4.0" {
		t.Errorf("args = %v", args)
	}
}

func TestTemplates_PinQualifiedForgeSpecVersion(t *testing.T) {
	tmpl := Templates()
	wantPackage := []byte("forgespec-mcp@1.4.0")

	jsonTemplates := map[string][]byte{
		"separate JSON": tmpl.SeparateFileJSON,
		"default JSON":  tmpl.DefaultOverlayJSON,
		"OpenCode":      tmpl.OpenCodeOverlayJSON,
		"VS Code":       tmpl.VSCodeOverlayJSON,
	}
	for name, content := range jsonTemplates {
		t.Run(name, func(t *testing.T) {
			if !bytes.Contains(content, wantPackage) {
				t.Fatalf("template does not pin %q:\n%s", wantPackage, content)
			}
		})
	}

	if len(tmpl.TOMLArgs) != 2 || tmpl.TOMLArgs[1] != string(wantPackage) {
		t.Fatalf("TOMLArgs = %v, want [-y %s]", tmpl.TOMLArgs, wantPackage)
	}
	if got, want := tmpl.Service.Versions.MaximumTested, ir.MustParseVersion("1.4.0"); got != want {
		t.Fatalf("contract maximum tested = %s, want qualified version %s", got, want)
	}
}

func TestTemplates_ForgeSpecName(t *testing.T) {
	tmpl := Templates()
	if tmpl.Name != "forgespec" {
		t.Errorf("Name = %s, want forgespec", tmpl.Name)
	}
}
