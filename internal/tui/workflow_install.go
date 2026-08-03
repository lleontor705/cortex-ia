package tui

import (
	"fmt"
	"strings"

	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

// WorkflowInstallReview is the TUI projection of the same immutable plan and
// diagnostic evidence used by command-mode installation.
type WorkflowInstallReview struct {
	Plan       sddinstall.Plan
	Doctor     verify.DoctorReport
	Receipt    sddinstall.Receipt
	InstallErr error
}

// RenderWorkflowInstallReview keeps safety-relevant details visible before
// install and exposes explicit restoration guidance after a failed apply.
func RenderWorkflowInstallReview(review WorkflowInstallReview) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Profile: %s\n", review.Plan.Profile)
	for _, degradation := range review.Plan.Degradations {
		fmt.Fprintf(&output, "Degradation: %s\n", degradation)
	}
	for _, effect := range review.Plan.Creates {
		fmt.Fprintf(&output, "Create: %s\n", effect.Path)
	}
	for _, effect := range review.Plan.Updates {
		fmt.Fprintf(&output, "Update: %s\n", effect.Path)
	}
	for _, effect := range review.Plan.Deletes {
		fmt.Fprintf(&output, "Delete: %s\n", effect.Path)
	}
	for _, permission := range review.Plan.PermissionChanges {
		fmt.Fprintf(&output, "Permission: %s %04o -> %04o\n", permission.Path, permission.From.Perm(), permission.To.Perm())
	}
	if review.Plan.Backup.Required {
		fmt.Fprintf(&output, "Backup: %s\n", strings.Join(review.Plan.Backup.Paths, ", "))
	} else {
		fmt.Fprintln(&output, "Backup: not required")
	}
	for _, conflict := range review.Plan.Conflicts {
		fmt.Fprintf(&output, "Conflict: %s (%s)\n", conflict.Path, conflict.Reason)
	}
	for _, finding := range review.Doctor.Findings {
		fmt.Fprintf(&output, "Doctor: %s target=%s path=%s remediation=%s blocking=%t\n",
			finding.Code, finding.Target, finding.Path, finding.Remediation, finding.Blocking)
	}
	if review.InstallErr != nil || review.Plan.HasBlockingConflicts() || review.Doctor.Blockers() != 0 || !review.Doctor.Qualified || review.Doctor.Profile != review.Plan.Profile {
		fmt.Fprintln(&output, "Install blocked")
	}
	if review.InstallErr != nil && review.Receipt.RestoreAvailable && review.Receipt.BackupVerified && review.Receipt.Backup.ID != "" {
		fmt.Fprintf(&output, "Restoration available: %s\n", review.Receipt.Backup.ID)
		fmt.Fprintln(&output, "Rollback requires explicit selection")
	}
	return output.String()
}
