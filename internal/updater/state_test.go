package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testBinaryName() string {
	if runtime.GOOS == "windows" {
		return "cortex-ia.exe"
	}
	return "cortex-ia"
}

func seed(t *testing.T, home string, state UpdateState) {
	t.Helper()
	if err := SaveUpdateStateAtomic(home, state); err != nil {
		t.Fatalf("SaveUpdateStateAtomic: %v", err)
	}
}

func TestUpdateStateRoundTripAtomic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	resolved, err := DefaultStateHome()
	if err != nil || resolved != filepath.Clean(home) || StatePath("") != filepath.Join(home, "update-state.json") {
		t.Fatalf("CORTEX_IA_HOME override not honored: %q err=%v", resolved, err)
	}

	checkedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	binaryPath := filepath.Join(home, "bin", "cortex-ia")
	seed(t, home, UpdateState{
		LastCheckedAt:     checkedAt,
		Available:         "v0.5.0",
		AvailableDigest:   "9f2c",
		AppliedFloor:      "v0.4.50",
		ManagedPath:       binaryPath,
		InstallCandidates: []string{binaryPath, binaryPath, ""},
	})
	if leftovers, _ := filepath.Glob(filepath.Join(home, ".cortex-ia-*.tmp")); len(leftovers) != 0 {
		t.Fatalf("partial state file observable: %v", leftovers)
	}

	got, err := LoadUpdateState(home)
	if err != nil {
		t.Fatalf("LoadUpdateState: %v", err)
	}
	if got.SchemaVersion != UpdateStateSchemaVersion || !got.LastCheckedAt.Equal(checkedAt) ||
		got.AppliedFloor != "v0.4.50" || got.ManagedPath != binaryPath {
		t.Fatalf("schema/time/floor/path not preserved: %+v", got)
	}
	if got.Available != "v0.5.0" || got.AvailableDigest != "9f2c" || !got.UpdateAvailable() || len(got.InstallCandidates) != 1 {
		t.Fatalf("available/candidates not preserved: %+v", got)
	}
}

func TestUpdateStateDegradesOnBadInput(t *testing.T) {
	state, err := LoadUpdateState(t.TempDir())
	if err != nil || state.SchemaVersion != UpdateStateSchemaVersion || state.AppliedFloor != "" || state.UpdateAvailable() {
		t.Fatalf("missing state must load empty: %+v err=%v", state, err)
	}

	for name, content := range map[string]string{
		"corrupt json":  "{not-json",
		"empty file":    "   \n",
		"future schema": `{"schema_version": 99}`,
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "update-state.json"), []byte(content), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			state, err := LoadUpdateState(home)
			if !errors.Is(err, ErrCorruptUpdateState) {
				t.Fatalf("want ErrCorruptUpdateState, got %v", err)
			}
			if state.SchemaVersion != UpdateStateSchemaVersion || state.AppliedFloor != "" {
				t.Fatalf("must degrade to empty: %+v", state)
			}
		})
	}
}

func TestUpdateStateInstallCandidateDetection(t *testing.T) {
	home := t.TempDir()
	localAppData := filepath.Join(home, "localappdata")
	gopath := filepath.Join(home, "gopath")
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("GOPATH", gopath)

	bin := testBinaryName()
	running := filepath.Join(home, "running", bin)
	localBin := filepath.Join(localAppData, "Programs", "cortex-ia", "bin", bin)
	goBin := filepath.Join(gopath, "bin", bin)
	for _, path := range []string{running, localBin, goBin} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
			t.Fatalf("seed binary: %v", err)
		}
	}

	candidates := DetectInstallCandidates(running)
	warning := DualInstallWarning(candidates)
	if len(candidates) != 3 || !strings.Contains(warning, localBin) || !strings.Contains(warning, goBin) || DualInstallWarning(candidates[:1]) != "" {
		t.Fatalf("want both known installs recorded and no single-install warning: %v warning=%q", candidates, warning)
	}

	seed(t, home, UpdateState{InstallCandidates: candidates})
	if state, err := LoadUpdateState(home); err != nil || len(state.InstallCandidates) != 3 {
		t.Fatalf("candidates not persisted: %+v err=%v", state, err)
	}
}

