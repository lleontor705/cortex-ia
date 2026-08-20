package install

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// InstallReceipt is the typed outcome of one install or sync execution. It
// separates what the run configured, which configured entries carried valid
// qualification evidence, what changed on disk, which conflicts blocked
// mutation, and which verified backup can restore the pre-run state.
type InstallReceipt struct {
	// DryRun reports that nothing was mutated.
	DryRun bool `json:"dry_run"`
	// Converged reports that the request needed zero writes.
	Converged bool `json:"converged"`
	// PlanDigest binds the run to exactly the planned operation set.
	PlanDigest string `json:"plan_digest"`
	// Plan is the full typed plan for front-end rendering.
	Plan *pipeline.Plan `json:"plan,omitempty"`
	// Configured lists the managed MCP entries the plan leaves configured
	// (added or already converged).
	Configured []string `json:"configured,omitempty"`
	// Qualified lists the configured entries that carried valid probe
	// evidence during this run. Dry-runs execute no probes, so a dry-run
	// reports no qualified entries.
	Qualified []string `json:"qualified,omitempty"`
	// Changed lists the applied mutations as "kind destination" strings.
	Changed []string `json:"changed,omitempty"`
	// Conflicts are the fail-closed blockers; nothing is mutated while
	// any conflict is present.
	Conflicts []pipeline.Conflict `json:"conflicts,omitempty"`
	// TransactionID identifies the engine transaction.
	TransactionID string `json:"transaction_id,omitempty"`
	// BackupID locates the verified pre-run backup.
	BackupID string `json:"backup_id,omitempty"`
	// BackupVerified reports the backup was proven restorable before any
	// mutation began.
	BackupVerified bool `json:"backup_verified"`
	// Restored reports that a failed apply completed a verified reverse
	// restoration of every preimage.
	Restored bool `json:"restored,omitempty"`
	// RestoreError describes a failed restoration; the journal is retained
	// on disk for a safe retry.
	RestoreError string   `json:"restore_error,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// newInstallReceipt projects the engine plan and receipt onto the service
// receipt, deriving the configured/qualified MCP separation from the planned
// effects and the engine's qualification warnings.
func newInstallReceipt(plan *pipeline.Plan, receipt *pipeline.Receipt) *InstallReceipt {
	out := &InstallReceipt{}
	if plan != nil {
		out.Plan = plan
		out.PlanDigest = plan.Digest
		for _, effect := range plan.Effects {
			switch effect.Kind {
			case pipeline.EffectMCPAdd, pipeline.EffectMCPNoop:
				out.Configured = append(out.Configured, effect.Dest)
			}
		}
	}
	if receipt == nil {
		return out
	}
	out.DryRun = receipt.DryRun
	out.Converged = receipt.Converged
	if out.DryRun {
		// Dry-runs intentionally never execute managed MCP probes; any qualified
		// list therefore reports zero successfully qualified entries.
		out.Qualified = nil
	}
	if out.PlanDigest == "" {
		out.PlanDigest = receipt.PlanDigest
	}
	out.Changed = receipt.Changes
	out.Conflicts = receipt.Conflicts
	out.TransactionID = receipt.TransactionID
	out.BackupID = receipt.BackupID
	out.BackupVerified = receipt.BackupVerified
	out.Restored = receipt.Restored
	out.RestoreError = receipt.RestoreError
	out.Warnings = receipt.Warnings
	if !out.DryRun {
		out.Qualified = qualifiedEntries(out.Configured, receipt.Warnings)
	}
	return out
}

// qualifiedEntries filters configured names against the engine's
// qualification warnings ("MCP %q configured without valid qualification
// evidence"). Entries the engine did not warn about carried valid evidence.
func qualifiedEntries(configured []string, warnings []string) []string {
	if len(configured) == 0 {
		return nil
	}
	qualified := make([]string, 0, len(configured))
	for _, name := range configured {
		if !flaggedUnqualified(name, warnings) {
			qualified = append(qualified, name)
		}
	}
	if len(qualified) == 0 {
		return nil
	}
	return qualified
}

func flaggedUnqualified(name string, warnings []string) bool {
	quoted := fmt.Sprintf("%q", name)
	for _, warning := range warnings {
		if strings.Contains(warning, quoted) && strings.Contains(warning, "qualification") {
			return true
		}
	}
	return false
}
