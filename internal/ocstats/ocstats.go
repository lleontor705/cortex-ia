// Package ocstats exposes read-only, windowed aggregates of OpenCode's own
// usage database for the cortex-ia statistics panel.
//
// The package never mutates the OpenCode database: it opens it with a read-only
// DSN, performs one bounded aggregate pass, and reduces every window in memory.
// One load therefore serves all window filters, so switching filters cannot pin
// the live WAL connection again.
package ocstats

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Window int

const (
	WindowAll Window = iota
	Window30d
	Window7d
)

// trailingWindowDays maps each trailing window to its calendar span. WindowAll
// is intentionally absent: it is bounded by the real data range, never a fixed
// span.
var trailingWindowDays = map[Window]int{
	Window30d: 30,
	Window7d:  7,
}

type DayBucket struct {
	Date     string
	Sessions int
	Messages int
	Tokens   int64
}

type HourBucket struct {
	Hour     int
	Messages int
}

type ModelUsage struct {
	ModelID  string
	Sessions int
	Tokens   int64
	SharePct float64
}

type Summary struct {
	Window           Window
	Sessions         int
	Messages         int
	Tokens           int64
	ActiveDays       int
	PeakHour         int
	PeakHourMessages int
	FavoriteModel    string
	FirstDay         string
	LastDay          string
}

type Report struct {
	DBPath    string
	Days      []DayBucket
	Hours     map[string][]HourBucket
	Models    map[Window][]ModelUsage
	Summaries map[Window]Summary
}

const dbEnvVar = "CORTEX_IA_OPENCODE_DB"

// NotFoundError reports a resolved database path that does not exist.
type NotFoundError struct{ Path string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("opencode database not found at %s", e.Path)
}

// UnreadableError reports a resolved database path that exists but cannot be
// read, including schema drift surfaced as a failed query.
type UnreadableError struct {
	Path string
	Err  error
}

func (e *UnreadableError) Error() string {
	return fmt.Sprintf("opencode database unreadable at %s: %v", e.Path, e.Err)
}

func (e *UnreadableError) Unwrap() error { return e.Err }

// DefaultDBPath resolves the OpenCode usage database. CORTEX_IA_OPENCODE_DB
// takes precedence when set to a non-empty value; otherwise the path is the
// platform default under the user home.
func DefaultDBPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(dbEnvVar)); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for opencode database: %w", err)
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db"), nil
}

// LoadReport resolves the database and returns every window's aggregates. The
// connection is closed before the report reaches the caller.
func LoadReport() (Report, error) {
	path, err := DefaultDBPath()
	if err != nil {
		return Report{}, err
	}
	return loadReport(path)
}

func loadReport(path string) (Report, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = db.Close() }()

	raw, err := scanRaw(db)
	if err != nil {
		return Report{}, &UnreadableError{Path: path, Err: err}
	}
	return reduceReport(path, raw, time.Now()), nil
}

func openReadOnly(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &NotFoundError{Path: path}
		}
		return nil, &UnreadableError{Path: path, Err: err}
	}
	db, err := sql.Open("sqlite", readOnlyDSN(path))
	if err != nil {
		return nil, &UnreadableError{Path: path, Err: err}
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, &UnreadableError{Path: path, Err: err}
	}
	return db, nil
}

func readOnlyDSN(path string) string {
	escaped := strings.NewReplacer("?", "%3F", "#", "%23").Replace(filepath.ToSlash(path))
	return "file:" + escaped + "?mode=ro&_pragma=busy_timeout(10000)&_pragma=query_only(1)"
}
