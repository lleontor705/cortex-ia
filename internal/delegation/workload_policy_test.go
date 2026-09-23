package delegation

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

const policyTestBoard = "policy-board"

func workloadPolicyStore(t *testing.T) (*Store, string) {
	t.Helper()
	workspace := t.TempDir()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(context.Background(), policyTestBoard, "Policy Board", ""); err != nil {
		t.Fatalf("create board: %v", err)
	}
	return store, workspace
}

func workloadPolicyCreate(t *testing.T, store *Store, workspace, id string, policy WorkloadPolicy) (WorkItem, error) {
	t.Helper()
	return store.CreateWorkInBoardWithDefinition(context.Background(), policyTestBoard, id, id+" title", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification",
		AllowedFiles: []string{"src/a.go"}, WorkloadPolicy: policy,
	})
}

func workloadPolicyStored(t *testing.T, store *Store, id string) string {
	t.Helper()
	var stored string
	if err := store.db.QueryRow(`SELECT workload_policy FROM work_items WHERE id=?`, id).Scan(&stored); err != nil {
		t.Fatalf("read stored policy for %s: %v", id, err)
	}
	return stored
}

func TestWorkloadPolicyDefaultsToFlexible(t *testing.T) {
	store, workspace := workloadPolicyStore(t)
	item, err := workloadPolicyCreate(t, store, workspace, "policy-default", "")
	if err != nil {
		t.Fatalf("create task without policy: %v", err)
	}
	if item.WorkloadPolicy != WorkloadPolicyFlexible {
		t.Fatalf("created task policy = %q, want %q", item.WorkloadPolicy, WorkloadPolicyFlexible)
	}
	if stored := workloadPolicyStored(t, store, item.ID); stored != string(WorkloadPolicyFlexible) {
		t.Fatalf("stored policy = %q, want %q", stored, WorkloadPolicyFlexible)
	}
	read, err := store.GetWork(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if read.WorkloadPolicy != WorkloadPolicyFlexible {
		t.Fatalf("read-back policy = %q, want %q", read.WorkloadPolicy, WorkloadPolicyFlexible)
	}
}

func TestWorkloadPolicyPersistsExplicitValue(t *testing.T) {
	store, workspace := workloadPolicyStore(t)
	for _, policy := range []WorkloadPolicy{WorkloadPolicyStrict, WorkloadPolicyUnbounded} {
		id := "policy-" + string(policy)
		item, err := workloadPolicyCreate(t, store, workspace, id, policy)
		if err != nil {
			t.Fatalf("create %s task: %v", policy, err)
		}
		if item.WorkloadPolicy != policy {
			t.Fatalf("created task policy = %q, want %q", item.WorkloadPolicy, policy)
		}
		if stored := workloadPolicyStored(t, store, id); stored != string(policy) {
			t.Fatalf("stored policy = %q, want %q", stored, policy)
		}
		read, err := store.GetWork(context.Background(), id)
		if err != nil {
			t.Fatalf("read %s task: %v", policy, err)
		}
		if read.WorkloadPolicy != policy {
			t.Fatalf("read-back policy = %q, want %q", read.WorkloadPolicy, policy)
		}
	}
}

func TestWorkloadPolicyRejectsUnknownValue(t *testing.T) {
	store, workspace := workloadPolicyStore(t)
	_, err := workloadPolicyCreate(t, store, workspace, "policy-invalid", "turbo")
	if err == nil {
		t.Fatal("unknown workload policy was accepted")
	}
	if !strings.Contains(err.Error(), "strict, flexible or unbounded") {
		t.Fatalf("unexpected rejection error: %v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_items WHERE id='policy-invalid'`).Scan(&count); err != nil {
		t.Fatalf("count rejected task: %v", err)
	}
	if count != 0 {
		t.Fatalf("rejected policy created %d work items", count)
	}
}

func TestWorkloadPolicyMigrationBackfillsPreviousSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delegation.db")
	seedPreviousSchema(t, dbPath, 14, true)

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var version int
	if err := store.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if version != 15 {
		t.Fatalf("ledger version = %d, want 15", version)
	}
	var backfilled string
	if err := store.db.QueryRow(`SELECT workload_policy FROM work_items WHERE id='legacy-task'`).Scan(&backfilled); err != nil {
		t.Fatalf("read backfilled legacy policy: %v", err)
	}
	if backfilled != string(WorkloadPolicyFlexible) {
		t.Fatalf("backfilled policy = %q, want %q", backfilled, WorkloadPolicyFlexible)
	}
}

func TestWorkloadPolicyMigrationRejectsFutureSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delegation.db")
	seedPreviousSchema(t, dbPath, 16, false)

	if _, err := OpenStore(dbPath); err == nil {
		t.Fatal("store accepted a future schema version")
	} else if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("unexpected future-schema error: %v", err)
	}
}

func TestWorkloadPolicyInheritedByDecompositionChildren(t *testing.T) {
	ctx := context.Background()
	store, workspace := workloadPolicyStore(t)
	parent, err := workloadPolicyCreate(t, store, workspace, "policy-parent", WorkloadPolicyStrict)
	if err != nil {
		t.Fatalf("create strict parent: %v", err)
	}
	if _, err := store.db.Exec(`UPDATE work_items SET status='blocked',revision=revision+1 WHERE id=?`, parent.ID); err != nil {
		t.Fatalf("block parent: %v", err)
	}
	blocked, err := store.GetWork(ctx, parent.ID)
	if err != nil {
		t.Fatalf("read blocked parent: %v", err)
	}

	result, err := store.DecomposeWork(ctx, parent.ID, blocked.Revision, []WorkStepDefinition{
		{ID: "policy-child-1", Title: "child one", Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/a.go"}},
		{ID: "policy-child-2", Title: "child two", Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/b.go"}},
	})
	if err != nil {
		t.Fatalf("decompose blocked parent: %v", err)
	}
	if len(result.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(result.Children))
	}
	for _, child := range result.Children {
		if child.WorkloadPolicy != WorkloadPolicyStrict {
			t.Fatalf("child %s policy = %q, want %q", child.ID, child.WorkloadPolicy, WorkloadPolicyStrict)
		}
	}
}

// seedPreviousSchema writes a delegation database stopped at the given ledger
// version. Only the ledger is always created; the pre-v15 work_items shape is
// added when withWorkItems is set so a legacy row can prove the ALTER backfill.
func seedPreviousSchema(t *testing.T, dbPath string, version int, withWorkItems bool) {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	defer func() { _ = raw.Close() }()

	statements := []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT`,
		`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-01-01T00:00:00Z')`,
	}
	if withWorkItems {
		statements = append(statements,
			`CREATE TABLE work_items (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('backlog','ready','in_progress','in_review','done','blocked')),
				revision INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				board_id TEXT NOT NULL DEFAULT 'default',
				workspace TEXT NOT NULL DEFAULT '',
				opencode_session_id TEXT NOT NULL DEFAULT '',
				opencode_root_session_id TEXT NOT NULL DEFAULT '',
				opencode_parent_session_id TEXT NOT NULL DEFAULT ''
			) STRICT`,
			`INSERT INTO work_items(id,title,status,created_at,updated_at) VALUES('legacy-task','legacy','ready','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	}
	for index, statement := range statements {
		if index == 1 {
			if _, err := raw.Exec(statement, version); err != nil {
				t.Fatalf("seed ledger version %d: %v", version, err)
			}
			continue
		}
		if _, err := raw.Exec(statement); err != nil {
			t.Fatalf("seed previous schema: %v", err)
		}
	}
}
