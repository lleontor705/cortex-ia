package pipeline

import (
	"errors"

	"github.com/lleontor705/cortex-ia/internal/backup"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
)

// ApplyPlannedInstall is the pipeline boundary for the exact SDD install plan.
// Planning stays read-only; this call owns backup verification and mutation.
func ApplyPlannedInstall(targetRoot, backupRoot string, plan sddinstall.Plan) (sddinstall.Receipt, error) {
	return sddinstall.NewApplier(targetRoot, backupRoot).Apply(plan)
}

// RestorePlannedInstall restores a failed or superseded apply from its verified
// receipt. Verification is repeated so corrupted recovery evidence is never
// applied to live targets.
func RestorePlannedInstall(receipt sddinstall.Receipt) error {
	if !receipt.RestoreAvailable || !receipt.BackupVerified || receipt.Backup.ID == "" {
		return errors.New("planned install receipt has no verified restoration")
	}
	_, err := sddinstall.Rollback(receipt, nil)
	return err
}

// RestorePlannedInstallWithDoctor restores with customization-preserving
// three-way merge and qualifies the resulting bundle before reporting success.
func RestorePlannedInstallWithDoctor(receipt sddinstall.Receipt, doctor func() error) (sddinstall.RollbackResult, error) {
	if !receipt.RestoreAvailable || !receipt.BackupVerified || receipt.Backup.ID == "" {
		return sddinstall.RollbackResult{}, errors.New("planned install receipt has no verified restoration")
	}
	if err := backup.Verify(receipt.Backup); err != nil {
		return sddinstall.RollbackResult{}, err
	}
	return sddinstall.Rollback(receipt, doctor)
}
