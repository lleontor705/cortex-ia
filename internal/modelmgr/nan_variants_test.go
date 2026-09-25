package modelmgr

import (
	"errors"
	"strings"
	"testing"
)

// nanVariantsForTest reads provider.nan.models.<model>.variants from the decoded
// config without asserting a shape, so a case can distinguish "absent" from an
// empty array.
func nanVariantsForTest(t *testing.T, path, model string) []any {
	t.Helper()
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	providers, _ := config[providerKey].(map[string]any)
	nanEntry, _ := providers[NanProvider].(map[string]any)
	models, _ := nanEntry[modelsKey].(map[string]any)
	entry, _ := models[model].(map[string]any)
	variants, _ := entry[variantsKey].([]any)
	return variants
}

func TestNanSetAuthorsSingleVariantAndPreservesComments(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, "{\n  // keep me\n  \"provider\": {\n    \"nan\": {\n      \"models\": {\n        \"glm5.3\": {}\n      }\n    }\n  }\n}\n")

	desired, err := ParseDesired("plan", "nan/glm5.3", "high")
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(home).Set(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AuthoredVariants) != 1 {
		t.Fatalf("authored %d variants, want exactly 1: %+v", len(result.AuthoredVariants), result.AuthoredVariants)
	}
	authored := result.AuthoredVariants[0]
	if authored.Provider != NanProvider || authored.Model != "glm5.3" || authored.ID != "high" {
		t.Fatalf("unexpected authored variant %+v", authored)
	}
	if content := readFile(t, path); !strings.Contains(content, "// keep me") {
		t.Fatalf("JSONC comment lost:\n%s", content)
	}
	variants := nanVariantsForTest(t, path, "glm5.3")
	if len(variants) != 1 {
		t.Fatalf("config lists %d variants, want 1: %+v", len(variants), variants)
	}
	object, ok := variants[0].(map[string]any)
	if !ok || len(object) != 2 {
		t.Fatalf("variant is not the exact {id, settings} shape: %+v", variants[0])
	}
	if object["id"] != "high" {
		t.Fatalf("variant id = %v, want high", object["id"])
	}
	settings, ok := object["settings"].(map[string]any)
	if !ok || len(settings) != 1 || settings["reasoningEffort"] != "high" {
		t.Fatalf("variant settings = %+v, want only reasoningEffort=high", object["settings"])
	}
}

func TestNanSetAppendsOnlyMissingVariant(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, `{"provider": {"nan": {"models": {"glm5.3": {"variants": [{"id": "low"}]}}}}}`)

	manager := New(home)
	missing, err := ParseDesired("plan", "nan/glm5.3", "max")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := manager.Set(missing, nil); err != nil || len(result.AuthoredVariants) != 1 {
		t.Fatalf("append: result=%+v err=%v", result, err)
	}
	variants := nanVariantsForTest(t, path, "glm5.3")
	if len(variants) != 2 {
		t.Fatalf("variants = %+v, want the existing entry plus one authored", variants)
	}
	if first, ok := variants[0].(map[string]any); !ok || first["id"] != "low" {
		t.Fatalf("existing variant dropped or reordered: %+v", variants)
	}

	present, err := ParseDesired("plan", "nan/glm5.3", "low")
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Set(present, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AuthoredVariants) != 0 {
		t.Fatalf("re-authoring an existing id appended variants: %+v", result.AuthoredVariants)
	}
	if len(nanVariantsForTest(t, path, "glm5.3")) != 2 {
		t.Fatal("already-present id changed the variant set")
	}
}

func TestNanSetRejectsUnlistedEffortWithoutWriting(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, "{\n  // keep me\n  \"provider\": {\"nan\": {\"models\": {\"glm5.3\": {}}}}\n}\n")
	before := readFile(t, path)

	desired, err := ParseDesired("plan", "nan/glm5.3", "ultra")
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(home).Set(desired, nil)
	var unsupported *UnsupportedEffortError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want UnsupportedEffortError, got %v", err)
	}
	if unsupported.Model != "glm5.3" || unsupported.Effort != "ultra" {
		t.Fatalf("error does not name the miss: %+v", unsupported)
	}
	if got := readFile(t, path); got != before {
		t.Fatalf("failing set mutated the config:\n%s", got)
	}
	if variants := nanVariantsForTest(t, path, "glm5.3"); len(variants) != 0 {
		t.Fatalf("unlisted effort authored %+v", variants)
	}
}

func TestDoctorNanVariantsFlagsUnlistedEffort(t *testing.T) {
	for name, tc := range map[string]struct {
		model    string
		severity Severity
		want     string
	}{
		"listed-effort":    {"nan/glm5.3#high", SeverityOK, "validated"},
		"unlisted-effort":  {"nan/glm5.3#ultra", SeverityWarning, "nan/glm5.3"},
		"empty-vocabulary": {"nan/deepseek-v4-flash#high", SeverityWarning, "(none)"},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			writeFile(t, configPath(home), `{"agents": {"plan": {"model": "`+tc.model+`"}}}`)
			report := New(home).Doctor(DoctorOptions{Template: cleanTemplate})
			finding, ok := findFinding(report, CheckNanVariants)
			if !ok {
				t.Fatalf("no %s finding: %+v", CheckNanVariants, report.Findings)
			}
			if finding.Severity != tc.severity {
				t.Fatalf("severity = %s, want %s (%s)", finding.Severity, tc.severity, finding.Message)
			}
			if !strings.Contains(finding.Message, tc.want) {
				t.Fatalf("message %q does not contain %q", finding.Message, tc.want)
			}
		})
	}
}
