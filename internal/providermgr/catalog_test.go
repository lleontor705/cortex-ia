package providermgr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	// Windows does not represent POSIX permission bits, so the mode assertion is
	// only meaningful on platforms that do.
	if runtime.GOOS != "windows" && info.Mode().Perm() != catalogFileMode {
		t.Fatalf("seed mode = %o, want %o", info.Mode().Perm(), catalogFileMode)
	}

	models := nan.Models()
	if len(models) == 0 {
		t.Fatal("catalog must declare at least one model")
	}
	for _, model := range models {
		if strings.TrimSpace(model.ID()) == "" || strings.TrimSpace(model.Name()) == "" {
			t.Fatalf("model entry must carry an id and a name: %+v", model)
		}
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
		{"non_array_efforts", `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[{"id":"m","name":"M","efforts":"high"}]}`, "models[0].efforts", nil},
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
	stateRoot := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	body := `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[` +
		`{"id":"alpha","name":"Alpha","efforts":["low"]},` +
		`{"id":"beta","name":"Beta","efforts":[]}]}`
	writeCatalog(t, stateRoot, body)

	providers, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	nan := providers[0]

	models := nan.Models()
	models[0] = ModelDef{id: "mutated"}
	if again := nan.Models(); len(again) != 2 || again[0].ID() != "alpha" {
		t.Fatalf("Models() aliases provider state: %+v", again)
	}

	efforts := nan.Models()[0].Efforts()
	efforts[0] = "mutated"
	if again := nan.Models()[0].Efforts(); again[0] != "low" {
		t.Fatalf("Efforts() aliases provider state: %v", again)
	}

	model, ok := nan.Model("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	model.efforts[0] = "mutated"
	again, _ := nan.Model("alpha")
	if again.Efforts()[0] != "low" {
		t.Fatalf("Model() aliases provider state: %v", again.Efforts())
	}

	reloaded, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded[0].Models()[0].ID() != "alpha" || reloaded[0].Models()[0].Efforts()[0] != "low" {
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

func TestREQ_PROV_001_MalformedLimitAndModalitiesRejectTyped(t *testing.T) {
	cases := []struct {
		name   string
		member string
		field  string
	}{
		{"limit_not_object", `"limit":5`, "models[0].limit"},
		{"limit_missing_output", `"limit":{"context":1}`, "models[0].limit.output"},
		{"limit_unknown_field", `"limit":{"context":1,"output":2,"evil":true}`, "models[0].limit.evil"},
		{"limit_non_integer", `"limit":{"context":1.5,"output":2}`, "models[0].limit.context"},
		{"modalities_not_object", `"modalities":[]`, "models[0].modalities"},
		{"modalities_empty_input", `"modalities":{"input":[],"output":["text"]}`, "models[0].modalities.input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			t.Setenv("CORTEX_IA_HOME", stateRoot)
			body := fmt.Sprintf(`{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[{"id":"m","name":"M","efforts":[],%s}]}`, tc.member)
			path := writeCatalog(t, stateRoot, body)

			_, err := Load(t.TempDir())
			var invalid *InvalidCatalogError
			if !errors.As(err, &invalid) {
				t.Fatalf("error %v is not *InvalidCatalogError", err)
			}
			if invalid.Field != tc.field {
				t.Fatalf("field = %q, want %q", invalid.Field, tc.field)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read back: %v", readErr)
			}
			if !bytes.Equal(after, []byte(body)) {
				t.Fatal("malformed catalog was rewritten")
			}
		})
	}
}

func TestREQ_PROV_001_OptionalLimitAndModalitiesAccepted(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	body := `{"schema":"cortex-ia/providers/v1","id":"nan","name":"Nan","npm":"pkg","baseURL":"https://example.invalid/v1","models":[` +
		`{"id":"m","name":"M","efforts":[],"limit":{"context":1000,"output":500},"modalities":{"input":["text"],"output":["text"]}},` +
		`{"id":"bare","name":"Bare","efforts":[]}]}`
	writeCatalog(t, stateRoot, body)

	providers, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	withLimit, _ := providers[0].Model("m")
	if limit, ok := withLimit.Limit(); !ok || limit.Context != 1000 || limit.Output != 500 {
		t.Fatalf("m limit = %+v (present=%v)", limit, ok)
	}
	bare, _ := providers[0].Model("bare")
	if _, ok := bare.Limit(); ok {
		t.Fatal("bare model must report no limit")
	}
	if _, ok := bare.Modalities(); ok {
		t.Fatal("bare model must report no modalities")
	}
}
