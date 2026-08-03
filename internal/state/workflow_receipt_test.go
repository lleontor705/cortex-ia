package state_test

import (
	"os"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestWorkflowReceiptStoreRejectsTampering(t *testing.T) {
	home := t.TempDir()
	store := state.NewWorkflowReceiptStore(home)
	receipt := install.Receipt{ID: "receipt-1", SchemaVersion: "1.0.0", State: install.ReceiptPrepared, PlanFingerprint: strings.Repeat("a", 64)}
	if err := store.Save(receipt); err != nil {
		t.Fatal(err)
	}
	path := state.WorkflowReceiptPath(home, receipt.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "prepared", "committed", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(receipt.ID); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered receipt error = %v", err)
	}
}
