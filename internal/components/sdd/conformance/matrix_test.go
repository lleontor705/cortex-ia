package conformance

import (
	"context"
	"testing"
)

func TestAdapterProfileMatrixIsCompleteAndExplicit(t *testing.T) {
	receipt, err := NewRuntimeHarness(RuntimeHarnessConfig{WorkDir: t.TempDir()}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(receipt.Cells); got != 3 {
		t.Fatalf("matrix cells = %d, want 3", got)
	}
	if got := len(receipt.Adapters) * len(receipt.Profiles); got != 3 {
		t.Fatalf("cartesian cardinality = %d, want 3", got)
	}
	for _, cell := range receipt.Cells {
		if cell.RequestedProfile == "" || cell.EffectiveProfile == "" || cell.ReasonID == "" {
			t.Fatalf("incomplete cell evidence: %+v", cell)
		}
		if cell.Command == "" || cell.ExitCode < 0 || cell.ReceiptDigest == "" || cell.Evidence == nil {
			t.Fatalf("incomplete execution evidence: %+v", cell)
		}
		if (cell.Disposition == DispositionRejected || cell.Disposition == DispositionBlocked) && cell.Evidence["mutation"] != "none" {
			t.Fatalf("rejected cell mutated: %+v", cell)
		}
	}
	if receipt.Digest == "" {
		t.Fatal("runtime receipt digest is empty")
	}
}
