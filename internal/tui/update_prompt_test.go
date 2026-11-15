package tui

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/updater"
)

func stubApply(t *testing.T, fn func(context.Context, string, string) (string, error)) {
	t.Helper()
	previous := applyUpdate
	applyUpdate = fn
	t.Cleanup(func() { applyUpdate = previous })
}

func seedState(t *testing.T, home string, state updater.UpdateState) {
	t.Helper()
	if err := updater.SaveUpdateStateAtomic(filepath.Join(home, ".cortex-ia"), state); err != nil {
		t.Fatalf("seed update state: %v", err)
	}
}

func loadState(t *testing.T, home string) updater.UpdateState {
	t.Helper()
	state, err := updater.LoadUpdateState(filepath.Join(home, ".cortex-ia"))
	if err != nil {
		t.Fatalf("load update state: %v", err)
	}
	return state
}

func runPromptVersion(t *testing.T, home, version, input string) string {
	t.Helper()
	var out strings.Builder
	if err := MaybePromptUpdate(home, version, bufio.NewReader(strings.NewReader(input)), &out); err != nil {
		t.Fatalf("MaybePromptUpdate returned error: %v", err)
	}
	return out.String()
}

func runPrompt(t *testing.T, home, input string) string {
	t.Helper()
	return runPromptVersion(t, home, "v0.4.50", input)
}

func seedLiveClaim(t *testing.T, home string) {
	t.Helper()
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	if _, err := store.CreateWorkInBoardWithDefinition(ctx, delegation.DefaultBoardID, "gate", "gate", nil, delegation.WorkDefinition{Project: home}); err != nil {
		t.Fatalf("create work: %v", err)
	}
	if _, err := store.ClaimWork(ctx, "gate", "owner", time.Hour); err != nil {
		t.Fatalf("claim work: %v", err)
	}
}

func seedDriftedLedger(t *testing.T, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("create database directory: %v", err)
	}
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT`); err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-01-01T00:00:00Z')`, delegation.MaxSupportedSchemaVersion+1); err != nil {
		t.Fatalf("seed drifted version: %v", err)
	}
}

func TestUpdatePrompt_ConfirmAppliesAndRecordsFloor(t *testing.T) {
	home := t.TempDir()
	seedState(t, home, updater.UpdateState{AppliedFloor: "v0.4.50", Available: "v0.5.0"})

	var gotVersion, gotFloor string
	stubApply(t, func(_ context.Context, version, floor string) (string, error) {
		gotVersion, gotFloor = version, floor
		return "v0.5.0", nil
	})

	out := runPrompt(t, home, "y\n")

	if !strings.Contains(out, "¿Actualizar a v0.5.0? [y/N]") {
		t.Fatalf("prompt did not name the cached target:\n%s", out)
	}
	if gotVersion != "v0.4.50" || gotFloor != "v0.4.50" {
		t.Fatalf("apply args = (%q, %q), want (v0.4.50, v0.4.50)", gotVersion, gotFloor)
	}
	if !strings.Contains(out, "reinicia cortex-ia") {
		t.Fatalf("restart notice missing:\n%s", out)
	}
	state := loadState(t, home)
	if state.AppliedFloor != "v0.5.0" || state.UpdateAvailable() {
		t.Fatalf("state after apply = %+v, want floor v0.5.0 and cleared offer", state)
	}
}

func TestUpdatePrompt_DeclinePreservesOffer(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"n", "n\n"},
		{"EOF", ""},
		{"other key", "yes please\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			seedState(t, home, updater.UpdateState{AppliedFloor: "v0.4.50", Available: "v0.5.0"})
			stubApply(t, func(context.Context, string, string) (string, error) {
				t.Fatal("apply ran without an explicit y")
				return "", nil
			})

			out := runPrompt(t, home, tc.input)
			if !strings.Contains(out, "v0.5.0") {
				t.Fatalf("prompt missing the offer:\n%s", out)
			}
			state := loadState(t, home)
			if state.Available != "v0.5.0" || state.AppliedFloor != "v0.4.50" {
				t.Fatalf("decline mutated state: %+v", state)
			}
		})
	}
}

