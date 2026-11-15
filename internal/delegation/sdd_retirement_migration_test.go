package delegation

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSDDRetirementMigrationClearsContractsAndDropsArchives(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open fresh store: %v", err)
	}
	if _, err := store.CreateBoard(ctx, "sdd-board", "SDD Board", ""); err != nil {
		t.Fatalf("create board: %v", err)
	}
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "sdd-board", "sdd-task", "SDD task", nil, WorkDefinition{
		Project: t.TempDir(), Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/a.go"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Rewind to the pre-retirement state: an sdd-lite contract stored, archive table present, ledger at 18.
	contract := `{"version":1,"workflow":"sdd-lite","change_id":"c","spec_plane":"hybrid","pins":[],"requirement_ids":["REQ-MIG-001"]}`
	if _, err := store.db.Exec(`UPDATE work_definitions SET contract_json=? WHERE item_id=?`, contract, item.ID); err != nil {
		t.Fatalf("seed sdd contract: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE change_archives (archive_key TEXT PRIMARY KEY) STRICT`,
		`DELETE FROM schema_migrations WHERE version=19`,
	} {
		if _, err := store.db.Exec(stmt); err != nil {
			t.Fatalf("seed previous schema %q: %v", stmt, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close seeded store: %v", err)
	}

	migrated, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	t.Cleanup(func() { _ = migrated.Close() })

	var version int
	if err := migrated.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if version != MaxSupportedSchemaVersion {
		t.Fatalf("ledger version = %d, want %d", version, MaxSupportedSchemaVersion)
	}
	var stored string
	if err := migrated.db.QueryRow(`SELECT contract_json FROM work_definitions WHERE item_id=?`, item.ID).Scan(&stored); err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if stored != "" {
		t.Fatalf("contract_json = %q, want empty direct definition", stored)
	}
	migratedItem, err := migrated.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatalf("read migrated task: %v", err)
	}
	if migratedItem.Objective != "objective" || migratedItem.Status != WorkReady {
		t.Fatalf("migration altered task definition: %+v", migratedItem)
	}
	var archives int
	if err := migrated.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='change_archives'`).Scan(&archives); err != nil {
		t.Fatalf("probe change archives: %v", err)
	}
	if archives != 0 {
		t.Fatal("change_archives survived the SDD retirement migration")
	}
}
