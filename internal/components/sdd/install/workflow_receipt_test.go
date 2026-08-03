package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingReceiptStore struct {
	receipts []Receipt
	failAt   int
}

func (s *recordingReceiptStore) Save(receipt Receipt) error {
	if s.failAt > 0 && len(s.receipts)+1 == s.failAt {
		return errors.New("injected receipt persistence failure")
	}
	s.receipts = append(s.receipts, receipt)
	return nil
}

func (s *recordingReceiptStore) Load(string) (Receipt, error) {
	if len(s.receipts) == 0 {
		return Receipt{}, os.ErrNotExist
	}
	return s.receipts[len(s.receipts)-1], nil
}

func TestApplyPersistsPreparedReceiptBeforeMutationAndCheckpointsOutcomes(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "a.txt", []byte("old-a"), 0o600)
	writeTarget(t, root, "b.txt", []byte("old-b"), 0o600)
	plan := Plan{
		Updates: []Effect{
			{Path: "a.txt", BeforeSHA256: SHA256([]byte("old-a")), AfterSHA256: SHA256([]byte("new-a")), Content: []byte("new-a"), AfterMode: 0o600},
			{Path: "b.txt", BeforeSHA256: SHA256([]byte("old-b")), AfterSHA256: SHA256([]byte("new-b")), Content: []byte("new-b"), AfterMode: 0o600},
		},
		Backup: BackupScope{Required: true, Paths: []string{"a.txt", "b.txt"}},
	}
	plan.Fingerprint = FingerprintPlan(plan)
	store := &recordingReceiptStore{}
	applier := NewApplier(root, filepath.Join(t.TempDir(), "backups"))
	applier.beforeMutation = func(receipt Receipt, _ Effect) error {
		if len(store.receipts) == 0 || store.receipts[0].State != ReceiptPrepared {
			t.Fatal("mutation began before durable PREPARED receipt")
		}
		return nil
	}

	receipt, err := applier.ApplyWithStore(plan, store)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != ReceiptCommitted || receipt.PlanFingerprint != plan.Fingerprint || len(receipt.OperationOutcomes) != 2 {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(store.receipts) != 4 { // PREPARED, two checkpoints, COMMITTED
		t.Fatalf("receipt checkpoints = %d, want 4", len(store.receipts))
	}
}

func TestApplyUsesDurableReceiptStoreByDefault(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "asset.txt", []byte("old"), 0o600)
	receipt, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(Plan{
		Updates: []Effect{{Path: "asset.txt", BeforeSHA256: SHA256([]byte("old")), Content: []byte("new"), AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{"asset.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".cortex-ia", "receipts", receipt.ID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default apply did not persist durable receipt: %v", err)
	}
}

func TestApplyReceiptFailurePrecedesTargetMutation(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "asset.txt", []byte("old"), 0o600)
	plan := Plan{
		Updates: []Effect{{Path: "asset.txt", BeforeSHA256: SHA256([]byte("old")), Content: []byte("new"), AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{"asset.txt"}},
	}
	plan.Fingerprint = FingerprintPlan(plan)
	_, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).ApplyWithStore(plan, &recordingReceiptStore{failAt: 1})
	if err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("error = %v, want receipt failure", err)
	}
	assertTarget(t, root, "asset.txt", "old")
}

