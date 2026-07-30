package app

import (
	"errors"
	"fmt"
	"io"
	"strings"

	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

// WorkflowInstallCommand wires the exact read-only plan and doctor evidence to
// its mutation boundary. Rollback remains a separate explicit command.
type WorkflowInstallCommand struct {
	DryRun bool
	Plan   sddinstall.Plan
	Doctor verify.DoctorReport
	Apply  func(sddinstall.Plan) (sddinstall.Receipt, error)
}

// RunWorkflowInstall discloses the exact plan before either returning from a
// dry-run or passing that same value to Apply.
func RunWorkflowInstall(writer io.Writer, command WorkflowInstallCommand) (sddinstall.Receipt, error) {
	WriteWorkflowInstallPlan(writer, command.Plan)
	WriteWorkflowDoctor(writer, command.Doctor)
	if command.Plan.HasBlockingConflicts() {
		return sddinstall.Receipt{}, fmt.Errorf("install blocked by %d conflict(s)", len(command.Plan.Conflicts))
	}
	if blockers := command.Doctor.Blockers(); blockers != 0 {
		return sddinstall.Receipt{}, fmt.Errorf("install blocked by %d doctor finding(s)", blockers)
	}
	if !command.Doctor.Qualified {
		return sddinstall.Receipt{}, errors.New("install blocked: doctor did not qualify the selected profile")
	}
	if command.Doctor.Profile != command.Plan.Profile {
		return sddinstall.Receipt{}, fmt.Errorf("install blocked: doctor profile %q does not match planned profile %q", command.Doctor.Profile, command.Plan.Profile)
	}
	if command.DryRun {
		_, _ = fmt.Fprintln(writer, "DRY RUN: zero persistent mutations")
		return sddinstall.Receipt{}, nil
	}
	if command.Apply == nil {
		return sddinstall.Receipt{}, errors.New("workflow install apply boundary is required")
	}

	receipt, err := command.Apply(command.Plan)
	if err == nil {
		return receipt, nil
	}
	_, _ = fmt.Fprintf(writer, "Install failed: %v\n", err)
	if receipt.RestoreAvailable && receipt.BackupVerified && strings.TrimSpace(receipt.Backup.ID) != "" {
		_, _ = fmt.Fprintf(writer, "Restoration available: cortex-ia rollback --backup %s\n", receipt.Backup.ID)
		_, _ = fmt.Fprintln(writer, "Rollback requires explicit operator selection; restoration was not run automatically.")
	}
	return receipt, err
}

// WriteWorkflowInstallPlan renders every planned effect, profile decision,
// degradation, backup target, permission delta, and blocking conflict.
func WriteWorkflowInstallPlan(writer io.Writer, plan sddinstall.Plan) {
	_, _ = fmt.Fprintf(writer, "Profile: %s\n", plan.Profile)
	for _, degradation := range plan.Degradations {
		_, _ = fmt.Fprintf(writer, "Degradation: %s\n", degradation)
	}
	for _, effect := range plan.Creates {
		_, _ = fmt.Fprintf(writer, "CREATE %s semantic=%s after=%s\n", effect.Path, effect.SemanticID, effect.AfterSHA256)
	}
	for _, effect := range plan.Updates {
		_, _ = fmt.Fprintf(writer, "UPDATE %s semantic=%s before=%s after=%s\n", effect.Path, effect.SemanticID, effect.BeforeSHA256, effect.AfterSHA256)
	}
	for _, effect := range plan.Deletes {
		_, _ = fmt.Fprintf(writer, "DELETE %s semantic=%s before=%s\n", effect.Path, effect.SemanticID, effect.BeforeSHA256)
	}
	for _, change := range plan.PermissionChanges {
		_, _ = fmt.Fprintf(writer, "PERMISSION %s %04o -> %04o\n", change.Path, change.From.Perm(), change.To.Perm())
	}
	if plan.Backup.Required {
		_, _ = fmt.Fprintf(writer, "BACKUP required: %s\n", strings.Join(plan.Backup.Paths, ", "))
	} else {
		_, _ = fmt.Fprintln(writer, "BACKUP not required")
	}
	for _, conflict := range plan.Conflicts {
		_, _ = fmt.Fprintf(writer, "CONFLICT %s semantic=%s state=%s reason=%s current=%s desired=%s\n",
			conflict.Path, conflict.SemanticID, conflict.State, conflict.Reason, conflict.CurrentHash, conflict.DesiredHash)
	}
	if len(plan.Conflicts) != 0 {
		_, _ = fmt.Fprintf(writer, "Install blocked: %d conflict(s)\n", len(plan.Conflicts))
	}
}

// WriteWorkflowDoctor exposes the complete stable finding contract rather than
// reducing diagnostics to a pass/fail count.
func WriteWorkflowDoctor(writer io.Writer, report verify.DoctorReport) {
	_, _ = fmt.Fprintf(writer, "Profile: %s\n", report.Profile)
	_, _ = fmt.Fprintf(writer, "Qualified: %t\n", report.Qualified)
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(writer, "%s severity=%v target=%s path=%s observed=%s expected=%s evidence=%s remediation=%s blocking=%t\n",
			finding.Code, finding.Severity, finding.Target, finding.Path, finding.Observed, finding.Expected,
			finding.Evidence, finding.Remediation, finding.Blocking)
	}
}

// RunWorkflowRollback requires the operator to select one backup explicitly.
// The callback owns receipt lookup and the core three-way rollback operation.
func RunWorkflowRollback(writer io.Writer, backupID string, rollback func(string) (sddinstall.RollbackResult, error)) (sddinstall.RollbackResult, error) {
	backupID = strings.TrimSpace(backupID)
	if backupID == "" {
		return sddinstall.RollbackResult{}, errors.New("rollback requires explicit --backup <id> selection")
	}
	if rollback == nil {
		return sddinstall.RollbackResult{}, errors.New("workflow rollback boundary is required")
	}
	_, _ = fmt.Fprintf(writer, "Selected backup: %s\n", backupID)
	result, err := rollback(backupID)
	for _, path := range result.Restored {
		_, _ = fmt.Fprintf(writer, "RESTORED %s\n", path)
	}
	for _, conflict := range result.Conflicts {
		_, _ = fmt.Fprintf(writer, "ROLLBACK CONFLICT %s semantic=%s prior=%s installed=%s current=%s\n",
			conflict.Path, conflict.SemanticID, conflict.PriorRef, conflict.InstalledRef, conflict.CurrentRef)
	}
	_, _ = fmt.Fprintf(writer, "Doctor qualified restored bundle: %t\n", result.DoctorPassed)
	return result, err
}
