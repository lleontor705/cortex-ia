package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureTUIPlugin(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Fresh configuration
	tuiPath, err := ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed on fresh home: %v", err)
	}
	data, err := os.ReadFile(tuiPath)
	if err != nil {
		t.Fatalf("failed to read tui.jsonc: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, TUIPluginPath) {
		t.Errorf("expected %s in tui.jsonc, got:\n%s", TUIPluginPath, content)
	}

	// 2. Re-run idempotency
	_, err = ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin re-run failed: %v", err)
	}
	data2, _ := os.ReadFile(tuiPath)
	if strings.Count(string(data2), TUIPluginPath) != 1 {
		t.Errorf("expected exactly one plugin entry, got:\n%s", string(data2))
	}

	// 3. Migration from legacy plugin path
	legacyJSON := `{"plugin": ["./plugins/cortex-ia-tui.js", "other-plugin.js"]}`
	_ = os.WriteFile(filepath.Join(tempHome, ".config", "opencode", "tui.jsonc"), []byte(legacyJSON), 0644)
	_, err = ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed migrating legacy: %v", err)
	}
	data3, _ := os.ReadFile(tuiPath)
	content3 := string(data3)
	if strings.Contains(content3, LegacyTUIPluginPath) {
		t.Errorf("expected legacy plugin to be removed, got:\n%s", content3)
	}
	if !strings.Contains(content3, TUIPluginPath) || !strings.Contains(content3, "other-plugin.js") {
		t.Errorf("expected TUIPluginPath and other-plugin.js, got:\n%s", content3)
	}
}