func TestApplyWholePlanPreflightBlocksStaleTargetBeforeBackup(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(t.TempDir(), "backups")
	writeTarget(t, root, "asset.txt", []byte("changed-after-preview"), 0o600)
	plan := Plan{
		Updates: []Effect{{Path: "asset.txt", BeforeSHA256: SHA256([]byte("preview")), Content: []byte("new"), AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{"asset.txt"}},
	}
	plan.Fingerprint = FingerprintPlan(plan)
	_, err := NewApplier(root, backupRoot).Apply(plan)
	if !errors.Is(err, ErrStalePlan) {
		t.Fatalf("error = %v, want ErrStalePlan", err)
	}
	if _, statErr := os.Stat(backupRoot); !os.IsNotExist(statErr) {
		t.Fatalf("stale plan created backup: %v", statErr)
	}
	assertTarget(t, root, "asset.txt", "changed-after-preview")
}

func TestApplyPerEffectGuardClosesPreflightRace(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "a.txt", []byte("old-a"), 0o600)
	writeTarget(t, root, "b.txt", []byte("old-b"), 0o600)
	plan := Plan{
		Updates: []Effect{
			{Path: "a.txt", BeforeSHA256: SHA256([]byte("old-a")), Content: []byte("new-a"), AfterMode: 0o600},
			{Path: "b.txt", BeforeSHA256: SHA256([]byte("old-b")), Content: []byte("new-b"), AfterMode: 0o600},
		},
		Backup: BackupScope{Required: true, Paths: []string{"a.txt", "b.txt"}},
	}
	plan.Fingerprint = FingerprintPlan(plan)
	applier := NewApplier(root, filepath.Join(t.TempDir(), "backups"))
	applier.beforeMutation = func(_ Receipt, effect Effect) error {
		if effect.Path == "b.txt" {
			writeTarget(t, root, "b.txt", []byte("raced"), 0o600)
		}
		return nil
	}
	receipt, err := applier.Apply(plan)
	if !errors.Is(err, ErrStalePlan) || receipt.FailedPath != "b.txt" {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
	assertTarget(t, root, "a.txt", "new-a")
	assertTarget(t, root, "b.txt", "raced")
}

func TestApplyPreflightChecksOwnershipAndBaseCAS(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, root, "asset.txt", []byte("old"), 0o600)
	writeTarget(t, root, "asset.txt.cortex-ia.json", []byte("changed-ownership"), 0o600)
	writeTarget(t, root, "asset.txt.cortex-ia.base", []byte("base"), 0o600)
	plan := Plan{
		Updates: []Effect{{
			Path: "asset.txt", BeforeSHA256: SHA256([]byte("old")), Content: []byte("new"), AfterMode: 0o600,
			OwnershipSHA256: SHA256([]byte("preview-ownership")), BaseSHA256: SHA256([]byte("base")),
		}},
		Backup: BackupScope{Required: true, Paths: []string{"asset.txt", "asset.txt.cortex-ia.json", "asset.txt.cortex-ia.base"}},
	}
	plan.Fingerprint = FingerprintPlan(plan)
	_, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(plan)
	if !errors.Is(err, ErrStalePlan) || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("error = %v, want stale ownership evidence", err)
	}
	assertTarget(t, root, "asset.txt", "old")
}

func TestRollbackRestoredLegacyMailboxIsDiagnosed(t *testing.T) {
	root := t.TempDir()
	path := "opencode.json"
	prior := []byte(`{"mcp":{"agent-mailbox":{"command":["npx","agent-mailbox-mcp"]}}}`)
	installed := []byte(`{"mcp":{}}`)
	writeTarget(t, root, path, prior, 0o600)
	receipt, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(Plan{
		Updates: []Effect{{Path: path, SemanticID: "retirement/agent-mailbox-registration", BeforeSHA256: SHA256(prior), Content: installed, AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Rollback(receipt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) == 0 || !strings.Contains(result.Findings[0], "legacy/unqualified") {
		t.Fatalf("missing legacy diagnosis: %+v", result)
	}
	assertTarget(t, root, path, string(prior))
}

func TestBuildInversePlanRestoresPriorManagedBytes(t *testing.T) {
	root := t.TempDir()
	path := "agent.md"
	prior := []byte("prior")
	installed := []byte("installed")
	writeTarget(t, root, path, prior, 0o600)
	receipt, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(Plan{
		Updates: []Effect{{Path: path, SemanticID: "asset/agent/test", BeforeSHA256: SHA256(prior), Content: installed, AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	inverse, err := BuildInversePlan(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if len(inverse.Updates) != 1 || string(inverse.Updates[0].Content) != string(prior) || inverse.Updates[0].BeforeSHA256 != SHA256(installed) {
		t.Fatalf("inverse plan = %+v", inverse)
	}
	if inverse.Fingerprint == "" {
		t.Fatal("inverse plan has no immutable fingerprint")
	}
}
