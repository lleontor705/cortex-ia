package delegation

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxSupportedSchemaVersion is the newest delegation schema this binary can read and write.
const MaxSupportedSchemaVersion = 15

// SchemaDriftError reports a delegation database migrated by a newer cortex-ia build.
// It exists because the plain fail-closed message alone left operators without a recovery path.
type SchemaDriftError struct {
	OnDisk    int
	Supported int
}

func (e *SchemaDriftError) Error() string {
	return fmt.Sprintf("cortex database schema %d is newer than supported schema %d; run 'cortex-ia update' to upgrade cortex-ia", e.OnDisk, e.Supported)
}

// CheckSchemaDriftBeforeOpen probes the migration ledger through a strictly read-only handle so a
// stale binary reports an actionable error before it keeps a write connection. A missing file,
// missing ledger table, or empty ledger is a fresh install, not drift. When the probe itself cannot
// run it returns nil: Store.initialize remains the authoritative fail-closed boundary and the guard
// only enriches the diagnosis.
func CheckSchemaDriftBeforeOpen(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	version, err := readLedgerVersion(ctx, db)
	if err != nil {
		return nil
	}
	if version > MaxSupportedSchemaVersion {
		return &SchemaDriftError{OnDisk: version, Supported: MaxSupportedSchemaVersion}
	}
	return nil
}

// readLedgerVersion reports 0 when the migration ledger has never been created or holds no rows.
func readLedgerVersion(ctx context.Context, db *sql.DB) (int, error) {
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&tables); err != nil {
		return 0, err
	}
	if tables == 0 {
		return 0, nil
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
