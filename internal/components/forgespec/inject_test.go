package forgespec

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestInjectForgeSpec_OpenCode(t *testing.T) {
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

func TestTemplates_PinQualifiedForgeSpecVersion(t *testing.T) {
	tmpl := Templates()
	wantPackage := []byte("forgespec-mcp@1.4.0")

	jsonTemplates := map[string][]byte{
		"separate JSON": tmpl.SeparateFileJSON,
		"default JSON":  tmpl.DefaultOverlayJSON,
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
	var openCode map[string]any
	if err := json.Unmarshal(tmpl.OpenCodeOverlayJSON, &openCode); err != nil {
		t.Fatal(err)
	}
	command := openCode["mcp"].(map[string]any)["forgespec"].(map[string]any)["command"].([]any)
	if len(command) != 1 || command[0] != OpenCodeCommand {
		t.Fatalf("OpenCode command = %v, want direct qualified wrapper %q", command, OpenCodeCommand)
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
