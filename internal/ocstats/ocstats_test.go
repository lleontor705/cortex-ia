package ocstats

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureSchema mirrors only the live columns this package reads (PRAGMA-verified before coding).
const fixtureSchema = `
CREATE TABLE session_v2 (
	id TEXT PRIMARY KEY, model TEXT,
	tokens_input INTEGER NOT NULL DEFAULT 0, tokens_output INTEGER NOT NULL DEFAULT 0,
	tokens_reasoning INTEGER NOT NULL DEFAULT 0, tokens_cache_read INTEGER NOT NULL DEFAULT 0,
	tokens_cache_write INTEGER NOT NULL DEFAULT 0,
	time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL
);
CREATE TABLE session_message (
	id TEXT PRIMARY KEY, session_id TEXT NOT NULL, type TEXT NOT NULL, seq INTEGER NOT NULL,
	time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL, data TEXT NOT NULL
);
`

type fixtureSession struct {
	id      string
	model   string
	tokens  int64
	created time.Time
}

func newFixture(t *testing.T, sessions []fixtureSession, messages map[string][]time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(fixtureSchema); err != nil {
		t.Fatalf("apply fixture schema: %v", err)
	}
	for _, session := range sessions {
		ms := session.created.UnixMilli()
		var model any
		if session.model != "" {
			model = session.model
		}
		if _, err := db.Exec(
			"INSERT INTO session_v2 (id, model, tokens_input, time_created, time_updated) VALUES (?,?,?,?,?)",
			session.id, model, session.tokens, ms, ms,
		); err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}
	seq := 0
	for sessionID, times := range messages {
		for _, at := range times {
			seq++
			ms := at.UnixMilli()
			if _, err := db.Exec(
				"INSERT INTO session_message (id, session_id, type, seq, time_created, time_updated, data) VALUES (?,?,?,?,?,?,?)",
				fmt.Sprintf("m%d", seq), sessionID, "assistant", seq, ms, ms, "{}",
			); err != nil {
				t.Fatalf("insert message: %v", err)
			}
		}
	}
	return path
}

func at(day time.Time, hour int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, time.Local)
}

func TestLoadReportAggregatesAndClipsWindows(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	old := now.AddDate(0, 0, -40)
	path := newFixture(t,
		[]fixtureSession{
			{id: "s1", model: `{"id":"alpha","variant":"a"}`, tokens: 1200, created: at(yesterday, 9)},
			{id: "s2", model: `{"id":"alpha","variant":"b"}`, tokens: 3000, created: at(now, 11)},
			{id: "s3", model: `{"id":"beta"}`, tokens: 500, created: at(now, 12)},
			{id: "s4", model: `{"id":"gamma"}`, tokens: 999, created: at(old, 8)},
		},
		map[string][]time.Time{
			"s1": {at(yesterday, 9), at(yesterday, 9)},
			"s2": {at(now, 10), at(now, 10), at(now, 10)},
		})

	report, err := loadReport(path)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}
	if report.DBPath != path || len(report.Days) != 3 {
		t.Fatalf("DBPath=%q days=%d, want fixture with 3 buckets", report.DBPath, len(report.Days))
	}
	for i := 1; i < len(report.Days); i++ {
		if report.Days[i-1].Date >= report.Days[i].Date {
			t.Fatalf("Days not ascending: %v", report.Days)
		}
	}

	all := report.Summaries[WindowAll]
	if all.Sessions != 4 || all.ActiveDays != 3 || all.Tokens != 5699 || all.FavoriteModel != "alpha" {
		t.Fatalf("WindowAll = %+v", all)
	}
	if all.FirstDay != old.Format(dateLayout) || all.LastDay != now.Format(dateLayout) {
		t.Fatalf("WindowAll range = %s..%s", all.FirstDay, all.LastDay)
	}

	week := report.Summaries[Window7d]
	if week.Sessions != 3 || week.ActiveDays != 2 || week.Messages != 5 || week.Tokens != 4700 {
		t.Fatalf("Window7d = %+v", week)
	}
	if week.PeakHour != 10 || week.PeakHourMessages != 3 {
		t.Fatalf("Window7d peak = %d:%d, want 10:3", week.PeakHour, week.PeakHourMessages)
	}
	if week.FirstDay != now.AddDate(0, 0, -6).Format(dateLayout) || week.LastDay != now.Format(dateLayout) {
		t.Fatalf("Window7d range = %s..%s", week.FirstDay, week.LastDay)
	}
	if report.Summaries[Window30d].Sessions != 3 {
		t.Fatalf("Window30d sessions = %d, want 3 (40-day-old clipped)", report.Summaries[Window30d].Sessions)
	}

	ranking := report.Models[Window7d]
	if len(ranking) != 2 {
		t.Fatalf("Window7d ranking = %d rows, want 2 (gamma clipped)", len(ranking))
	}
	if alpha := ranking[0]; alpha.ModelID != "alpha" || alpha.Sessions != 2 || alpha.Tokens != 4200 || alpha.SharePct != 89.4 {
		t.Fatalf("alpha row = %+v", alpha)
	}
	if beta := ranking[1]; beta.ModelID != "beta" || beta.Sessions != 1 || beta.Tokens != 500 || beta.SharePct != 10.6 {
		t.Fatalf("beta row = %+v", beta)
	}
	if got := report.Hours[now.Format(dateLayout)]; len(got) != 1 || got[0].Hour != 10 || got[0].Messages != 3 {
		t.Fatalf("today hour buckets = %+v", got)
	}
}

