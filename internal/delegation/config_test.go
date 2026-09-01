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