func TestUpdatePrompt_SilentWithoutOfferOrOnDevBuild(t *testing.T) {
	t.Setenv(inlineCheckEnv, "")
	stubApply(t, func(context.Context, string, string) (string, error) {
		t.Fatal("apply ran without an offer")
		return "", nil
	})

	if out := runPrompt(t, t.TempDir(), "y\n"); out != "" {
		t.Fatalf("unexpected output without cached state:\n%s", out)
	}

	home := t.TempDir()
	seedState(t, home, updater.UpdateState{Available: "v0.5.0"})
	if out := runPromptVersion(t, home, "dev", "y\n"); out != "" {
		t.Fatalf("dev build must stay silent:\n%s", out)
	}
}

func TestUpdatePrompt_LiveAuthorityBlocksApply(t *testing.T) {
	home := t.TempDir()
	seedState(t, home, updater.UpdateState{AppliedFloor: "v0.4.50", Available: "v0.5.0"})
	seedLiveClaim(t, home)
	stubApply(t, func(context.Context, string, string) (string, error) {
		t.Fatal("apply ran while work authority was active")
		return "", nil
	})

	out := runPrompt(t, home, "y\n")
	if !strings.Contains(out, "autoridad de trabajo activa") {
		t.Fatalf("blocked message did not name the condition:\n%s", out)
	}
	if state := loadState(t, home); state.Available != "v0.5.0" {
		t.Fatalf("blocked apply lost the cached offer: %+v", state)
	}
}

func TestUpdatePrompt_ApplyFailureKeepsBinaryAndFloor(t *testing.T) {
	home := t.TempDir()
	seedState(t, home, updater.UpdateState{AppliedFloor: "v0.4.50", Available: "v0.5.0"})
	stubApply(t, func(context.Context, string, string) (string, error) {
		return "", errors.New("artifact digest mismatch")
	})

	out := runPrompt(t, home, "y\n")
	if !strings.Contains(out, "artifact digest mismatch") || !strings.Contains(out, "Se continúa con la versión actual") {
		t.Fatalf("typed apply failure not surfaced:\n%s", out)
	}
	state := loadState(t, home)
	if state.AppliedFloor != "v0.4.50" || state.Available != "v0.5.0" {
		t.Fatalf("failed apply mutated state: %+v", state)
	}
}

func TestUpdatePrompt_SchemaDriftRendersGuidance(t *testing.T) {
	home := t.TempDir()
	seedState(t, home, updater.UpdateState{AppliedFloor: "v0.4.50", Available: "v0.5.0"})
	seedDriftedLedger(t, delegation.DefaultDBPath(home))
	stubApply(t, func(context.Context, string, string) (string, error) {
		t.Fatal("apply ran despite schema drift")
		return "", nil
	})

	out := runPrompt(t, home, "y\n")
	if !strings.Contains(out, "cortex-ia update") {
		t.Fatalf("drift guidance missing the recovery command:\n%s", out)
	}
	if !strings.Contains(out, strconv.Itoa(delegation.MaxSupportedSchemaVersion+1)) {
		t.Fatalf("drift guidance missing the on-disk version:\n%s", out)
	}
}

func TestUpdatePrompt_InlineCheckOptInOffersUpdate(t *testing.T) {
	t.Setenv(inlineCheckEnv, "1")
	previous := inlineUpdateCheck
	inlineUpdateCheck = func(context.Context, string) (string, error) { return "v0.9.0", nil }
	t.Cleanup(func() { inlineUpdateCheck = previous })
	stubApply(t, func(context.Context, string, string) (string, error) {
		t.Fatal("declined inline offer must not apply")
		return "", nil
	})

	out := runPrompt(t, t.TempDir(), "n\n")
	if !strings.Contains(out, "v0.9.0") {
		t.Fatalf("opt-in inline check did not offer the release:\n%s", out)
	}
}