func TestLoadReportTieBreaksAreDeterministic(t *testing.T) {
	now := time.Now()
	path := newFixture(t,
		[]fixtureSession{
			{id: "b", model: `{"id":"bbb"}`, tokens: 100, created: at(now, 15)},
			{id: "a", model: `{"id":"aaa"}`, tokens: 100, created: at(now, 14)},
		},
		map[string][]time.Time{"a": {at(now, 14)}, "b": {at(now, 15)}})

	report, err := loadReport(path)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}
	summary := report.Summaries[WindowAll]
	if summary.PeakHour != 14 || summary.FavoriteModel != "aaa" {
		t.Fatalf("summary = %+v, want earliest hour 14 and favorite aaa", summary)
	}
	if report.Models[WindowAll][0].ModelID != "aaa" {
		t.Fatalf("ranking head = %q, want aaa", report.Models[WindowAll][0].ModelID)
	}
}

func TestLoadReportExcludesNullModelIDs(t *testing.T) {
	now := time.Now()
	path := newFixture(t,
		[]fixtureSession{
			{id: "a", model: `{"id":"aaa"}`, tokens: 100, created: at(now, 10)},
			{id: "n", tokens: 100, created: at(now, 10)},
			{id: "w", model: `{"providerID":"p"}`, tokens: 100, created: at(now, 10)},
		}, nil)

	report, err := loadReport(path)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}
	if len(report.Models[WindowAll]) != 1 || report.Models[WindowAll][0].ModelID != "aaa" {
		t.Fatalf("ranking = %+v, want only aaa", report.Models[WindowAll])
	}
	if report.Summaries[WindowAll].Tokens != 300 {
		t.Fatalf("tokens = %d, want 300 including null-model sessions", report.Summaries[WindowAll].Tokens)
	}
}

func TestLoadReportTypedErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	_, err := loadReport(missing)
	var notFound *NotFoundError
	if !errors.As(err, &notFound) || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing error = %v, want typed *NotFoundError naming %q", err, missing)
	}

	garbage := filepath.Join(t.TempDir(), "garbage.db")
	if werr := os.WriteFile(garbage, []byte("not a sqlite database"), 0o600); werr != nil {
		t.Fatalf("write garbage fixture: %v", werr)
	}
	_, err = loadReport(garbage)
	var unreadable *UnreadableError
	if !errors.As(err, &unreadable) || !strings.Contains(err.Error(), garbage) {
		t.Fatalf("unreadable error = %v, want typed *UnreadableError naming %q", err, garbage)
	}
}

func TestDefaultDBPathAndEnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv(dbEnvVar, custom)
	resolved, err := DefaultDBPath()
	if err != nil || resolved != custom {
		t.Fatalf("override = %q (%v), want %q", resolved, err, custom)
	}

	os.Unsetenv(dbEnvVar)
	fallback, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("resolve fallback: %v", err)
	}
	if want := filepath.Join(".local", "share", "opencode", "opencode.db"); !strings.HasSuffix(fallback, want) {
		t.Fatalf("fallback = %q, want suffix %q", fallback, want)
	}

	now := time.Now()
	path := newFixture(t, []fixtureSession{{id: "s1", tokens: 10, created: at(now, 10)}}, nil)
	t.Setenv(dbEnvVar, path)
	report, err := LoadReport()
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}
	if report.DBPath != path || report.Summaries[WindowAll].Sessions != 1 {
		t.Fatalf("report = %+v, want fixture %q with 1 session", report, path)
	}
}

func TestLoadReportOnEmptyFixtureDoesNotPanic(t *testing.T) {
	report, err := loadReport(newFixture(t, nil, nil))
	if err != nil {
		t.Fatalf("load empty fixture: %v", err)
	}
	summary := report.Summaries[WindowAll]
	if len(report.Days) != 0 || summary.Sessions != 0 || summary.Tokens != 0 || summary.FavoriteModel != "" {
		t.Fatalf("empty summary = %+v", summary)
	}
	if report.Summaries[Window7d].FirstDay == "" {
		t.Fatalf("trailing window bounds must stay anchored to today: %+v", report.Summaries[Window7d])
	}
}
