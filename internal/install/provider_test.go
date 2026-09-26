package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// providerInstalledHome builds an isolated temp home with CORTEX_IA_HOME
// pinned to it and a minimal agreed v2 installation, so provider ownership can
// be committed without ever touching the developer's real OpenCode state.
func providerInstalledHome(t *testing.T) (string, *Service) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	root := filepath.Join(home, ".config", "opencode")
	for _, dir := range []string{root, filepath.Join(home, ".cortex-ia")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	meta := state.MetadataV2{SchemaVersion: state.MetadataSchemaV2, OpencodeRoot: root,
		TransactionID: "txn-provider-test", UpdatedAt: time.Unix(1700000000, 0).UTC()}
	if err := state.SaveMetadataV2(home, meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	if err := state.SaveLockV2(home, state.NewLockFromMetadataV2(meta)); err != nil {
		t.Fatalf("save lock: %v", err)
	}
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return home, service
}

func providerConfigPath(home, name string) string {
	return filepath.Join(home, ".config", "opencode", name)
}

func providerWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func providerRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func providerDecode(t *testing.T, path string) map[string]any {
	t.Helper()
	object, err := filemerge.DecodeJSONObject(providerRead(t, path))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return object
}

func providerEntry(t *testing.T, config map[string]any, id string) map[string]any {
	t.Helper()
	entry, ok := providerConfigBlock(config, id)
	if !ok {
		t.Fatalf("config has no provider.%s", id)
	}
	return entry
}

func providerVariantIDs(t *testing.T, model map[string]any) []string {
	t.Helper()
	raw, ok := model["variants"].([]any)
	if !ok {
		t.Fatalf("model variants = %#v", model["variants"])
	}
	ids := make([]string, 0, len(raw))
	for _, item := range raw {
		object, _ := item.(map[string]any)
		settings, _ := object["settings"].(map[string]any)
		if object["id"] != settings["reasoningEffort"] {
			t.Fatalf("variant shape = %#v", object)
		}
		ids = append(ids, object["id"].(string))
	}
	return ids
}

func providerBackupCount(t *testing.T, home string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".cortex-ia", "backups"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read backups: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

const providerWinnerBody = "{\n  // keep this comment\n  \"theme\": \"dark\",\n  \"provider\": { \"other\": { \"npm\": \"other-pkg\" } }\n}\n"

func TestREQ_PROV_002_InstallWritesEveryCatalogModel(t *testing.T) {
	home, service := providerInstalledHome(t)
	const token = "sentinel-token-prov-002"
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, providerWinnerBody)

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: token})
	if err != nil {
		t.Fatalf("ProviderInstall: %v", err)
	}
	if receipt.Action != "installed" || receipt.Provider != "nan" || receipt.ConfigPath != winner || receipt.ReconciledTwinPath != "" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.BackupID == "" || receipt.RollbackCmd != "cortex-ia rollback "+receipt.BackupID {
		t.Fatalf("receipt disclosure = %+v", receipt)
	}
	raw := string(providerRead(t, winner))
	for _, want := range []string{"keep this comment", `"theme": "dark"`, "other-pkg"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("winner lost %q:\n%s", want, raw)
		}
	}
	entry := providerEntry(t, providerDecode(t, winner), "nan")
	options, _ := entry["options"].(map[string]any)
	if entry["npm"] != "@ai-sdk/openai-compatible" || entry["name"] != "Nan" ||
		options["baseURL"] != "https://api.nan.builders/v1" || options["apiKey"] != token {
		t.Fatalf("provider definition = %#v", entry)
	}
	models, ok := entry["models"].(map[string]any)
	if !ok {
		t.Fatalf("entry must carry a models object: %#v", entry)
	}
	for id, value := range models {
		if _, ok := value.(map[string]any); !ok {
			t.Fatalf("model %q must be a JSON object: %#v", id, value)
		}
	}

	record := state.LoadMetadataV2(home).Metadata.Providers[0]
	if record.Name != "nan" || record.ConfigPath != "opencode.jsonc" ||
		record.Ownership != state.OwnershipManaged || !strings.HasPrefix(record.SemanticDigest, "pvd1:") {
		t.Fatalf("ownership record = %+v", record)
	}
}

