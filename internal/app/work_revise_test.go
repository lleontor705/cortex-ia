package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestWorkReviseAppliesPlanAndPrintsUpdatedTask(t *testing.T) {
	home, workspace, item := setupWorkReviseTask(t)
	pin := "openspec/changes/c/plan.md"
	writeAppRevisePin(t, workspace, pin, "new")
	plan := appRevisePlan(workspace, item, appReviseContract(workspace, pin, []byte("new")))
	planFile := writeJSONPlan(t, home, plan)

	out, err := captureStdout(func() error { return runWork([]string{"revise", "--plan", planFile}) })
	if err != nil {
		t.Fatalf("work revise failed: %v", err)
	}
	var got delegation.WorkItem
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode output %q: %v", out, err)
	}
	if got.ID != item.ID || got.Revision != item.Revision+1 || got.Title != "revised title" || got.Objective != "revised objective" || got.Status != delegation.WorkReady {
		t.Fatalf("unexpected revised item: %+v", got)
	}
}

func TestWorkReviseRejectsUnknownFieldsTrailingOversizeAndArgs(t *testing.T) {
	home, workspace, item := setupWorkReviseTask(t)
	pin := "openspec/changes/c/plan.md"
	base := appRevisePlan(workspace, item, appReviseContract(workspace, pin, []byte("old")))
	before := appReviseSnapshot(t, home, item.ID)

	cases := []string{
		`{"version":1,"task_id":"app-ready","board_id":"app-board","project":".","expected_revision":1,"expected_status":"ready","extra":true,"definition":{}}`,
		`{"version":1,"task_id":"app-ready","board_id":"app-board","project":".","expected_revision":1,"expected_status":"ready","definition":{"title":"t","objective":"o","acceptance_criteria":"a","verification":"v","allowed_files":[],"extra":true,"sdd_contract":null}}`,
		`{"version":1,"task_id":"app-ready","board_id":"app-board","project":".","expected_revision":1,"expected_status":"ready","definition":{"title":"t","objective":"o","acceptance_criteria":"a","verification":"v","allowed_files":[],"sdd_contract":{"version":1,"workflow":"sdd-lite","change_id":"c","spec_plane":"hybrid","pins":[],"requirement_ids":[],"extra":true}}}`,
		mustJSON(t, base) + ` {}`,
	}
	for i, body := range cases {
		planFile := filepath.Join(home, "bad-"+string(rune('a'+i))+".json")
		if err := os.WriteFile(planFile, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := captureStdout(func() error { return runWork([]string{"revise", "--plan", planFile}) }); err == nil {
			t.Fatalf("case %d unexpectedly succeeded", i)
		}
		appAssertSameSnapshot(t, before, appReviseSnapshot(t, home, item.ID))
	}

	tooLarge := filepath.Join(home, "too-large.json")
	if err := os.WriteFile(tooLarge, bytes.Repeat([]byte("x"), maxWorkRevisionPlanBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(func() error { return runWork([]string{"revise", "--plan", tooLarge}) }); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversize failure, got %v", err)
	}
	if _, err := captureStdout(func() error {
		return runWork([]string{"revise", "--plan", writeJSONPlan(t, home, base), "--unexpected", "x"})
	}); err == nil {
		t.Fatal("expected unrecognized argument failure")
	}
	appAssertSameSnapshot(t, before, appReviseSnapshot(t, home, item.ID))
}

func TestWorkReviseRejectsIdentityStatusRevisionAndSDDDropWithoutMutation(t *testing.T) {
	home, workspace, item := setupWorkReviseTask(t)
	before := appReviseSnapshot(t, home, item.ID)
	pin := "openspec/changes/c/plan.md"
	for name, mutate := range map[string]func(*delegation.WorkRevisionPlan){
		"missing identity": func(p *delegation.WorkRevisionPlan) { p.BoardID = "" },
		"stale revision":   func(p *delegation.WorkRevisionPlan) { p.ExpectedRevision++ },
		"wrong status":     func(p *delegation.WorkRevisionPlan) { p.ExpectedStatus = delegation.WorkBacklog },
		"sdd drop":         func(p *delegation.WorkRevisionPlan) { p.Definition.Contract = nil },
	} {
		plan := appRevisePlan(workspace, item, appReviseContract(workspace, pin, []byte("old")))
		mutate(&plan)
		if _, err := captureStdout(func() error { return runWork([]string{"revise", "--plan", writeJSONPlan(t, home, plan)}) }); err == nil {
			t.Fatalf("%s unexpectedly succeeded", name)
		}
		appAssertSameSnapshot(t, before, appReviseSnapshot(t, home, item.ID))
	}
}

func setupWorkReviseTask(t *testing.T) (string, string, delegation.WorkItem) {
	t.Helper()
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBoard(context.Background(), "app-board", "App Board", ""); err != nil {
		t.Fatal(err)
	}
	pin := "openspec/changes/c/plan.md"
	writeAppRevisePin(t, workspace, pin, "old")
	item, err := store.CreateWorkInBoardWithDefinition(context.Background(), "app-board", "app-ready", "old title", nil, delegation.WorkDefinition{Project: workspace, Objective: "old objective", Acceptance: "old acceptance", Verification: "old verification", AllowedFiles: []string{"old.go"}, Contract: appReviseContract(workspace, pin, []byte("old"))})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return home, workspace, item
}

func appRevisePlan(workspace string, item delegation.WorkItem, contract *delegation.SDDContract) delegation.WorkRevisionPlan {
	return delegation.WorkRevisionPlan{Version: 1, TaskID: item.ID, BoardID: item.BoardID, Project: workspace, ExpectedRevision: item.Revision, ExpectedStatus: item.Status, ExpectedWorkflow: "sdd-lite", ExpectedChangeID: "c", ExpectedSpecPlane: "hybrid", Definition: delegation.WorkRevisionDefinition{Title: "revised title", Objective: "revised objective", Acceptance: "revised acceptance", Verification: "revised verification", AllowedFiles: []string{"new.go"}, Contract: contract}}
}

func appReviseContract(workspace, locator string, content []byte) *delegation.SDDContract {
	sum := sha256.Sum256(content)
	return &delegation.SDDContract{Version: 1, Workflow: "sdd-lite", ChangeID: "c", SpecPlane: "hybrid", Pins: []delegation.ContractPin{{Transport: "workspace_file", Project: workspace, Locator: locator, SHA256: hex.EncodeToString(sum[:])}}, RequirementIDs: []string{"REQ-TEST-001"}}
}

func writeAppRevisePin(t *testing.T, workspace, locator, content string) {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(locator))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeJSONPlan(t *testing.T, dir string, plan delegation.WorkRevisionPlan) string {
	t.Helper()
	path := filepath.Join(dir, "plan-"+plan.TaskID+".json")
	if err := os.WriteFile(path, []byte(mustJSON(t, plan)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type appSnapshot struct {
	Title, Objective, Acceptance, Verification string
	AllowedFiles                               string
	Contract                                   string
	Revision                                   int64
}

func appReviseSnapshot(t *testing.T, home, id string) appSnapshot {
	t.Helper()
	store, err := delegation.OpenStoreReadOnly(delegation.DefaultDBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	item, err := store.GetWork(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	files, err := json.Marshal(item.AllowedFiles)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := json.Marshal(item.Contract)
	if err != nil {
		t.Fatal(err)
	}
	return appSnapshot{Title: item.Title, Objective: item.Objective, Acceptance: item.Acceptance, Verification: item.Verification, AllowedFiles: string(files), Contract: string(contract), Revision: item.Revision}
}

func appAssertSameSnapshot(t *testing.T, want, got appSnapshot) {
	t.Helper()
	if want != got {
		t.Fatalf("snapshot changed\nwant: %+v\n got: %+v", want, got)
	}
}

func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	return buf.String(), runErr
}
