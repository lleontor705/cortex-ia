package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/install"
)

const workflowReceiptsDir = "receipts"

type WorkflowReceipt = install.Receipt

type WorkflowReceiptStore struct{ homeDir string }

func NewWorkflowReceiptStore(homeDir string) WorkflowReceiptStore {
	return WorkflowReceiptStore{homeDir: homeDir}
}

func WorkflowReceiptPath(homeDir, id string) string {
	return filepath.Join(BaseDir(homeDir), workflowReceiptsDir, id+".json")
}

func (s WorkflowReceiptStore) Save(receipt install.Receipt) error {
	if err := validateReceiptID(receipt.ID); err != nil {
		return err
	}
	receipt = install.SealReceipt(receipt)
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow receipt: %w", err)
	}
	encoded = append(encoded, '\n')
	path := WorkflowReceiptPath(s.homeDir, receipt.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create workflow receipt directory: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return fmt.Errorf("write workflow receipt: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit workflow receipt: %w", err)
	}
	return nil
}

func (s WorkflowReceiptStore) Load(id string) (install.Receipt, error) {
	if err := validateReceiptID(id); err != nil {
		return install.Receipt{}, err
	}
	path := WorkflowReceiptPath(s.homeDir, id)
	encoded, err := os.ReadFile(path)
	if err != nil {
		return install.Receipt{}, fmt.Errorf("read workflow receipt: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var receipt install.Receipt
	if err := decoder.Decode(&receipt); err != nil {
		return install.Receipt{}, fmt.Errorf("decode workflow receipt: %w", err)
	}
	if err := install.ValidateReceipt(receipt); err != nil {
		return install.Receipt{}, err
	}
	return receipt, nil
}

func validateReceiptID(id string) error {
	if id == "" || strings.ContainsAny(id, `/\\:`) || id == "." || id == ".." {
		return fmt.Errorf("invalid workflow receipt ID %q", id)
	}
	return nil
}
