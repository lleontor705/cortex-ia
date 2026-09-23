package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
)

// productionBoot unlocks the shipped zero-argument boot surface for one test and
// restores whatever navigation_test.go's init() pinned afterwards.
func productionBoot(t *testing.T) {
	t.Helper()
	previous := bootScreen
	bootScreen = productionBootScreen
	t.Cleanup(func() { bootScreen = previous })
}

// TestStatsBootProductionScreenLoadingFrame proves the production boot surface:
// newModel opens screenStats and the first rendered frame is the loading state,
// before any load command result is processed (REQ-US-002).
func TestStatsBootProductionScreenLoadingFrame(t *testing.T) {
	productionBoot(t)
	if productionBootScreen != screenStats {
		t.Fatalf("production boot surface = %v, want screenStats", productionBootScreen)
	}

	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	if m.screen != screenStats || !m.stats.loading {
		t.Fatalf("zero-arg boot = screen %v loading %v, want screenStats/loading", m.screen, m.stats.loading)
	}

	view := m.View()
	if !strings.Contains(view, "Estadísticas de uso") || !strings.Contains(view, "Cargando") {
		t.Fatalf("first boot frame must render the loading stats screen:\n%s", view)
	}
	if strings.Contains(view, "Sesiones") {
		t.Fatalf("first boot frame must not fabricate metric cards:\n%s", view)
	}
}

// TestStatsBootLoadRendersSyntheticReport proves the boot load command fills the
// screen from the injected report and that cards, heatmap and footer co-render
// on one frame (REQ-US-003/005/007) without opening the real OpenCode database.
func TestStatsBootLoadRendersSyntheticReport(t *testing.T) {
	productionBoot(t)
	previous := statsLoadReport
	statsLoadReport = func() (ocstats.Report, error) { return statsFixtureReport(), nil }
	t.Cleanup(func() { statsLoadReport = previous })

	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	m = drive(t, m, m.Init())

	if m.screen != screenStats || m.stats.loading || m.stats.err != nil {
		t.Fatalf("after load: screen %v loading %v err %v, want loaded screenStats", m.screen, m.stats.loading, m.stats.err)
	}
	view := m.View()
	for _, want := range []string{
		"Sesiones", "Mensajes", "Tokens totales", "Días activos", "Hora pico", "Modelo favorito",
		"12,345", "10:00", "model-x", "█", "dom", "Menos", "Un 24.7% de The Great Gatsby",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("loaded boot frame missing %q:\n%s", want, view)
		}
	}
}

// TestStatsBootDegradesOnMissingDatabase proves a resolved-but-absent
// CORTEX_IA_OPENCODE_DB still boots the TUI through the production loader: the
// degraded frame names the resolved path, drops the cards, and esc stays
// functional (REQ-US-002/003).
func TestStatsBootDegradesOnMissingDatabase(t *testing.T) {
	productionBoot(t)
	missing := filepath.Join(t.TempDir(), "missing-opencode.db")
	t.Setenv("CORTEX_IA_OPENCODE_DB", missing)

	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	if view := m.View(); !strings.Contains(view, "Cargando") {
		t.Fatalf("degraded boot must still render its loading frame:\n%s", view)
	}
	m = drive(t, m, m.Init())

	if m.stats.loading || m.stats.err == nil {
		t.Fatalf("missing database must degrade: loading %v err %v", m.stats.loading, m.stats.err)
	}
	view := m.View()
	if !strings.Contains(view, "No se encontró la base de OpenCode en "+missing) {
		t.Fatalf("degraded frame must name the resolved path %q:\n%s", missing, view)
	}
	if strings.Contains(view, "Sesiones") {
		t.Fatalf("degraded frame must not render cards:\n%s", view)
	}

	m = press(m, "esc")
	if m.screen != screenHome || m.cursor != statsEntryIndex {
		t.Fatalf("esc from degraded stats = screen %v cursor %d, want home/%d", m.screen, m.cursor, statsEntryIndex)
	}
}

// TestStatsBootEscAndHotkeyReopen proves esc returns Home resting on the stats
// entry and the numeric hotkey re-enters the screen dispatching a fresh load
// (REQ-US-002).
func TestStatsBootEscAndHotkeyReopen(t *testing.T) {
	productionBoot(t)
	loads := 0
	previous := statsLoadReport
	statsLoadReport = func() (ocstats.Report, error) {
		loads++
		return statsFixtureReport(), nil
	}
	t.Cleanup(func() { statsLoadReport = previous })

	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	m = drive(t, m, m.Init())
	m = press(m, "esc")
	if m.screen != screenHome || m.cursor != statsEntryIndex {
		t.Fatalf("esc = screen %v cursor %d, want home/%d", m.screen, m.cursor, statsEntryIndex)
	}

	updated, cmd := m.Update(key("5"))
	m = updated.(model)
	if m.screen != screenStats || !m.stats.loading || cmd == nil {
		t.Fatalf("hotkey 5 = screen %v loading %v cmd present %v, want stats/loading/command", m.screen, m.stats.loading, cmd != nil)
	}
	m = drive(t, m, cmd)
	if loads != 2 || m.stats.loading || m.stats.err != nil {
		t.Fatalf("hotkey reopen: loads %d loading %v err %v, want 2 loaded loads", loads, m.stats.loading, m.stats.err)
	}
}
