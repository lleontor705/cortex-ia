package modelmgr

import (
	"strings"
	"testing"
)

func TestNanSetAuthorsVariantsUnderPluralContainer(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, "{\n  // keep me\n  \"providers\": {\n    \"nan\": {\n      \"models\": {\n        \"glm5.3\": {}\n      }\n    }\n  }\n}\n")

	desired, err := ParseDesired("plan", "nan/glm5.3", "high")
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(home).Set(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AuthoredVariants) != 1 {
		t.Fatalf("authored %d variants, want 1: %+v", len(result.AuthoredVariants), result.AuthoredVariants)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, present := config[providerKey]; present {
		t.Fatal("plural authoring appended a parallel singular provider section")
	}
	container, _ := config[providersKey].(map[string]any)
	entry, _ := container[NanProvider].(map[string]any)
	models, _ := entry[modelsKey].(map[string]any)
	glm, _ := models["glm5.3"].(map[string]any)
	variants, _ := glm[variantsKey].([]any)
	if len(variants) != 1 {
		t.Fatalf("plural variants = %#v", variants)
	}
	if object, _ := variants[0].(map[string]any); object["id"] != "high" {
		t.Fatalf("plural variant shape = %#v", variants[0])
	}
	if content := readFile(t, path); !strings.Contains(content, "// keep me") {
		t.Fatalf("JSONC comment lost:\n%s", content)
	}
}

func TestNanSetDoesNotCreateProviderSectionWhenAbsent(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, `{"providers":{"other":{"package":"other-pkg"}}}`)

	desired, err := ParseDesired("plan", "nan/glm5.3", "high")
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(home).Set(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AuthoredVariants) != 0 {
		t.Fatalf("authored variants without a provider model entry: %+v", result.AuthoredVariants)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, present := config[providerKey]; present {
		t.Fatal("set created a parallel singular provider section")
	}
	container, _ := config[providersKey].(map[string]any)
	if _, present := container[NanProvider]; present {
		t.Fatal("set created a provider definition it cannot trust")
	}
}
