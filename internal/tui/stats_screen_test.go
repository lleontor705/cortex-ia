package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/ocstats"
)

func statsFixtureReport() ocstats.Report {
	all := ocstats.Summary{
		Window: ocstats.WindowAll, Sessions: 3, Messages: 14, Tokens: 12345,
		ActiveDays: 3, PeakHour: 10, PeakHourMessages: 6, FavoriteModel: "model-x",
		FirstDay: "2026-09-20", LastDay: "2026-09-22",
	}
	week := ocstats.Summary{
		Window: ocstats.Window7d, Sessions: 1, Messages: 3, Tokens: 10000,
		ActiveDays: 1, PeakHour: 9, PeakHourMessages: 3, FavoriteModel: "model-y",
		FirstDay: "2026-09-16", LastDay: "2026-09-22",
	}
	return ocstats.Report{
		DBPath: "C:/synthetic/opencode.db",
		Days: []ocstats.DayBucket{
			{Date: "2026-09-20", Sessions: 1, Messages: 5, Tokens: 4000},
			{Date: "2026-09-21", Sessions: 1, Messages: 6, Tokens: 4000},
			{Date: "2026-09-22", Sessions: 1, Messages: 3, Tokens: 4345},
		},
		Summaries: map[ocstats.Window]ocstats.Summary{
			ocstats.WindowAll: all,
			ocstats.Window30d: all,
			ocstats.Window7d:  week,
		},
		Models: map[ocstats.Window][]ocstats.ModelUsage{
			ocstats.WindowAll: {
				{ModelID: "model-b", Sessions: 2, Tokens: 700, SharePct: 70.0},
				{ModelID: "model-a", Sessions: 2, Tokens: 300, SharePct: 30.0},
			},
			ocstats.Window7d: {{ModelID: "model-y", Sessions: 1, Tokens: 10000, SharePct: 100.0}},
		},
	}
}

func loadedStats() statsState {
	return newStatsState().onLoaded(statsLoadedMsg{
		report: statsFixtureReport(),
		dbPath: "C:/synthetic/opencode.db",
	})
}

func statsStateWithTokens(tokens int64) statsState {
	s := newStatsState()
	report := statsFixtureReport()
	summary := report.Summaries[ocstats.WindowAll]
	summary.Tokens = tokens
	report.Summaries[ocstats.WindowAll] = summary
	return s.onLoaded(statsLoadedMsg{report: report})
}

func TestStatsLoadingStateShowsNoCards(t *testing.T) {
	out := newStatsState().view(80)
	if !strings.Contains(out, "Cargando") {
		t.Fatalf("loading view = %q, want a loading indicator", out)
	}
	if strings.Contains(out, "Sesiones") {
		t.Fatalf("loading view must not fabricate metric cards: %q", out)
	}
}

func TestStatsDefaultWindowIsTodoSummary(t *testing.T) {
	s := newStatsState()
	if s.window != ocstats.WindowAll {
		t.Fatalf("default window = %v, want WindowAll (Todo)", s.window)
	}
	if s.tab != statsTabSummary {
		t.Fatalf("default tab = %v, want summary", s.tab)
	}
}

func TestStatsCardsRenderWindowValues(t *testing.T) {
	out := loadedStats().view(90)
	for _, want := range []string{
		"Sesiones", "Mensajes", "Tokens totales", "Días activos", "Hora pico", "Modelo favorito",
		"12,345", "10:00", "model-x", "[ 1 Todo ]", "[ Resumen ]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary view missing %q", want)
		}
	}
}

func TestStatsWindowSwitchIsPureRender(t *testing.T) {
	previous := statsLoadReport
	statsLoadReport = func() (ocstats.Report, error) {
		t.Fatal("window switching must not reload the database")
		return ocstats.Report{}, nil
	}
	t.Cleanup(func() { statsLoadReport = previous })

	s := loadedStats()
	s, action := s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if action != statsActionNone || s.window != ocstats.Window7d {
		t.Fatalf("after '3' window = %v action = %v, want Window7d/none", s.window, action)
	}
	out := s.view(90)
	if !strings.Contains(out, "[ 3 7d ]") {
		t.Errorf("view = %q, want the 7d filter active", out)
	}
	if !strings.Contains(out, "10,000") {
		t.Errorf("view = %q, want the 7d token total", out)
	}
	if strings.Contains(out, "12,345") {
		t.Errorf("view = %q, must derive from the 7d window only", out)
	}
}

