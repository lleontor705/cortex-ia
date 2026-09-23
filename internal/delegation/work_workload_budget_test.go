package delegation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkloadBudgetClassifiesChurn(t *testing.T) {
	churn := parseNumStat(strings.Join([]string{
		"100\t200\tsrc/app.ts",
		"200\t100\tcmd/main.go",
		"5\t5\tdb/migrations/0001.sql",
		"9\t9\tconfig/app.json",
		"7\t7\tdeploy/app.yaml",
		"-\t-\tassets/logo.png",
		"",
	}, "\n"))
	if churn.scriptSource != 140 || churn.plainSource != 300 || churn.test != 0 {
		t.Fatalf("churn = %+v, want script 140, plain 300, test 0", churn)
	}
}

func TestWorkloadBudgetRoutesTestChurn(t *testing.T) {
	churn := parseNumStat("120\t30\tinternal/x/foo_test.go\n100\t0\tweb/src/bar.test.ts")
	if churn.test != 250 || churn.plainSource != 0 || churn.scriptSource != 0 {
		t.Fatalf("churn = %+v, want test 250 only", churn)
	}
}

func TestWorkloadBudgetCapsTrackPolicy(t *testing.T) {
	strict, ok := workloadCapsFor(WorkloadPolicyStrict)
	if !ok || strict.plainSource != 350 || strict.scriptSource != 250 || strict.test != 600 {
		t.Fatalf("strict caps = %+v ok=%v", strict, ok)
	}
	flexible, ok := workloadCapsFor(WorkloadPolicyFlexible)
	if !ok || flexible.plainSource != 700 || flexible.scriptSource != 500 || flexible.test != 1200 {
		t.Fatalf("flexible caps = %+v ok=%v", flexible, ok)
	}
	if _, ok := workloadCapsFor(WorkloadPolicyUnbounded); ok {
		t.Fatal("unbounded must not carry enforceable caps")
	}
}

func TestWorkloadBudgetStrictRefusesOversizedSource(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("src/a.go", 800)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-strict-source", WorkloadPolicyStrict); !errors.Is(err, ErrWorkloadSourceBudgetExceeded) {
		t.Fatalf("err = %v, want WORKLOAD_SOURCE_BUDGET_EXCEEDED", err)
	}
	item, err := store.GetWork(context.Background(), "wl-strict-source")
	if err != nil {
		t.Fatalf("read refused task: %v", err)
	}
	if item.Status != WorkInProgress {
		t.Fatalf("status = %s, want in_progress", item.Status)
	}
	var reviews int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_reviews WHERE item_id='wl-strict-source'`).Scan(&reviews); err != nil {
		t.Fatalf("count reviews: %v", err)
	}
	if reviews != 0 {
		t.Fatalf("review rows = %d, want 0", reviews)
	}
}

func TestWorkloadBudgetStrictRefusesOversizedTests(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("src/a_test.go", 700)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-strict-tests", WorkloadPolicyStrict); !errors.Is(err, ErrWorkloadTestBudgetExceeded) {
		t.Fatalf("err = %v, want WORKLOAD_TEST_BUDGET_EXCEEDED", err)
	}
}

func TestWorkloadBudgetStrictAcceptsChurnWithinCap(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("src/a.go", 300)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-within-cap", WorkloadPolicyStrict); err != nil {
		t.Fatalf("300 Go lines fit the 350 strict cap: %v", err)
	}
}

func TestWorkloadBudgetStrictExcludesDeclarativeChurn(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("config/app.json", 900)
	grow("db/schema.sql", 900)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-declarative", WorkloadPolicyStrict); err != nil {
		t.Fatalf("declarative churn is exempt from both budgets: %v", err)
	}
}

func TestWorkloadBudgetStrictIgnoresUntrackedChurn(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("src/untracked.go", 900)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-untracked", WorkloadPolicyStrict); err != nil {
		t.Fatalf("untracked files are outside git diff HEAD and cannot block: %v", err)
	}
}

func TestWorkloadBudgetFlexibleRecordsAdvisory(t *testing.T) {
	store := workloadBudgetStore(t)
	dir, grow := workloadGitWorkspace(t)
	grow("src/a.go", 900)

	if _, err := workloadBudgetTransition(t, store, dir, "wl-flexible", WorkloadPolicyFlexible); err != nil {
		t.Fatalf("flexible overage must not block: %v", err)
	}
	var detail string
	if err := store.db.QueryRow(`SELECT detail FROM work_events WHERE item_id='wl-flexible' AND kind='transition'`).Scan(&detail); err != nil {
		t.Fatalf("read transition event: %v", err)
	}
	if !strings.Contains(detail, "WORKLOAD_ADVISORY") {
		t.Fatalf("transition detail %q lacks WORKLOAD_ADVISORY", detail)
	}
}

func TestWorkloadBudgetStrictNonGitFailsClosed(t *testing.T) {
	store := workloadBudgetStore(t)

	if _, err := workloadBudgetTransition(t, store, t.TempDir(), "wl-nongit", WorkloadPolicyStrict); !errors.Is(err, ErrWorkloadBudgetUnverifiable) {
		t.Fatalf("err = %v, want WORKLOAD_BUDGET_UNVERIFIABLE", err)
	}
}

func TestWorkloadBudgetUnboundedSkipsChurnCommand(t *testing.T) {
	store := workloadBudgetStore(t)
	original := workloadNumStatCommand
	t.Cleanup(func() { workloadNumStatCommand = original })
	workloadNumStatCommand = func(context.Context, string) ([]byte, error) {
		t.Fatal("unbounded policy must not compute churn")
		return nil, nil
	}

	if _, err := workloadBudgetTransition(t, store, t.TempDir(), "wl-unbounded", WorkloadPolicyUnbounded); err != nil {
		t.Fatalf("unbounded transition failed: %v", err)
	}
}

func workloadBudgetStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(context.Background(), "workload-budget-board", "Workload Budget Board", ""); err != nil {
		t.Fatalf("create board: %v", err)
	}
	return store
}

func workloadGitWorkspace(t *testing.T) (string, func(string, int)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required to build churn workspaces")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	grow := func(rel string, lines int) {
		var body strings.Builder
		for i := 0; i < lines; i++ {
			fmt.Fprintf(&body, "line %d\n", i)
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s dir: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	grow("src/a.go", 1)
	grow("src/a_test.go", 1)
	grow("config/app.json", 1)
	grow("db/schema.sql", 1)
	run("init", "-q")
	run("add", "-A")
	run("commit", "-qm", "base")
	return dir, grow
}

func workloadBudgetTransition(t *testing.T, store *Store, workspace, id string, policy WorkloadPolicy) (WorkItem, error) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.CreateWorkInBoardWithDefinition(ctx, "workload-budget-board", id, id+" title", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification",
		AllowedFiles: []string{"src/a.go"}, WorkloadPolicy: policy,
	}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
	claim, err := store.ClaimWork(ctx, id, "impl", time.Minute)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}
	return store.TransitionWork(ctx, id, claim.Token, claim.Revision, WorkInReview)
}
