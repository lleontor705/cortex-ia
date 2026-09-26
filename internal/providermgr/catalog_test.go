package providermgr

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func loadInIsolatedRoot(t *testing.T) (string, string, []Provider) {
	t.Helper()
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	providers, err := Load(home)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return stateRoot, home, providers
}

func writeCatalog(t *testing.T, stateRoot, body string) string {
	t.Helper()
	dir := filepath.Join(stateRoot, providersDirName)
	if err := os.MkdirAll(dir, providersDirMode); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	path := filepath.Join(dir, "nan.json")
	if err := os.WriteFile(path, []byte(body), catalogFileMode); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	return path
}

func TestREQ_PROV_001_SeedAndLoad(t *testing.T) {
	stateRoot, _, providers := loadInIsolatedRoot(t)
	if len(providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(providers))
	}
	nan := providers[0]
	if nan.ID() != "nan" || nan.Name() != "Nan" || nan.Schema() != SchemaV1 {
		t.Fatalf("provider identity = %q/%q/%q", nan.ID(), nan.Name(), nan.Schema())
	}
	if nan.NPM() != "@ai-sdk/openai-compatible" || nan.BaseURL() != "https://api.nan.builders/v1" {
		t.Fatalf("provider endpoints = %q/%q", nan.NPM(), nan.BaseURL())
	}
	if strings.TrimSpace(nan.EffortPosture()) == "" {
		t.Fatal("effort_posture must be declared")
	}

	info, err := os.Stat(filepath.Join(stateRoot, providersDirName, "nan.json"))
	if err != nil {
		t.Fatalf("seed not materialized: %v", err)
	}
	if info.Mode().Perm() != catalogFileMode {
		t.Fatalf("seed mode = %o, want %o", info.Mode().Perm(), catalogFileMode)
	}

	want := map[string][]string{
		"glm5.3":            {"low", "medium", "high", "max"},
		"glm5.3-flash":      {"low", "medium", "high", "max"},
		"deepseek-v4-flash": {},
		"qwen3.8-flash":     {},
		"mimo-v2.5":         {},
		"mimo-v2.6-flash":   {},
		"gemma4":            {"none", "minimal", "low", "medium", "high", "max"},
		"qwen3.6":           {"none", "minimal", "low", "medium", "high", "max"},
	}
	models := nan.Models()
	if len(models) != len(want) {
		t.Fatalf("models = %d, want %d", len(models), len(want))
	}
	for _, model := range models {
		expected, ok := want[model.ID()]
		if !ok {
			t.Fatalf("unexpected model %q", model.ID())
		}
		if got := model.Efforts(); !slices.Equal(got, expected) {
			t.Fatalf("%s efforts = %v, want %v", model.ID(), got, expected)
		}
		delete(want, model.ID())
	}
	if len(want) != 0 {
		t.Fatalf("missing models: %v", want)
	}

	premium, ok := nan.Model("glm5.3")
	if !ok || !premium.Premium() || premium.Context() != "1M" {
		t.Fatalf("glm5.3 premium/context = %v/%q", premium.Premium(), premium.Context())
	}
}

func TestREQ_PROV_001_CORTEXIAHOMEOverride(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	if _, err := Load(home); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, providersDirName, "nan.json")); err != nil {
		t.Fatalf("override seed missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cortex-ia")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("homeDir state root must stay untouched, stat err=%v", err)
	}
}

func TestREQ_PROV_001_ExistingCatalogIsNotReseeded(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	body := `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Custom Nan","npm":"@custom/nan","baseURL":"https://example.invalid/v1","models":[{"id":"custom","name":"Custom","efforts":["low"]}]}`
	path := writeCatalog(t, stateRoot, body)

	providers, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(after, []byte(body)) {
		t.Fatal("existing catalog was overwritten")
	}
	if len(providers) != 1 || providers[0].Name() != "Custom Nan" {
		t.Fatalf("existing catalog not loaded: %+v", providers)
	}
}

func TestREQ_PROV_001_MalformedCatalogRejectsTyped(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		field  string
		absent []string
	}{
		{"unsupported_schema", `{"schema":"cortex-ia/providers/v999-sentinel"}`, "schema", []string{"v999-sentinel"}},
		{"missing_models", `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1"}`, "models", nil},
		{"non_array_efforts", `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[{"id":"glm5.3","name":"GLM 5.3","efforts":"high"}]}`, "models[0].efforts", nil},
		{"unknown_top_level", `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","token_value":"SENTINEL-TOP","models":[{"id":"m","name":"M","efforts":[]}]}`, "token_value", []string{"SENTINEL-TOP"}},
		{"unknown_model_field", `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[{"id":"m","name":"M","efforts":[],"evil":"SENTINEL-MODEL"}]}`, "models[0].evil", []string{"SENTINEL-MODEL"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			t.Setenv("CORTEX_IA_HOME", stateRoot)
			path := writeCatalog(t, stateRoot, tc.body)

			_, err := Load(t.TempDir())
			if err == nil {
				t.Fatal("expected a typed rejection")
			}
			var invalid *InvalidCatalogError
			if !errors.As(err, &invalid) {
				t.Fatalf("error %v is not *InvalidCatalogError", err)
			}
			if invalid.Provider != "nan" {
				t.Fatalf("provider = %q, want nan", invalid.Provider)
			}
			if invalid.Field != tc.field {
				t.Fatalf("field = %q, want %q", invalid.Field, tc.field)
			}
			for _, leak := range tc.absent {
				if strings.Contains(err.Error(), leak) {
					t.Fatalf("error echoed raw value %q: %v", leak, err)
				}
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read back: %v", readErr)
			}
			if !bytes.Equal(after, []byte(tc.body)) {
				t.Fatal("malformed catalog was rewritten")
			}
		})
	}
}

func TestREQ_PROV_001_AccessorsReturnDeepCopies(t *testing.T) {
	_, _, providers := loadInIsolatedRoot(t)
	nan := providers[0]

	models := nan.Models()
	models[0] = ModelDef{id: "mutated"}
	if again := nan.Models(); len(again) != 8 || again[0].ID() != "glm5.3" {
		t.Fatalf("Models() aliases provider state: %+v", again)
	}

	efforts := nan.Models()[0].Efforts()
	efforts[0] = "mutated"
	if again := nan.Models()[0].Efforts(); again[0] != "low" {
		t.Fatalf("Efforts() aliases provider state: %v", again)
	}

	model, ok := nan.Model("glm5.3")
	if !ok {
		t.Fatal("glm5.3 not found")
	}
	model.efforts[0] = "mutated"
	again, _ := nan.Model("glm5.3")
	if again.Efforts()[0] != "low" {
		t.Fatalf("Model() aliases provider state: %v", again.Efforts())
	}

	reloaded, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded[0].Models()[0].ID() != "glm5.3" || reloaded[0].Models()[0].Efforts()[0] != "low" {
		t.Fatal("reloaded catalog mutated by an earlier caller")
	}

	direct := ModelDef{efforts: []string{"low"}}
	clonedEfforts := direct.Efforts()
	clonedEfforts[0] = "mutated"
	if direct.efforts[0] != "low" {
		t.Fatalf("Efforts() aliases the model vocabulary: %v", direct.efforts)
	}

	directProvider := Provider{models: []ModelDef{{id: "a"}}}
	clonedModels := directProvider.Models()
	clonedModels[0] = ModelDef{id: "b"}
	if directProvider.models[0].id != "a" {
		t.Fatalf("Models() aliases provider models: %+v", directProvider.models)
	}
}