func TestStatsUpdateTabAndFilterKeys(t *testing.T) {
	s := loadedStats()
	s, action := s.update(tea.KeyMsg{Type: tea.KeyTab})
	if action != statsActionNone || s.tab != statsTabModels {
		t.Fatalf("tab key: tab = %v action = %v, want models/none", s.tab, action)
	}
	s, _ = s.update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.tab != statsTabSummary {
		t.Fatalf("left key: tab = %v, want summary", s.tab)
	}

	windows := []struct {
		runes []rune
		want  ocstats.Window
	}{
		{[]rune("1"), ocstats.WindowAll},
		{[]rune("2"), ocstats.Window30d},
		{[]rune("3"), ocstats.Window7d},
	}
	for _, tc := range windows {
		s, _ = s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: tc.runes})
		if s.window != tc.want {
			t.Errorf("key %q: window = %v, want %v", string(tc.runes), s.window, tc.want)
		}
	}

	if _, action := s.update(tea.KeyMsg{Type: tea.KeyEsc}); action != statsActionHome {
		t.Errorf("esc action = %v, want home", action)
	}
	if _, action := s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); action != statsActionQuit {
		t.Errorf("q action = %v, want quit", action)
	}
	if _, action := s.update(tea.KeyMsg{Type: tea.KeyCtrlC}); action != statsActionQuit {
		t.Errorf("ctrl+c action = %v, want quit", action)
	}
}

func TestStatsModelsTabRanksByOrder(t *testing.T) {
	s := loadedStats()
	s, _ = s.update(tea.KeyMsg{Type: tea.KeyTab})
	out := s.view(90)
	if !strings.Contains(out, "[ Modelos ]") {
		t.Fatalf("view = %q, want the Modelos tab active", out)
	}
	for _, want := range []string{"Modelo", "Sesiones", "Tokens", "Share", "70.0%", "30.0%"} {
		if !strings.Contains(out, want) {
			t.Errorf("ranking view missing %q", want)
		}
	}
	if strings.Index(out, "model-b") > strings.Index(out, "model-a") {
		t.Errorf("ranking = %q, want model-b before model-a", out)
	}
}

func TestStatsModelsTabEmptyWindow(t *testing.T) {
	s := newStatsState().onLoaded(statsLoadedMsg{})
	s.tab = statsTabModels
	if out := s.view(90); !strings.Contains(out, "Sin modelos registrados") {
		t.Fatalf("empty ranking view = %q, want the empty notice", out)
	}
}

func TestStatsFooterBranches(t *testing.T) {
	cases := []struct {
		tokens int64
		want   string
		absent string
	}{
		{150000, "3.0x más tokens que The Great Gatsby", "NaN"},
		{5000, "Un 10.0% de The Great Gatsby en tokens", "NaN"},
		{0, "Un 0.0% de The Great Gatsby en tokens", "NaN"},
	}
	for _, tc := range cases {
		out := statsStateWithTokens(tc.tokens).view(90)
		if !strings.Contains(out, tc.want) {
			t.Errorf("tokens %d: view = %q, want %q", tc.tokens, out, tc.want)
		}
		if strings.Contains(out, tc.absent) {
			t.Errorf("tokens %d: view must not contain %q", tc.tokens, tc.absent)
		}
	}
}

func TestStatsDegradedStateNamesResolvedPath(t *testing.T) {
	notFound := newStatsState().onLoaded(statsLoadedMsg{
		dbPath: "X:/missing.db",
		err:    &ocstats.NotFoundError{Path: "X:/missing.db"},
	})
	out := notFound.view(90)
	if !strings.Contains(out, "No se encontró la base de OpenCode en X:/missing.db") {
		t.Fatalf("degraded view = %q, want the not-found path", out)
	}
	if strings.Contains(out, "Sesiones") {
		t.Fatalf("degraded view must not render cards: %q", out)
	}

	unreadable := newStatsState().onLoaded(statsLoadedMsg{
		dbPath: "X:/locked.db",
		err:    &ocstats.UnreadableError{Path: "X:/locked.db", Err: errors.New("locked")},
	})
	if out := unreadable.view(90); !strings.Contains(out, "No se pudo leer la base de OpenCode en X:/locked.db") {
		t.Fatalf("unreadable view = %q, want the unreadable path", out)
	}
}

func TestStatsLoadCmdCarriesTypedFailurePath(t *testing.T) {
	previous := statsLoadReport
	statsLoadReport = func() (ocstats.Report, error) {
		return ocstats.Report{}, &ocstats.NotFoundError{Path: "X:/absent.db"}
	}
	t.Cleanup(func() { statsLoadReport = previous })

	msg, ok := statsLoadCmd()().(statsLoadedMsg)
	if !ok {
		t.Fatal("load command must deliver a statsLoadedMsg")
	}
	if msg.err == nil || msg.dbPath != "X:/absent.db" {
		t.Fatalf("msg = %+v, want the typed failure and its resolved path", msg)
	}
}

func TestStatsGroupThousands(t *testing.T) {
	cases := map[int64]string{0: "0", 14: "14", 12345: "12,345", 1234567: "1,234,567"}
	for in, want := range cases {
		if got := groupThousands(in); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", in, got, want)
		}
	}
}
