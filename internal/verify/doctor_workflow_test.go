package verify

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestDiagnoseWorkflowEmitsStableBlockingAndAdvisoryFindings(t *testing.T) {
	report := DiagnoseWorkflow(WorkflowDiagnosticInput{
		Profile: "portable-sequential",
		Resolution: forgespec.ForgeSpecResolution{
			Mode: forgespec.CoordinationBlocked,
			Missing: []forgespec.CapabilityRequirement{{
				ID:       "forgespec/task-cas",
				Versions: ir.VersionRange{Minimum: ir.MustParseVersion("1.0.0"), MaximumTested: ir.MustParseVersion("1.0.0")},
			}},
			Degradations: []forgespec.Degradation{{CapabilityID: "forgespec/file-lease", Reason: "optional P1 capability is not qualified"}},
		},
		RetiredRegistrations: []DiagnosticTarget{{Target: "opencode", Path: "opencode.json", Observed: "agent-mailbox", Evidence: "sha256:retired"}},
		ExternalMailboxPaths: []string{"~/.agent-mailbox/mailbox.db"},
		Instructions: []InstructionContract{{
			Target: "claude", Path: "CLAUDE.md", Digest: "bad", ExpectedDigest: "good",
			Size: 9000, MaximumSize: 8000, Precedence: "user>project", ExpectedPrecedence: "project>user",
		}},
	})

	if report.Qualified || report.Blockers() != 6 {
		t.Fatalf("report qualified=%t blockers=%d findings=%+v", report.Qualified, report.Blockers(), report.Findings)
	}
	want := map[FindingCode]bool{
		FindingForgeSpecMode: false, FindingCapabilityMissing: false, FindingRetiredRegistration: false,
		FindingExternalData: false, FindingP1Degradation: false, FindingInstructionDigest: false,
		FindingInstructionSize: false, FindingInstructionPrecedence: false,
	}
	for _, finding := range report.Findings {
		if _, ok := want[finding.Code]; ok {
			want[finding.Code] = true
		}
		if finding.Target == "" || finding.Path == "" || finding.Observed == "" || finding.Expected == "" || finding.Evidence == "" || finding.Remediation == "" {
			t.Errorf("finding lacks actionable evidence: %+v", finding)
		}
		if finding.Code == FindingExternalData && finding.Blocking {
			t.Errorf("protected external data finding must be informational: %+v", finding)
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("missing stable finding %s", code)
		}
	}
}

func TestDiagnoseWorkflowHealthyQualifiedStateHasZeroBlockers(t *testing.T) {
	now := time.Now().UTC()
	report := DiagnoseWorkflow(WorkflowDiagnosticInput{
		Profile: "portable-sequential",
		Resolution: forgespec.ForgeSpecResolution{
			Mode:     forgespec.CoordinationDirectV1,
			Snapshot: forgespec.CapabilitySnapshot{ProbeStatus: forgespec.ProbeQualified},
		},
		Instructions: []InstructionContract{{
			Target: "codex", Path: "AGENTS.md", Digest: "same", ExpectedDigest: "same",
			Size: 100, MaximumSize: 8000, Precedence: "project>user", ExpectedPrecedence: "project>user",
		}},
		ObservedAt: now,
	})
	if !report.Qualified || report.Blockers() != 0 || len(report.Findings) != 0 {
		t.Fatalf("healthy report = %+v", report)
	}
}
