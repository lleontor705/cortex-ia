package delegation

import (
	"os"
	"testing"
)

func TestNormalConfig(t *testing.T) {
	cfg := NormalConfig()
	if cfg.DelegationEnabled {
		t.Error("expected delegation_enabled to be false for NormalConfig")
	}
	if cfg.UseHerdr {
		t.Error("expected use_herdr to be false for NormalConfig")
	}
	for role, rCfg := range cfg.Roles {
		if rCfg.Delegate {
			t.Errorf("expected role %s to have delegate=false", role)
		}
		if rCfg.CLI != "native" {
			t.Errorf("expected role %s to have cli=native", role)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cortex-delegation-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := DefaultDelegationConfig(true)
	if err := Save(tempDir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !loaded.DelegationEnabled {
		t.Error("expected loaded config to have delegation_enabled=true")
	}
	if !loaded.UseHerdr {
		t.Error("expected loaded config to have use_herdr=true")
	}
	if loaded.Roles["implement"].CLI != "agy" {
		t.Errorf("expected implement CLI to be agy, got %s", loaded.Roles["implement"].CLI)
	}
	if loaded.Roles["reviewer"].CLI != "agy" {
		t.Errorf("expected reviewer CLI to be agy, got %s", loaded.Roles["reviewer"].CLI)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cortex-delegation-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load non-existent should not error, got: %v", err)
	}
	if loaded.DelegationEnabled {
		t.Error("expected fallback NormalConfig to have delegation_enabled=false")
	}
}

func TestConfigValidationModelAndEffort(t *testing.T) {
	cfg := DefaultDelegationConfig(true)
	role := cfg.Roles["implement"]
	role.Model = "gemini-3.8-flash-high"
	role.Effort = "high"
	cfg.Roles["implement"] = role

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	role.Effort = "invalid-effort"
	cfg.Roles["implement"] = role
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid effort, got nil")
	}
}

func TestConfigValidationTimeoutUnboundedAndTerminal(t *testing.T) {
	cfg := DefaultDelegationConfig(true)
	cfg.HerdrSettings.TimeoutSeconds = 0
	cfg.TerminalSettings = TerminalSettings{
		Launcher:        "auto",
		KeepOpenOnError: true,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected timeout 0 to be valid, got: %v", err)
	}

	cfg.HerdrSettings.TimeoutSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative timeout, got nil")
	}

	cfg.HerdrSettings.TimeoutSeconds = 86401
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for timeout > 86400, got nil")
	}

	cfg.HerdrSettings.TimeoutSeconds = 900
	cfg.HerdrSettings.Presentation = "tab"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected presentation 'tab' to be valid, got: %v", err)
	}

	cfg.HerdrSettings.Presentation = "split"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected presentation 'split' to be valid, got: %v", err)
	}

	cfg.HerdrSettings.Presentation = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for presentation 'invalid', got nil")
	}
}