func TestREQ_PROV_003_TokenStaysOutAndRotationKeepsDigest(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{}\n")
	const first, rotated = "sentinel-token-first", "sentinel-token-rotated"

	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: first}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	digest, backups, before := state.LoadMetadataV2(home).Metadata.Providers[0].SemanticDigest, providerBackupCount(t, home), providerRead(t, winner)
	again, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: first})
	if err != nil {
		t.Fatalf("identical reinstall: %v", err)
	}
	if again.Action != "already-present" || again.BackupID != "" || providerBackupCount(t, home) != backups || string(providerRead(t, winner)) != string(before) {
		t.Fatalf("identical reinstall = %+v", again)
	}

	rotatedReceipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: rotated})
	if err != nil {
		t.Fatalf("rotated install: %v", err)
	}
	entry := providerEntry(t, providerDecode(t, winner), "nan")
	options, _ := entry["options"].(map[string]any)
	if rotatedReceipt.Action != "installed" || state.LoadMetadataV2(home).Metadata.Providers[0].SemanticDigest != digest ||
		options["apiKey"] != rotated {
		t.Fatalf("rotation = %+v entry = %#v", rotatedReceipt, entry)
	}
	stateBytes := string(providerRead(t, state.StatePath(home)))
	receiptJSON, _ := json.Marshal(rotatedReceipt)
	for _, leak := range []string{first, rotated} {
		if strings.Contains(stateBytes, leak) || strings.Contains(string(receiptJSON), leak) {
			t.Fatalf("token %q leaked into persisted artifacts", leak)
		}
	}
	if !strings.Contains(stateBytes, digest) {
		t.Fatal("state must carry the identity digest")
	}
}

func TestREQ_PROV_002_PreviewIsInert(t *testing.T) {
	home, service := providerInstalledHome(t)
	if _, err := service.ProviderCatalog(); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, providerWinnerBody)
	watched := []string{winner, state.StatePath(home), state.LockPath(home), filepath.Join(home, "providers", "nan.json")}
	before := map[string]time.Time{}
	for _, path := range watched {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		before[path] = info.ModTime()
	}
	configBefore := providerRead(t, winner)

	receipt, err := service.ProviderPreview(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-preview"})
	if err != nil {
		t.Fatalf("ProviderPreview: %v", err)
	}
	if !receipt.DryRun || receipt.Action != "installed" || receipt.BackupID != "" {
		t.Fatalf("preview receipt = %+v", receipt)
	}
	if !strings.Contains(strings.Join(receipt.Detail, "\n"), "backup first") {
		t.Fatalf("preview must disclose backup-first ordering: %v", receipt.Detail)
	}
	for _, path := range watched {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.ModTime().Equal(before[path]) {
			t.Fatalf("preview modified %s", path)
		}
	}
	if string(providerRead(t, winner)) != string(configBefore) || providerBackupCount(t, home) != 0 {
		t.Fatal("preview must not write config or create a backup")
	}
}

// providerEditedCatalog differs from the seed in name, both runtime keys,
// endpoint, model set, limits, modalities and effort vocabulary, so a
// projection assertion proves the installer reads the file rather than code.
const providerEditedCatalog = `{
  "schema": "cortex-ia/providers/v1",
  "id": "nan",
  "name": "Nan Edited",
  "npm": "@edited/npm",
  "package": "@edited/pkg",
  "baseURL": "https://edited.invalid/v2",
  "models": [
    {
      "id": "glm5.3",
      "name": "GLM 5.3 Edited",
      "efforts": ["low", "turbo"],
      "limit": { "context": 1024, "output": 512 },
      "modalities": { "input": ["text", "image"], "output": ["text"] }
    },
    { "id": "sentinel-model", "name": "Sentinel Model", "efforts": [] }
  ]
}`

func providerWriteCatalog(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nan.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
}

