package verify

import (
	"fmt"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
)

const (
	FindingForgeSpecMode         FindingCode = "doctor.forgespec.mode"
	FindingCapabilityMissing     FindingCode = "doctor.forgespec.capability.missing"
	FindingP1Degradation         FindingCode = "doctor.forgespec.p1.degradation"
	FindingRetiredRegistration   FindingCode = "doctor.retirement.registration"
	FindingExternalData          FindingCode = "doctor.retirement.external-data"
	FindingInstructionDigest     FindingCode = "doctor.instructions.digest"
	FindingInstructionSize       FindingCode = "doctor.instructions.size"
	FindingInstructionPrecedence FindingCode = "doctor.instructions.precedence"
)

type DiagnosticTarget struct {
	Target   string
	Path     string
	Observed string
	Expected string
	Evidence string
}

type InstructionContract struct {
	Target             string
	Path               string
	Digest             string
	ExpectedDigest     string
	Size               int
	MaximumSize        int
	Precedence         string
	ExpectedPrecedence string
}

type WorkflowDiagnosticInput struct {
	Profile              string
	Resolution           forgespec.ForgeSpecResolution
	RetiredRegistrations []DiagnosticTarget
	ExternalMailboxPaths []string
	Instructions         []InstructionContract
	Additional           []Observation
	ObservedAt           time.Time
}

func DiagnoseWorkflow(input WorkflowDiagnosticInput) DoctorReport {
	report := DoctorReport{Profile: input.Profile, Qualified: true}
	add := func(finding Finding) {
		report.Findings = append(report.Findings, finding)
		if finding.Blocking {
			report.Qualified = false
		}
	}
	if input.Resolution.Mode == forgespec.CoordinationBlocked {
		add(actionableFinding(FindingForgeSpecMode, SeverityError, "forgespec", "capabilities", "blocked", "direct-v1 or legacy-sequential", "ForgeSpec capability resolution", "probe a compatible fresh endpoint or select supported legacy-sequential requirements", true))
	}
	for _, missing := range input.Resolution.Missing {
		add(actionableFinding(FindingCapabilityMissing, SeverityError, "forgespec", string(missing.ID), "missing or incompatible", missing.Versions.String(), "ForgeSpec capability snapshot", "upgrade or configure ForgeSpec and rerun the capability probe", true))
	}
	for _, degradation := range input.Resolution.Degradations {
		add(actionableFinding(FindingP1Degradation, SeverityWarning, "forgespec", string(degradation.CapabilityID), degradation.Reason, "qualified P1 capability or declared sequential substitution", "ForgeSpec capability snapshot", "continue only with the disclosed sequential/no-concurrent-write behavior or qualify the capability", false))
	}
	for _, target := range input.RetiredRegistrations {
		expected := target.Expected
		if expected == "" {
			expected = "retired registration absent"
		}
		add(actionableFinding(FindingRetiredRegistration, SeverityError, nonEmpty(target.Target, "runtime"), nonEmpty(target.Path, "configuration"), nonEmpty(target.Observed, "present"), expected, nonEmpty(target.Evidence, "configuration scan"), "preview and apply the exact ownership-proven retirement plan, then reload the runtime", true))
	}
	for _, path := range input.ExternalMailboxPaths {
		add(actionableFinding(FindingExternalData, SeverityWarning, "external-data", path, "preserved", "never mutated automatically", "protected-path policy", "archive or remove manually only after operator-controlled preservation checks", false))
	}
	for _, instruction := range input.Instructions {
		if instruction.Digest != instruction.ExpectedDigest {
			add(actionableFinding(FindingInstructionDigest, SeverityError, instruction.Target, instruction.Path, instruction.Digest, instruction.ExpectedDigest, "rendered instruction digest", "regenerate the workflow bundle from the immutable plan", true))
		}
		if instruction.MaximumSize > 0 && instruction.Size > instruction.MaximumSize {
			add(actionableFinding(FindingInstructionSize, SeverityError, instruction.Target, instruction.Path, fmt.Sprintf("%d bytes", instruction.Size), fmt.Sprintf("<= %d bytes", instruction.MaximumSize), "rendered instruction size", "reduce mandatory instruction layers or select a supported degraded profile", true))
		}
		if instruction.Precedence != instruction.ExpectedPrecedence {
			add(actionableFinding(FindingInstructionPrecedence, SeverityError, instruction.Target, instruction.Path, instruction.Precedence, instruction.ExpectedPrecedence, "runtime instruction precedence probe", "reorder instruction layers and regenerate the bundle", true))
		}
	}
	additional := Diagnose(input.Profile, input.Additional)
	for _, finding := range additional.Findings {
		add(finding)
	}
	// Advisory findings remain visible, but qualification is derived solely
	// from the final blocker set rather than finding count or severity.
	report.Qualified = report.Blockers() == 0
	return report
}

func actionableFinding(code FindingCode, severity Severity, target, path, observed, expected, evidence, remediation string, blocking bool) Finding {
	return Finding{
		Code: code, Severity: severity, Target: nonEmpty(target, "unknown"), Path: nonEmpty(path, "unknown"),
		Observed: nonEmpty(observed, "unknown"), Expected: nonEmpty(expected, "declared contract"),
		Evidence: nonEmpty(evidence, "unavailable"), Remediation: nonEmpty(remediation, "collect evidence and rerun doctor"), Blocking: blocking,
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