func TestAppliedFloorRejectsReplayAcrossSessions(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	defer SetTrustedKeysForTesting([]TrustedKey{{ID: "floor-key", PublicKey: pub, Repository: DefaultRepo}})()

	t.Run("persisted floor rejects a replay before download", func(t *testing.T) {
		home := t.TempDir()
		seed(t, home, UpdateState{AppliedFloor: "v0.5.0", Available: "v0.5.0"})

		state, err := LoadUpdateState(home)
		if err != nil || state.AppliedFloor != "v0.5.0" {
			t.Fatalf("persisted floor not readable: %+v err=%v", state, err)
		}
		_, _, err = downloadAndVerifyReleaseWithFloor(context.Background(), http.DefaultClient, DefaultRepo, "v0.4.9", &Release{TagName: "v0.5.0"}, state.AppliedFloor)
		if !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("replay must be rejected before download: %v", err)
		}
		_, _, err = downloadAndVerifyReleaseWithFloor(context.Background(), http.DefaultClient, DefaultRepo, "v0.4.9", &Release{TagName: "v0.5.1"}, state.AppliedFloor)
		if errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("a release above the floor must pass the gate: %v", err)
		}
	})

	t.Run("absent state keeps first-run behavior", func(t *testing.T) {
		state, err := LoadUpdateState(t.TempDir())
		if err != nil || state.AppliedFloor != "" {
			t.Fatalf("first run must load empty floor: %+v err=%v", state, err)
		}
		if err := VerifyVersionFloor("v0.4.9", "v0.5.0", state.AppliedFloor); err != nil {
			t.Fatalf("empty floor must allow a newer release: %v", err)
		}
		if err := VerifyVersionFloor("v0.5.0", "v0.5.0", state.AppliedFloor); !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("equality check must stay intact: %v", err)
		}
		_, _, err = downloadAndVerifyReleaseWithFloor(context.Background(), http.DefaultClient, DefaultRepo, "v0.4.9", &Release{TagName: "v0.5.0"}, state.AppliedFloor)
		if err == nil || errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("empty floor must not block the pipeline: %v", err)
		}
	})
}

func TestAppliedFloorRecordIsForwardOnly(t *testing.T) {
	home := t.TempDir()
	seed(t, home, UpdateState{Available: "v0.5.2"})
	if err := RecordAppliedFloor(home, "v0.5.1", filepath.Join(home, "bin", "cortex-ia")); err != nil {
		t.Fatalf("RecordAppliedFloor: %v", err)
	}
	state, err := LoadUpdateState(home)
	if err != nil || state.AppliedFloor != "v0.5.1" || state.UpdateAvailable() {
		t.Fatalf("apply outcome not reflected: %+v err=%v", state, err)
	}
	if err := RecordAppliedFloor(home, "v0.5.0", ""); err != nil {
		t.Fatalf("RecordAppliedFloor: %v", err)
	}
	if state, _ = LoadUpdateState(home); state.AppliedFloor != "v0.5.1" {
		t.Fatalf("floor moved backwards: %q", state.AppliedFloor)
	}
	if err := RecordAppliedFloor(home, "dev", ""); err == nil {
		t.Fatal("a non-canonical version must not poison the floor")
	}
}

func TestAppliedFloorApplyOutcome(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	keyID := "floor-apply-key"
	defer SetTrustedKeysForTesting([]TrustedKey{{ID: keyID, PublicKey: pub, Repository: DefaultRepo, MinVersion: "v0.5.0"}})()

	bin := testBinaryName()
	payload := []byte("verified-binary-v0.5.1")
	archiveBytes, err := createTestArchive(bin, payload)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	home := t.TempDir()
	for _, tc := range []struct {
		name    string
		tag     string
		corrupt bool
	}{
		{"successful apply raises the persisted floor", "v0.5.1", false},
		{"failed apply never moves the floor", "v0.5.2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed(t, home, UpdateState{AppliedFloor: "v0.5.0", Available: "v0.5.1"})
			rel, srv := setupReleaseServer(t, tc.tag, priv, keyID, archiveBytes, tc.corrupt)
			defer srv.Close()

			path := filepath.Join(t.TempDir(), bin)
			if err := os.WriteFile(path, []byte("old-binary"), 0o755); err != nil {
				t.Fatalf("seed target: %v", err)
			}
			client := &Client{Repo: DefaultRepo, HTTPClient: srv.Client(), StateHome: home}
			applyErr := client.ApplyUpdateToTarget(context.Background(), "v0.5.0", rel, path)
			state, loadErr := LoadUpdateState(home)

			if tc.corrupt {
				if applyErr == nil {
					t.Fatal("expected signature verification failure")
				}
				if loadErr != nil || state.AppliedFloor != "v0.5.0" || state.Available != "v0.5.1" {
					t.Fatalf("failed apply mutated state: %+v err=%v", state, loadErr)
				}
				if got, _ := os.ReadFile(path); string(got) != "old-binary" {
					t.Fatal("target modified by a failed apply")
				}
				return
			}
			if applyErr != nil {
				t.Fatalf("apply failed: %v", applyErr)
			}
			if loadErr != nil || state.AppliedFloor != "v0.5.1" || state.UpdateAvailable() || state.ManagedPath != path {
				t.Fatalf("apply outcome not persisted: %+v err=%v", state, loadErr)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, payload) {
				t.Fatal("target was not replaced with the verified payload")
			}
		})
	}
}
