package pipeline

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/v2/internal/assets"
)

// configMappingDest returns the home-relative destination of the OpenCode
// settings template for a fresh plan, so a test can plant a user config there.
func configMappingDest(t *testing.T, home string) string {
	t.Helper()
	plan, err := PlanInstall(engineRequest(home))
	if err != nil {
		t.Fatalf("plan install: %v", err)
	}
	for _, mapping := range plan.mappings {
		if mapping.Kind == assets.KindConfig {
			return mapping.Dest
		}
	}
	t.Fatal("plan declares no settings template")
	return ""
}

// A rewrite of a user's private settings file keeps its 0600 mode: provider
// and MCP keys in it must never become world-readable.
func TestInstall_PreservesPrivateConfigMode(t *testing.T) {
	home := engineHome(t)
	dest := configMappingDest(t, home)
	abs := engineJoin(home, dest)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	userConfig := `{"provider":{"acme":{"apiKey":"sk-test-secret"}}}`
	if err := os.WriteFile(abs, []byte(userConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := InstallV2(engineRequest(home)); err != nil {
		t.Fatalf("install over private config: %v", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private config mode = %v after merge, want 0600", info.Mode().Perm())
	}
	merged := engineAssertRegular(t, abs)
	if merged == userConfig {
		t.Fatal("settings template was not merged; the rewrite path was not exercised")
	}
	if !strings.Contains(merged, "sk-test-secret") {
		t.Fatal("merge dropped the user's provider key")
	}
}

// Backup directories are private, and the copy of a 0600 file keeps 0600.
func TestInstall_BackupTreeIsPrivate(t *testing.T) {
	home := engineHome(t)
	abs := engineJoin(home, configMappingDest(t, home))
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(`{"provider":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, receipt, err := InstallV2(engineRequest(home))
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	root := engineJoin(home, ".cortex-ia/backups")
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() && info.Mode().Perm() != 0o700 {
			t.Errorf("backup directory %s mode = %v, want 0700", path, info.Mode().Perm())
		}
		if !d.IsDir() && filepath.Base(path) == "opencode.jsonc" && info.Mode().Perm() != 0o600 {
			t.Errorf("backup copy %s mode = %v, want 0600", path, info.Mode().Perm())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if receipt.BackupID == "" {
		t.Fatal("expected a backup for the pre-existing config")
	}
}
