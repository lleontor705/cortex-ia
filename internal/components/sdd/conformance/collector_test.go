package conformance

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectRepositoryDerivesAbsenceEvidenceFromFiles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "internal/current.go", "package current\n// route through team-lead\n")
	writeFixture(t, root, "docs/migration.md", "agent-mailbox is retired historical data\n")
	writeFixture(t, root, "release/bundle.txt", "provider tool agent-mailbox_msg_send\n")
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	allowances := []LegacyAllowance{{
		Path: "docs/migration.md", Pattern: "agent-mailbox", Reason: "retirement migration guidance",
		Owner: "release-engineering", Classification: AllowanceHistorical, ReviewBy: now.AddDate(0, 1, 0),
	}}

	evidence, err := CollectRepository(CollectorRequest{Root: root, Allowances: allowances, Now: now, Budget: 100})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != EvidenceFailed || len(evidence.Violations) != 2 {
		t.Fatalf("evidence status=%s violations=%+v", evidence.Status, evidence.Violations)
	}
	if evidence.FilesScanned != 3 || evidence.Digest == "" || evidence.Command != "repository-source-scan" || evidence.CWD == "" || evidence.Tree == "" || evidence.StartedAt.IsZero() || evidence.FinishedAt.IsZero() {
		t.Fatalf("incomplete source-derived evidence: %+v", evidence)
	}
	if evidence.Occurrences != 3 || evidence.AllowedOccurrences != 1 {
		t.Fatalf("occurrence inventory = %d/%d", evidence.AllowedOccurrences, evidence.Occurrences)
	}
}

func TestCollectRepositoryRejectsUngovernedOrExpiredAllowances(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/history.md", "agent-mailbox retired\n")
	now := time.Now().UTC()
	for _, tt := range []struct {
		name      string
		allowance LegacyAllowance
	}{
		{name: "path only", allowance: LegacyAllowance{Path: "docs/history.md"}},
		{name: "expired", allowance: LegacyAllowance{Path: "docs/history.md", Pattern: "agent-mailbox", Reason: "history", Owner: "docs", Classification: AllowanceHistorical, ReviewBy: now.Add(-time.Hour)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CollectRepository(CollectorRequest{Root: root, Allowances: []LegacyAllowance{tt.allowance}, Now: now, Budget: 10}); err == nil {
				t.Fatal("CollectRepository() accepted ungoverned allowance")
			}
		})
	}
}

func TestCollectRepositoryBudgetExhaustionIsInconclusiveNotPass(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.txt", "clean\n")
	writeFixture(t, root, "b.txt", "clean\n")
	evidence, err := CollectRepository(CollectorRequest{Root: root, Now: time.Now().UTC(), Budget: 1})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != EvidenceInconclusive || evidence.Complete {
		t.Fatalf("budget exhaustion was represented as pass: %+v", evidence)
	}
}

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
