package gga

import (
	"os"
	"strings"
	"testing"
)

func TestIsValidProvider(t *testing.T) {
	for _, valid := range SupportedProviders {
		if !IsValidProvider(valid) {
			t.Errorf("IsValidProvider(%q) = false, want true", valid)
		}
	}
	for _, invalid := range []string{"", "azure", "claude2", " anthropic"} {
		if IsValidProvider(invalid) {
			t.Errorf("IsValidProvider(%q) = true, want false", invalid)
		}
	}
}

func TestBuildConfig_Anthropic(t *testing.T) {
	cfg := string(BuildConfig("anthropic"))
	if !strings.Contains(cfg, `PROVIDER="anthropic"`) {
		t.Errorf("missing PROVIDER line: %s", cfg)
	}
	if !strings.Contains(cfg, `MODEL="claude`) {
		t.Errorf("anthropic config should contain a default Claude MODEL: %s", cfg)
	}
	if strings.Contains(cfg, "API_BASE") {
		t.Errorf("anthropic config should NOT contain API_BASE: %s", cfg)
	}
}

func TestBuildConfig_Ollama(t *testing.T) {
	cfg := string(BuildConfig("ollama"))
	if !strings.Contains(cfg, `MODEL="llama3.2"`) {
		t.Errorf("ollama config should default MODEL to llama3.2: %s", cfg)
	}
	if !strings.Contains(cfg, `API_BASE="http://localhost:11434"`) {
		t.Errorf("ollama config should contain API_BASE: %s", cfg)
	}
}

func TestBuildConfig_AgentRouted_NoModel(t *testing.T) {
	cfg := string(BuildConfig("claude"))
	if strings.Contains(cfg, "MODEL=") {
		t.Errorf("agent-routed providers should not emit MODEL: %s", cfg)
	}
}

func TestSetProvider_WritesFile(t *testing.T) {
	home := t.TempDir()
	changed, err := SetProvider(home, "anthropic")
	if err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if !changed {
		t.Error("expected changed=true on first write")
	}

	got, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), `PROVIDER="anthropic"`) {
		t.Errorf("file content missing PROVIDER line: %s", string(got))
	}
}

func TestSetProvider_Idempotent(t *testing.T) {
	home := t.TempDir()
	if _, err := SetProvider(home, "openai"); err != nil {
		t.Fatalf("SetProvider 1: %v", err)
	}
	changed, err := SetProvider(home, "openai")
	if err != nil {
		t.Fatalf("SetProvider 2: %v", err)
	}
	if changed {
		t.Error("expected changed=false on second identical write")
	}
}

func TestSetProvider_InvalidProvider(t *testing.T) {
	home := t.TempDir()
	_, err := SetProvider(home, "azure")
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("error message lacking context: %v", err)
	}
}
