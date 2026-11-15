package delegation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestApproveWorkRejectsPassWithoutEvidence(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	_, err = s.ApproveWork(context.Background(), "missing-task", "reviewer", "PASS", "   ", 1)
	if err == nil || !strings.Contains(err.Error(), "EVIDENCE_REQUIRED") {
		t.Fatalf("PASS without evidence error = %v, want EVIDENCE_REQUIRED", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM work_approvals`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected PASS wrote %d approval rows", count)
	}
}
