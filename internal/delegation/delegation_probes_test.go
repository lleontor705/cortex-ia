package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedLedger(t *testing.T, dbPath string, version int) {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT`); err != nil {
		t.Fatalf("create seed ledger: %v", err)
	}
	if version > 0 {
		if _, err := raw.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-01-01T00:00:00Z')`, version); err != nil {
			t.Fatalf("insert seed ledger version %d: %v", version, err)
		}
	}
}

func seedLedgerVersion(t *testing.T, dbPath string, version int) {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open ledger database: %v", err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-01-01T00:00:00Z')`, version); err != nil {
		t.Fatalf("insert ledger version %d: %v", version, err)
	}
}

func ledgerHasTable(t *testing.T, dbPath, table string) bool {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open probe database: %v", err)
	}
	defer func() { _ = raw.Close() }()
	var count int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		t.Fatalf("inspect sqlite_master: %v", err)
	}
	return count > 0
}

func TestSchemaDriftGuard(t *testing.T) {
	t.Run("future schema fails closed pre-open with recovery hint", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "delegation.db")
		seedLedger(t, dbPath, MaxSupportedSchemaVersion+1)

		store, err := OpenStore(dbPath)
		if err == nil {
			_ = store.Close()
			t.Fatal("OpenStore accepted a future schema")
		}
		var drift *SchemaDriftError
		if !errors.As(err, &drift) {
			t.Fatalf("error type = %T (%v), want *SchemaDriftError", err, err)
		}
		if drift.OnDisk != MaxSupportedSchemaVersion+1 || drift.Supported != MaxSupportedSchemaVersion {
			t.Fatalf("drift = %+v, want OnDisk=%d Supported=%d", drift, MaxSupportedSchemaVersion+1, MaxSupportedSchemaVersion)
		}
		for _, want := range []string{"cortex-ia update", "newer than supported"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q missing %q", err.Error(), want)
			}
		}
		if ledgerHasTable(t, dbPath, "work_items") {
			t.Fatal("pre-open guard let the write path migrate the database")
		}
	})

	t.Run("fresh database passes", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "delegation.db")
		if err := CheckSchemaDriftBeforeOpen(dbPath); err != nil {
			t.Fatalf("guard rejected a missing database: %v", err)
		}
		store, err := OpenStore(dbPath)
		if err != nil {
			t.Fatalf("OpenStore failed on fresh database: %v", err)
		}
		defer func() { _ = store.Close() }()
		var version int
		if err := store.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		if version != MaxSupportedSchemaVersion {
			t.Fatalf("fresh ledger version = %d, want %d", version, MaxSupportedSchemaVersion)
		}
	})

	t.Run("missing ledger table passes", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "delegation.db")
		raw, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("open seed database: %v", err)
		}
		if _, err := raw.Exec(`CREATE TABLE unrelated (id INTEGER PRIMARY KEY) STRICT`); err != nil {
			t.Fatalf("create unrelated table: %v", err)
		}
		if err := raw.Close(); err != nil {
			t.Fatalf("close seed database: %v", err)
		}
		if err := CheckSchemaDriftBeforeOpen(dbPath); err != nil {
			t.Fatalf("guard rejected a ledger-less database: %v", err)
		}
		store, err := OpenStore(dbPath)
		if err != nil {
			t.Fatalf("OpenStore failed on ledger-less database: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	t.Run("real WAL database drift is detected", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "delegation.db")
		store, err := OpenStore(dbPath)
		if err != nil {
			t.Fatalf("OpenStore failed: %v", err)
		}
		_ = store.Close()
		seedLedgerVersion(t, dbPath, MaxSupportedSchemaVersion+1)
		var drift *SchemaDriftError
		if err := CheckSchemaDriftBeforeOpen(dbPath); !errors.As(err, &drift) {
			t.Fatalf("guard error = %v, want *SchemaDriftError", err)
		}
	})

	for _, version := range []int{0, MaxSupportedSchemaVersion} {
		t.Run(fmt.Sprintf("ledger version %d passes", version), func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "delegation.db")
			seedLedger(t, dbPath, version)
			if err := CheckSchemaDriftBeforeOpen(dbPath); err != nil {
				t.Fatalf("guard rejected ledger version %d: %v", version, err)
			}
		})
	}
}

func probeStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedProbeItem(t *testing.T, store *Store, id, status string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO work_items(id,title,status,created_at,updated_at) VALUES(?,?,?,?,?)`, id, id, status, now, now); err != nil {
		t.Fatalf("seed work item %s: %v", id, err)
	}
}

func seedProbeClaim(t *testing.T, store *Store, itemID string, expires time.Time) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO work_claims(item_id,owner,token_hash,attempt,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		itemID, "probe-owner", "hash", 1, expires.UTC().Format(time.RFC3339Nano), now, now); err != nil {
		t.Fatalf("seed claim for %s: %v", itemID, err)
	}
}

func seedProbeLease(t *testing.T, store *Store, itemID, path string, expires time.Time) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO work_leases(path,item_id,token_hash,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		path, itemID, "hash", expires.UTC().Format(time.RFC3339Nano), now, now); err != nil {
		t.Fatalf("seed lease %s: %v", path, err)
	}
}

func TestActiveAuthorityProbe(t *testing.T) {
	ctx := context.Background()

	t.Run("idle store reports false", func(t *testing.T) {
		store := probeStore(t)
		active, err := HasAnyActiveAuthority(ctx, store)
		if err != nil || active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (false, nil)", active, err)
		}
	})

	t.Run("unexpired claim reports true", func(t *testing.T) {
		store := probeStore(t)
		seedProbeItem(t, store, "probe-claim", "ready")
		seedProbeClaim(t, store, "probe-claim", time.Now().Add(time.Hour))
		if active, err := HasAnyActiveAuthority(ctx, store); err != nil || !active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (true, nil)", active, err)
		}
	})

	t.Run("expired claim without in_progress reports false", func(t *testing.T) {
		store := probeStore(t)
		seedProbeItem(t, store, "probe-expired", "ready")
		seedProbeClaim(t, store, "probe-expired", time.Now().Add(-time.Hour))
		if active, err := HasAnyActiveAuthority(ctx, store); err != nil || active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (false, nil)", active, err)
		}
	})

	t.Run("unexpired lease reports true", func(t *testing.T) {
		store := probeStore(t)
		seedProbeItem(t, store, "probe-lease", "ready")
		seedProbeClaim(t, store, "probe-lease", time.Now().Add(-time.Hour))
		seedProbeLease(t, store, "probe-lease", "internal/delegation/store.go", time.Now().Add(time.Hour))
		if active, err := HasAnyActiveAuthority(ctx, store); err != nil || !active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (true, nil)", active, err)
		}
	})

	t.Run("in_progress task reports true", func(t *testing.T) {
		store := probeStore(t)
		seedProbeItem(t, store, "probe-running", "in_progress")
		seedProbeClaim(t, store, "probe-running", time.Now().Add(-time.Hour))
		if active, err := HasAnyActiveAuthority(ctx, store); err != nil || !active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (true, nil)", active, err)
		}
	})

	t.Run("missing database reports false through the gate path", func(t *testing.T) {
		store, err := OpenStoreReadOnly(filepath.Join(t.TempDir(), "absent", "delegation.db"))
		if err == nil {
			_ = store.Close()
			t.Fatal("expected a missing-database error from the read-only open")
		}
		var absent *Store
		if active, probeErr := HasAnyActiveAuthority(ctx, absent); probeErr != nil || active {
			t.Fatalf("HasAnyActiveAuthority = (%v, %v), want (false, nil)", active, probeErr)
		}
	})
}
