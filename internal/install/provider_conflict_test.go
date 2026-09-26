package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func providerModels(t *testing.T, entry map[string]any) map[string]any {
	t.Helper()
	models, ok := entry["models"].(map[string]any)
	if !ok {
		t.Fatalf("provider entry has no models object: %#v", entry)
	}
	return models
}

func providerManifest(t *testing.T, home, id string) backup.Manifest {
	t.Helper()
	var manifest backup.Manifest
	raw := providerRead(t, filepath.Join(home, ".cortex-ia", "backups", id, backup.ManifestFilename))
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}

func providerManifestSnapshot(t *testing.T, manifest backup.Manifest, original string) []byte {
	t.Helper()
	for _, entry := range manifest.Entries {
		if entry.OriginalPath == original {
			if !entry.Existed {
				t.Fatalf("backup entry %s reports Existed=false", original)
			}
			return providerRead(t, entry.SnapshotPath)
		}
	}
	t.Fatalf("backup manifest has no entry for %s", original)
	return nil
}

func TestREQ_PROV_004_TwinCopyReconciledBackupFirst(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	twin := providerConfigPath(home, "opencode.json")
	providerWrite(t, winner, "{\n  \"winnerNote\": \"keep\"\n}\n")
	providerWrite(t, twin, "{\n  \"twinNote\": \"keep\",\n  \"provider\": { \"nan\": { \"npm\": \"stale\" } }\n}\n")
	preWinner, preTwin := providerRead(t, winner), providerRead(t, twin)

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-twin"})
	if err != nil {
		t.Fatalf("ProviderInstall: %v", err)
	}
	if receipt.Action != "installed" || receipt.ConfigPath != winner || receipt.ReconciledTwinPath != twin {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.BackupID == "" {
		t.Fatal("reconciliation must disclose its backup id")
	}
	if _, present := providerConfigBlock(providerDecode(t, twin), "nan"); present {
		t.Fatal("stale twin block survived")
	}
	if providerDecode(t, twin)["twinNote"] != "keep" || providerDecode(t, winner)["winnerNote"] != "keep" {
		t.Fatal("unrelated members were not preserved")
	}
	if won := providerEntry(t, providerDecode(t, winner), "nan"); len(providerModels(t, won)) != 8 {
		t.Fatal("winner was not materialized with the full catalog")
	}

	manifest := providerManifest(t, home, receipt.BackupID)
	if string(providerManifestSnapshot(t, manifest, winner)) != string(preWinner) ||
		string(providerManifestSnapshot(t, manifest, twin)) != string(preTwin) {
		t.Fatal("backup must protect both pre-edit config files")
	}
}

func TestREQ_PROV_004_DivergentTwinsConverge(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{}\n")
	if _, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-first"}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	twin := providerConfigPath(home, "opencode.json")
	providerWrite(t, twin, "{\"provider\":{\"nan\":{\"npm\":\"divergent\"}}}")

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-second"})
	if err != nil {
		t.Fatalf("reconciling install: %v", err)
	}
	if receipt.ReconciledTwinPath != twin || receipt.Action != "installed" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, present := providerConfigBlock(providerDecode(t, twin), "nan"); present {
		t.Fatal("divergent twin block survived")
	}
	if len(providerModels(t, providerEntry(t, providerDecode(t, winner), "nan"))) != 8 {
		t.Fatal("winner did not converge to the catalog")
	}
}

func TestREQ_PROV_004_DryRunDisclosesReconciliationWithoutWrites(t *testing.T) {
	home, service := providerInstalledHome(t)
	if _, err := service.ProviderCatalog(); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	winner := providerConfigPath(home, "opencode.jsonc")
	twin := providerConfigPath(home, "opencode.json")
	providerWrite(t, winner, "{}\n")
	providerWrite(t, twin, "{\"provider\":{\"nan\":{\"npm\":\"stale\"}}}")
	watched := []string{winner, twin, state.StatePath(home), state.LockPath(home), filepath.Join(home, "providers", "nan.json")}
	before := map[string]time.Time{}
	for _, path := range watched {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		before[path] = info.ModTime()
	}
	preTwin := providerRead(t, twin)

	receipt, err := service.ProviderPreview(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-dry"})
	if err != nil {
		t.Fatalf("ProviderPreview: %v", err)
	}
	if !receipt.DryRun || receipt.ReconciledTwinPath != twin {
		t.Fatalf("dry-run receipt = %+v", receipt)
	}
	disclosure := strings.Join(receipt.Detail, "\n")
	for _, want := range []string{"backup first", "twin cleanup", "winner write"} {
		if !strings.Contains(disclosure, want) {
			t.Fatalf("dry-run must disclose %q: %v", want, receipt.Detail)
		}
	}
	for _, path := range watched {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.ModTime().Equal(before[path]) {
			t.Fatalf("dry-run modified %s", path)
		}
	}
	if string(providerRead(t, twin)) != string(preTwin) || providerBackupCount(t, home) != 0 {
		t.Fatal("dry-run must not edit the twin or create a backup")
	}
}

func TestREQ_PROV_004_FaultInjectedFailuresRestoreBothFiles(t *testing.T) {
	for _, failure := range []string{"twin-edit", "winner-overlay"} {
		t.Run(failure, func(t *testing.T) {
			home, service := providerInstalledHome(t)
			winner := providerConfigPath(home, "opencode.jsonc")
			twin := providerConfigPath(home, "opencode.json")
			providerWrite(t, winner, "{}\n")
			providerWrite(t, twin, "{\"note\":\"keep\",\"provider\":{\"nan\":{\"npm\":\"stale\"}}}")
			preWinner, preTwin := providerRead(t, winner), providerRead(t, twin)
			preState := providerRead(t, state.StatePath(home))
			injected := errors.New("injected " + failure + " failure")
			fault := func(string, filemerge.JSONMutation) (filemerge.JSONFileResult, error) {
				return filemerge.JSONFileResult{}, injected
			}
			winnerCalled := false
			if failure == "twin-edit" {
				twinOriginal, winnerOriginal := providerTwinMutate, providerWinnerMutate
				providerTwinMutate = fault
				providerWinnerMutate = func(path string, mutation filemerge.JSONMutation) (filemerge.JSONFileResult, error) {
					winnerCalled = true
					return winnerOriginal(path, mutation)
				}
				t.Cleanup(func() { providerTwinMutate, providerWinnerMutate = twinOriginal, winnerOriginal })
			} else {
				original := providerWinnerMutate
				providerWinnerMutate = fault
				t.Cleanup(func() { providerWinnerMutate = original })
			}

			_, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-fault"})
			if !errors.Is(err, injected) {
				t.Fatalf("err = %v, want injected failure", err)
			}
			if winnerCalled {
				t.Fatal("winner write must not run before twin cleanup completes")
			}
			if string(providerRead(t, winner)) != string(preWinner) || string(providerRead(t, twin)) != string(preTwin) {
				t.Fatal("both config files must be restored byte-identical")
			}
			if string(providerRead(t, state.StatePath(home))) != string(preState) {
				t.Fatal("state.json must remain uncommitted")
			}
		})
	}
}

func TestREQ_PROV_004_UnmanagedWinnerRefuses(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.jsonc")
	providerWrite(t, winner, "{\"provider\":{\"nan\":{\"npm\":\"user-owned\"}}}")
	before := providerRead(t, winner)
	preState := providerRead(t, state.StatePath(home))

	_, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-unmanaged"})
	if !errors.Is(err, ErrProviderUnmanaged) {
		t.Fatalf("err = %v, want ErrProviderUnmanaged", err)
	}
	if providerBackupCount(t, home) != 0 {
		t.Fatal("unmanaged refusal must capture zero backups")
	}
	if string(providerRead(t, winner)) != string(before) || string(providerRead(t, state.StatePath(home))) != string(preState) {
		t.Fatal("unmanaged refusal must mutate nothing")
	}
}

func TestREQ_PROV_004_SoloTwinNoReconciliation(t *testing.T) {
	home, service := providerInstalledHome(t)
	winner := providerConfigPath(home, "opencode.json")
	providerWrite(t, winner, "{\"note\":\"keep\"}\n")

	receipt, err := service.ProviderInstall(ProviderInstallOptions{ProviderID: "nan", Token: "sentinel-solo"})
	if err != nil {
		t.Fatalf("ProviderInstall: %v", err)
	}
	if receipt.ConfigPath != winner || receipt.ReconciledTwinPath != "" || receipt.Action != "installed" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, present := providerConfigBlock(providerDecode(t, winner), "nan"); !present {
		t.Fatal("solo winner was not materialized")
	}
	if providerDecode(t, winner)["note"] != "keep" {
		t.Fatal("unrelated member was not preserved")
	}
	if _, err := os.Stat(providerConfigPath(home, "opencode.jsonc")); !os.IsNotExist(err) {
		t.Fatal("no twin existed and none may be created")
	}
}