func TestREQ_PROV_007_CatalogPreviewAndEmissionTrackJSONEdits(t *testing.T) {
	home, service := providerInstalledHome(t)
	providerWriteCatalog(t, home, providerEditedCatalog)

	report, err := service.ProviderCatalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	entry := report.Providers[0]
	if entry.Name != "Nan Edited" || entry.NPM != "@edited/npm" || entry.Package != "@edited/pkg" || entry.BaseURL != "https://edited.invalid/v2" {
		t.Fatalf("catalog entry = %+v", entry)
	}
	if len(entry.Models) != 2 {
		t.Fatalf("catalog models = %d, want 2", len(entry.Models))
	}
	edited := entry.Models[0]
	if edited.ID != "glm5.3" || edited.Name != "GLM 5.3 Edited" {
		t.Fatalf("edited model identity = %+v", edited)
	}
	if edited.Limit == nil || edited.Limit.Context != 1024 || edited.Limit.Output != 512 {
		t.Fatalf("edited model limit = %+v", edited.Limit)
	}
	if edited.Modalities == nil || strings.Join(edited.Modalities.Input, ",") != "text,image" || strings.Join(edited.Modalities.Output, ",") != "text" {
		t.Fatalf("edited model modalities = %+v", edited.Modalities)
	}
	if strings.Join(edited.Efforts, ",") != "low,turbo" || len(edited.Variants) != 2 ||
		edited.Variants[0].ID != "low" || edited.Variants[1].ReasoningEffort != "turbo" {
		t.Fatalf("edited model efforts/variants = %+v / %+v", edited.Efforts, edited.Variants)
	}

	providerWrite(t, providerConfigPath(home, "opencode.jsonc"), "{}\n")
	singular, err := service.ProviderPreview(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-token"})
	if err != nil {
		t.Fatalf("singular preview: %v", err)
	}
	if singular.Entry == nil || singular.Entry.Name != "Nan Edited" || singular.Entry.BaseURL != "https://edited.invalid/v2" {
		t.Fatalf("preview entry = %+v", singular.Entry)
	}
	if singular.Container != providerKey || singular.RuntimeMember != npmKey || singular.RuntimeKey != "@edited/npm" {
		t.Fatalf("singular runtime = %q/%q/%q", singular.Container, singular.RuntimeMember, singular.RuntimeKey)
	}
	if singular.ModelsWritten != 2 || singular.VariantsWritten != 2 {
		t.Fatalf("singular counts = %d/%d", singular.ModelsWritten, singular.VariantsWritten)
	}

	providerWrite(t, providerConfigPath(home, "opencode.jsonc"), `{"providers":{"other":{"package":"other-pkg"}}}`)
	plural, err := service.ProviderPreview(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-token"})
	if err != nil {
		t.Fatalf("plural preview: %v", err)
	}
	if plural.Container != providersKey || plural.RuntimeMember != packageKey || plural.RuntimeKey != "@edited/pkg" {
		t.Fatalf("plural runtime = %q/%q/%q", plural.Container, plural.RuntimeMember, plural.RuntimeKey)
	}

	installed, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-token"})
	if err != nil {
		t.Fatalf("plural install: %v", err)
	}
	if installed.RuntimeMember != packageKey || installed.RuntimeKey != "@edited/pkg" {
		t.Fatalf("installed runtime = %q/%q", installed.RuntimeMember, installed.RuntimeKey)
	}
	emit := providerEntry(t, providerDecode(t, providerConfigPath(home, "opencode.jsonc")), "nan")
	settings, _ := emit["settings"].(map[string]any)
	if emit["package"] != "@edited/pkg" || emit["name"] != "Nan Edited" || settings["baseURL"] != "https://edited.invalid/v2" {
		t.Fatalf("emitted provider = %#v", emit)
	}
	models, _ := emit["models"].(map[string]any)
	glm, _ := models["glm5.3"].(map[string]any)
	limit, _ := glm["limit"].(map[string]any)
	if len(models) != 2 || limit["context"] != float64(1024) || limit["output"] != float64(512) {
		t.Fatalf("emitted models = %#v", models)
	}
	if got := strings.Join(providerVariantIDs(t, glm), ","); got != "low,turbo" {
		t.Fatalf("emitted variants = %s", got)
	}
}

func TestREQ_PROV_004_UnmanagedNonObjectBlockIsTypedRefusal(t *testing.T) {
	cases := []struct{ name, body string }{
		{"container_is_scalar", `{"provider": "sentinel-container"}`},
		{"entry_is_array", `{"provider": {"nan": ["sentinel-entry"]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, service := providerInstalledHome(t)
			winner := providerConfigPath(home, "opencode.jsonc")
			providerWrite(t, winner, tc.body)

			if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-token"}); !errors.Is(err, ErrProviderUnmanaged) {
				t.Fatalf("install err = %v, want ErrProviderUnmanaged", err)
			}
			if _, err := service.ProviderPreview(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-token"}); !errors.Is(err, ErrProviderUnmanaged) {
				t.Fatalf("preview err = %v, want ErrProviderUnmanaged", err)
			}
			if providerBackupCount(t, home) != 0 {
				t.Fatal("refusal must capture no backup")
			}
			if string(providerRead(t, winner)) != tc.body {
				t.Fatal("refusal must not mutate the winner config")
			}
		})
	}
}
