package install

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/providermgr"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func providerContainer(t *testing.T, config map[string]any, container string) map[string]any {
	t.Helper()
	object, ok := config[container].(map[string]any)
	if !ok {
		t.Fatalf("config has no %q container: %#v", container, config)
	}
	return object
}

func TestProviderInstall_MergesIntoSingularContainer(t *testing.T) {
	home, service := providerInstalledHome(t)
	const token = "sentinel-merge-token"
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, providerWinnerBody)

	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: token}); err != nil {
		t.Fatalf("install: %v", err)
	}
	config := providerDecode(t, winner)
	if _, present := config["providers"]; present {
		t.Fatal("singular install must not create a parallel plural section")
	}
	container := providerContainer(t, config, "provider")
	if _, present := container["other"]; !present {
		t.Fatal("unrelated provider lost during merge")
	}
	entry, _ := container["nan"].(map[string]any)
	for id, value := range providerModels(t, entry) {
		if _, ok := value.(map[string]any); !ok {
			t.Fatalf("model %q must be a JSON object: %#v", id, value)
		}
	}
	if raw := string(providerRead(t, winner)); !strings.Contains(raw, "keep this comment") {
		t.Fatalf("merge lost the JSONC comment:\n%s", raw)
	}
}

func TestProviderInstall_MergesIntoPluralContainerWithoutParallelSection(t *testing.T) {
	home, service := providerInstalledHome(t)
	const token = "sentinel-plural-token"
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{\n  // keep this comment\n  \"providers\": { \"other\": { \"package\": \"other-pkg\" } }\n}\n")

	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: token}); err != nil {
		t.Fatalf("install: %v", err)
	}
	config := providerDecode(t, winner)
	if _, present := config["provider"]; present {
		t.Fatal("plural install appended a parallel singular provider section")
	}
	container := providerContainer(t, config, "providers")
	if _, present := container["other"]; !present {
		t.Fatal("unrelated provider lost during merge")
	}
	entry, _ := container["nan"].(map[string]any)
	if entry["package"] != "@opencode/ai/providers/openai-compatible" {
		t.Fatalf("plural package = %#v", entry["package"])
	}
	if _, present := entry["npm"]; present {
		t.Fatal("plural entry must not carry the singular npm member")
	}
	settings, _ := entry["settings"].(map[string]any)
	if settings["baseURL"] != "https://api.nan.builders/v1" || settings["apiKey"] != token {
		t.Fatalf("plural settings = %#v", settings)
	}
	for id, value := range providerModels(t, entry) {
		model, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("model %q must be a JSON object: %#v", id, value)
		}
		if _, present := model["modalities"]; present {
			t.Fatalf("plural model %q must not carry the singular modalities member", id)
		}
		if _, ok := model["capabilities"].(map[string]any); !ok {
			t.Fatalf("plural model %q must carry a capabilities object: %#v", id, model)
		}
	}
}

func TestProviderInstall_ReinstallPreservesUserFieldsAndModels(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{}\n")
	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-first"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	config := providerDecode(t, winner)
	models := providerModels(t, providerEntry(t, config, "nan"))
	var targetID string
	var target map[string]any
	for id, value := range models {
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if _, hasVariants := object["variants"].([]any); hasVariants {
			targetID, target = id, object
			break
		}
	}
	if target == nil {
		t.Fatal("seed emitted no model with a variants array to exercise preservation")
	}
	originalVariants := len(target["variants"].([]any))
	target["temperature"] = 0.7
	target["variants"] = append(target["variants"].([]any), map[string]any{"id": "custom"})
	models["my-extra"] = map[string]any{"name": "Mine"}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("encode edited config: %v", err)
	}
	providerWrite(t, winner, string(encoded))

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-rotated"})
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if receipt.Action != "installed" {
		t.Fatalf("reinstall action = %q", receipt.Action)
	}
	merged := providerModels(t, providerEntry(t, providerDecode(t, winner), "nan"))
	if _, present := merged["my-extra"]; !present {
		t.Fatal("user-added model was deleted")
	}
	reinstalled, _ := merged[targetID].(map[string]any)
	if reinstalled["temperature"] != float64(0.7) {
		t.Fatalf("user per-model field lost: %#v", reinstalled)
	}
	variants, _ := reinstalled["variants"].([]any)
	ids := make([]string, 0, len(variants))
	for _, item := range variants {
		object, _ := item.(map[string]any)
		id, _ := object["id"].(string)
		ids = append(ids, id)
	}
	if len(ids) != originalVariants+1 || !strings.Contains(strings.Join(ids, ","), "custom") {
		t.Fatalf("user-added variant lost: %v", ids)
	}
}

func TestProviderInstall_AdoptsCatalogEquivalentBlockAndRecordsOwnership(t *testing.T) {
	home, service := providerInstalledHome(t)
	const token = "sentinel-equivalent"
	winner := providerConfigPath(home, "opencode.jsonc")

	providers, err := providermgr.Load(home)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	entry, _ := providerEntryForCatalog(providers[0], token, providerKey, nil)
	block, err := json.Marshal(map[string]any{providerKey: map[string]any{"nan": entry}})
	if err != nil {
		t.Fatalf("encode block: %v", err)
	}
	providerWrite(t, winner, string(block))
	before := providerRead(t, winner)

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: token})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if receipt.Action != "adopted" || receipt.BackupID == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if string(providerRead(t, winner)) != string(before) {
		t.Fatal("an already-equivalent block must not be rewritten")
	}
	if records := state.LoadMetadataV2(home).Metadata.Providers; len(records) != 1 || records[0].Name != "nan" {
		t.Fatalf("ownership not recorded: %+v", records)
	}
}

func TestProviderInstall_EmitsCatalogDeclaredLimitAndModalities(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{}\n")
	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-limits"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	catalog, err := providermgr.Load(home)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("no provider catalog was seeded")
	}
	models := providerModels(t, providerEntry(t, providerDecode(t, winner), "nan"))
	for _, declared := range catalog[0].Models() {
		emitted, ok := models[declared.ID()].(map[string]any)
		if !ok {
			t.Fatalf("catalog model %q was not emitted", declared.ID())
		}
		if limit, hasLimit := declared.Limit(); hasLimit {
			emittedLimit, _ := emitted["limit"].(map[string]any)
			if emittedLimit["context"] != float64(limit.Context) || emittedLimit["output"] != float64(limit.Output) {
				t.Fatalf("%s limit = %#v, want %d/%d", declared.ID(), emittedLimit, limit.Context, limit.Output)
			}
		}
		modalities, hasModalities := declared.Modalities()
		if !hasModalities {
			continue
		}
		emittedModalities, _ := emitted["modalities"].(map[string]any)
		if !reflect.DeepEqual(emittedModalities["input"], anyStrings(modalities.Input)) ||
			!reflect.DeepEqual(emittedModalities["output"], anyStrings(modalities.Output)) {
			t.Fatalf("%s modalities = %#v", declared.ID(), emittedModalities)
		}
	}
}

func anyStrings(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
