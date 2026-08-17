package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// writeConfig writes a config file into a fresh temporary directory and
// returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestConfig_SourcePathRecorded(t *testing.T) {
	t.Run("LoadFile records verbatim origin", func(t *testing.T) {
		path := writeConfig(t, "preset: full\n")
		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		if cfg.sourcePath != path {
			t.Fatalf("source path = %q, want %q", cfg.sourcePath, path)
		}
	})

	t.Run("FindProjectConfig records discovered origin", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, FileName)
		if err := os.WriteFile(path, []byte("preset: full\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, foundDir, err := FindProjectConfig(dir)
		if err != nil {
			t.Fatalf("FindProjectConfig: %v", err)
		}
		if cfg == nil || foundDir != dir {
			t.Fatalf("config not discovered in %s", dir)
		}
		if cfg.sourcePath != path {
			t.Fatalf("source path = %q, want %q", cfg.sourcePath, path)
		}
	})

	t.Run("origin is non-serialized provenance", func(t *testing.T) {
		path := writeConfig(t, "preset: full\n")
		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		// The origin must never round-trip through YAML: re-parsing the
		// same bytes leaves a fresh config without inherited provenance.
		fresh, err := LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		if fresh.sourcePath != cfg.sourcePath {
			t.Fatalf("origin drifted between loads: %q vs %q", fresh.sourcePath, cfg.sourcePath)
		}
		var parsed ProjectConfig
		parsed.sourcePath = "must-not-appear-in-yaml"
		if parsed.sourcePath == "" {
			t.Fatal("sourcePath not retained on in-memory config")
		}
	})
}

func TestConfig_EmptyLegacyBaseline(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "empty config", content: ""},
		{name: "comments only", content: "# cortex-ia project configuration\n"},
		{name: "legacy config without overlay", content: "preset: full\nstrict-tdd: true\nagents:\n  - opencode\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadFile(writeConfig(t, tt.content))
			if err != nil {
				t.Fatalf("LoadFile: %v", err)
			}
			var sel model.Selection
			if err := ApplyToSelection(cfg, &sel); err != nil {
				t.Fatalf("ApplyToSelection: %v", err)
			}
			if sel.Registry != nil {
				t.Fatalf("registry selection = %+v, want nil (no CustomSkills/DisabledComponents)", sel.Registry)
			}
		})
	}

	t.Run("legacy fields still merge", func(t *testing.T) {
		cfg, err := LoadFile(writeConfig(t, "preset: full\nstrict-tdd: true\n"))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		var sel model.Selection
		if err := ApplyToSelection(cfg, &sel); err != nil {
			t.Fatalf("ApplyToSelection: %v", err)
		}
		if sel.Preset != model.PresetFull {
			t.Fatalf("preset = %q, want %q", sel.Preset, model.PresetFull)
		}
		if !sel.StrictTDD {
			t.Fatal("strict-tdd not applied")
		}
		if sel.Registry != nil {
			t.Fatalf("registry selection = %+v, want nil", sel.Registry)
		}
	})
}

func TestConfig_Propagation(t *testing.T) {
	t.Run("copies paths and disables unchanged with origin", func(t *testing.T) {
		path := writeConfig(t, `preset: full
custom-skills:
  - path: ./skills/domain-validator
  - path: skills/another
disabled-components:
  - skills
`)
		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		var sel model.Selection
		if err := ApplyToSelection(cfg, &sel); err != nil {
			t.Fatalf("ApplyToSelection: %v", err)
		}
		if sel.Registry == nil {
			t.Fatal("RegistrySelection not propagated")
		}
		if sel.Registry.ConfigFile != path {
			t.Fatalf("ConfigFile = %q, want %q", sel.Registry.ConfigFile, path)
		}
		wantPaths := []string{"./skills/domain-validator", "skills/another"}
		if !reflect.DeepEqual(sel.Registry.CustomSkillPaths, wantPaths) {
			t.Fatalf("CustomSkillPaths = %v, want %v", sel.Registry.CustomSkillPaths, wantPaths)
		}
		wantDisabled := []model.ComponentID{"skills"}
		if !reflect.DeepEqual(sel.Registry.DisabledComponents, wantDisabled) {
			t.Fatalf("DisabledComponents = %v, want %v", sel.Registry.DisabledComponents, wantDisabled)
		}
	})

	t.Run("no validation of skill content or path existence", func(t *testing.T) {
		cfg, err := LoadFile(writeConfig(t, `custom-skills:
  - path: ./does/not/exist
`))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		var sel model.Selection
		if err := ApplyToSelection(cfg, &sel); err != nil {
			t.Fatalf("ApplyToSelection: %v", err)
		}
		if sel.Registry == nil {
			t.Fatal("RegistrySelection not propagated")
		}
		want := []string{"./does/not/exist"}
		if !reflect.DeepEqual(sel.Registry.CustomSkillPaths, want) {
			t.Fatalf("CustomSkillPaths = %v, want %v (transport must not validate)", sel.Registry.CustomSkillPaths, want)
		}
	})

	t.Run("disabled components only", func(t *testing.T) {
		cfg, err := LoadFile(writeConfig(t, `disabled-components:
  - skills
`))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		var sel model.Selection
		if err := ApplyToSelection(cfg, &sel); err != nil {
			t.Fatalf("ApplyToSelection: %v", err)
		}
		if sel.Registry == nil {
			t.Fatal("RegistrySelection not propagated")
		}
		if len(sel.Registry.CustomSkillPaths) != 0 {
			t.Fatalf("CustomSkillPaths = %v, want empty", sel.Registry.CustomSkillPaths)
		}
		if !reflect.DeepEqual(sel.Registry.DisabledComponents, []model.ComponentID{"skills"}) {
			t.Fatalf("DisabledComponents = %v, want [skills]", sel.Registry.DisabledComponents)
		}
	})

	t.Run("selection holds copies not aliases", func(t *testing.T) {
		cfg, err := LoadFile(writeConfig(t, `custom-skills:
  - path: ./skills/domain-validator
disabled-components:
  - skills
`))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		var sel model.Selection
		if err := ApplyToSelection(cfg, &sel); err != nil {
			t.Fatalf("ApplyToSelection: %v", err)
		}
		if sel.Registry == nil {
			t.Fatal("RegistrySelection not propagated")
		}
		cfg.CustomSkills[0].Path = "./mutated"
		cfg.DisabledComponents[0] = "mutated"
		if sel.Registry.CustomSkillPaths[0] != "./skills/domain-validator" {
			t.Fatalf("CustomSkillPaths aliased config: %v", sel.Registry.CustomSkillPaths)
		}
		if sel.Registry.DisabledComponents[0] != model.ComponentID("skills") {
			t.Fatalf("DisabledComponents aliased config: %v", sel.Registry.DisabledComponents)
		}
	})
}
