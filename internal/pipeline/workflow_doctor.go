package pipeline

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

func productionWorkflowDoctor(profile string, resolution forgespec.ForgeSpecResolution, plan sddinstall.Plan, bundles []TargetBundle, retirements []mcpinject.ConfigRetirement, homeDir string) verify.DoctorReport {
	input := verify.WorkflowDiagnosticInput{
		Profile: profile, Resolution: resolution,
		ExternalMailboxPaths: protectedMailboxPaths(homeDir),
	}
	for _, bundle := range bundles {
		for _, asset := range bundle.Bundle.Assets {
			if asset.Kind != renderers.AssetInstruction {
				continue
			}
			digest := sddinstall.SHA256(asset.Content)
			input.Instructions = append(input.Instructions, verify.InstructionContract{
				Target: string(bundle.Target), Path: asset.Path, Digest: digest, ExpectedDigest: digest,
				Size: len(asset.Content), MaximumSize: 128 * 1024,
				Precedence: "project-policy>user-content", ExpectedPrecedence: "project-policy>user-content",
			})
		}
	}
	for _, conflict := range plan.Conflicts {
		input.Additional = append(input.Additional, verify.Observation{
			Kind: verify.CheckOwnership, State: verify.StateUnknown, Target: "workflow", Path: conflict.Path,
			Observed: string(conflict.State), Expected: "clean managed ownership", Evidence: fmt.Sprintf("current=%s desired=%s", conflict.CurrentHash, conflict.DesiredHash),
			Remediation: "resolve the disclosed ownership conflict and prepare a new immutable plan",
		})
	}
	// Exact planned retirements are not blockers: apply consumes them from this
	// same fingerprint. Unplanned registrations are supplied by post-apply scans.
	_ = retirements
	return verify.DiagnoseWorkflow(input)
}
