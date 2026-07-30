// Package conformance provides the fail-closed release gate for deterministic,
// CI-safe workflow compiler evidence. It aggregates evidence produced by the
// focused schema, renderer, security, doctor, install, and rollback suites; it
// does not execute credentialed external runtimes.
package conformance

import (
	"fmt"
	"slices"
	"strings"
)

// Domain identifies one mandatory static release-conformance area.
type Domain string

const (
	DomainSchemas         Domain = "schemas"
	DomainAdapterProfiles Domain = "adapter-profiles"
	DomainDeterminism     Domain = "determinism"
	DomainEquivalence     Domain = "equivalence"
	DomainPrompts         Domain = "prompts"
	DomainBindings        Domain = "bindings"
	DomainPermissions     Domain = "permissions"
	DomainSecrets         Domain = "secrets"
	DomainDoctor          Domain = "doctor"
	DomainInstall         Domain = "install"
	DomainRollback        Domain = "rollback"
	DomainSourceInventory Domain = "source-inventory"
)

var requiredDomains = []Domain{
	DomainAdapterProfiles,
	DomainBindings,
	DomainDeterminism,
	DomainDoctor,
	DomainEquivalence,
	DomainInstall,
	DomainPermissions,
	DomainPrompts,
	DomainRollback,
	DomainSchemas,
	DomainSecrets,
	DomainSourceInventory,
}

// RequiredDomains returns an owned, deterministically ordered domain list.
func RequiredDomains() []Domain { return slices.Clone(requiredDomains) }

// Outcome is the terminal result of one focused conformance suite. Only passed
// is release-clean; degraded and inconclusive results remain visible failures.
type Outcome string

const (
	OutcomePassed       Outcome = "passed"
	OutcomeFailed       Outcome = "failed"
	OutcomeDegraded     Outcome = "degraded"
	OutcomeInconclusive Outcome = "inconclusive"
)

// Check links one mandatory domain to its focused CI evidence.
type Check struct {
	Domain   Domain   `json:"domain"`
	Outcome  Outcome  `json:"outcome"`
	Evidence []string `json:"evidence"`
}

// Metrics captures the exact release invariants that cannot be represented by
// a narrative pass/fail check. Every field is a count so reports are auditable.
type Metrics struct {
	SupportedGoldens           int `json:"supported_goldens"`
	PassedGoldens              int `json:"passed_goldens"`
	DeterminismComparisons     int `json:"determinism_comparisons"`
	EqualDeterminismResults    int `json:"equal_determinism_results"`
	RequiredBindings           int `json:"required_bindings"`
	ResolvedOrBlockedBindings  int `json:"resolved_or_blocked_bindings"`
	Degradations               int `json:"degradations"`
	MachineVisibleDegradations int `json:"machine_visible_degradations"`
	HumanVisibleDegradations   int `json:"human_visible_degradations"`
	PreinstallDegradations     int `json:"preinstall_degradations"`
	RenderedSecrets            int `json:"rendered_secrets"`
	PermissionWidenings        int `json:"permission_widenings"`
	UnresolvedRequiredValues   int `json:"unresolved_required_values"`
	FalseEnforcementClaims     int `json:"false_enforcement_claims"`
	CriticalApprovalBypasses   int `json:"critical_approval_bypasses"`
	FalsePasses                int `json:"false_passes"`
}

type ReleaseEvidence struct {
	Checks  []Check `json:"checks"`
	Metrics Metrics `json:"metrics"`
}

type RepositoryReleaseRequest struct {
	Collector CollectorRequest
	Checks    []Check
	Metrics   Metrics
}

// EvaluateRepositoryRelease is the production release boundary. Repository
// absence evidence is always collected from source; callers cannot substitute
// declared strings or counts for the scan result.
func EvaluateRepositoryRelease(request RepositoryReleaseRequest) (Report, RepositoryEvidence, error) {
	collected, err := CollectRepository(request.Collector)
	if err != nil {
		return Report{}, RepositoryEvidence{}, err
	}
	outcome := OutcomePassed
	switch collected.Status {
	case EvidencePassed:
	case EvidenceInconclusive:
		outcome = OutcomeInconclusive
	default:
		outcome = OutcomeFailed
	}
	checks := slices.Clone(request.Checks)
	checks = append(checks, Check{
		Domain: DomainSourceInventory, Outcome: outcome,
		Evidence: []string{"repository-source-scan:" + collected.Digest},
	})
	return Evaluate(ReleaseEvidence{Checks: checks, Metrics: request.Metrics}), collected, nil
}

type FindingCode string

const (
	FindingMissingDomain      FindingCode = "release.domain.missing"
	FindingDuplicateDomain    FindingCode = "release.domain.duplicate"
	FindingUnexpectedDomain   FindingCode = "release.domain.unexpected"
	FindingDomainFailed       FindingCode = "release.domain.failed"
	FindingDomainInconclusive FindingCode = "release.domain.inconclusive"
	FindingMissingEvidence    FindingCode = "release.evidence.missing"
	FindingInvalidMetric      FindingCode = "release.metric.invalid"
	FindingGoldenCoverage     FindingCode = "release.goldens.coverage"
	FindingDeterminism        FindingCode = "release.determinism.mismatch"
	FindingBindingCoverage    FindingCode = "release.bindings.coverage"
	FindingHiddenDegradation  FindingCode = "release.degradation.hidden"
	FindingSecretExposure     FindingCode = "release.secret.exposure"
	FindingPermissionWidening FindingCode = "release.permission.widening"
	FindingUnresolvedValue    FindingCode = "release.value.unresolved"
	FindingFalseEnforcement   FindingCode = "release.enforcement.false-claim"
	FindingApprovalBypass     FindingCode = "release.approval.bypass"
	FindingFalsePass          FindingCode = "release.outcome.false-pass"
)

// Finding is a stable machine-readable release blocker.
type Finding struct {
	Code     FindingCode `json:"code"`
	Domain   Domain      `json:"domain,omitempty"`
	Observed string      `json:"observed"`
	Expected string      `json:"expected"`
}

type Report struct {
	Passed   bool      `json:"passed"`
	Checks   []Check   `json:"checks"`
	Metrics  Metrics   `json:"metrics"`
	Findings []Finding `json:"findings"`
}

// Evaluate applies every release invariant without retries or permissive
// defaults. Missing, degraded, inconclusive, or contradictory evidence blocks.
func Evaluate(evidence ReleaseEvidence) Report {
	report := Report{Checks: normalizeChecks(evidence.Checks), Metrics: evidence.Metrics}
	report.Findings = append(report.Findings, evaluateChecks(report.Checks)...)
	report.Findings = append(report.Findings, evaluateMetrics(evidence.Metrics)...)
	slices.SortFunc(report.Findings, compareFindings)
	report.Passed = len(report.Findings) == 0
	return report
}

func normalizeChecks(checks []Check) []Check {
	result := make([]Check, len(checks))
	for index, check := range checks {
		result[index] = check
		result[index].Evidence = slices.Clone(check.Evidence)
		slices.Sort(result[index].Evidence)
		result[index].Evidence = slices.Compact(result[index].Evidence)
	}
	slices.SortFunc(result, func(left, right Check) int {
		if difference := strings.Compare(string(left.Domain), string(right.Domain)); difference != 0 {
			return difference
		}
		return strings.Compare(string(left.Outcome), string(right.Outcome))
	})
	return result
}

func evaluateChecks(checks []Check) []Finding {
	required := make(map[Domain]struct{}, len(requiredDomains))
	for _, domain := range requiredDomains {
		required[domain] = struct{}{}
	}
	counts := make(map[Domain]int, len(checks))
	findings := make([]Finding, 0)
	for _, check := range checks {
		counts[check.Domain]++
		if _, known := required[check.Domain]; !known {
			findings = append(findings, finding(FindingUnexpectedDomain, check.Domain, string(check.Domain), "a required release-conformance domain"))
		}
		if len(check.Evidence) == 0 {
			findings = append(findings, finding(FindingMissingEvidence, check.Domain, "0 evidence references", "at least one focused CI evidence reference"))
		}
		switch check.Outcome {
		case OutcomePassed:
		case OutcomeInconclusive:
			findings = append(findings, finding(FindingDomainInconclusive, check.Domain, string(check.Outcome), string(OutcomePassed)))
		default:
			findings = append(findings, finding(FindingDomainFailed, check.Domain, string(check.Outcome), string(OutcomePassed)))
		}
	}
	for _, domain := range requiredDomains {
		switch counts[domain] {
		case 0:
			findings = append(findings, finding(FindingMissingDomain, domain, "0 checks", "exactly 1 check"))
		case 1:
		default:
			findings = append(findings, finding(FindingDuplicateDomain, domain, fmt.Sprintf("%d checks", counts[domain]), "exactly 1 check"))
		}
	}
	return findings
}

func evaluateMetrics(metrics Metrics) []Finding {
	findings := make([]Finding, 0)
	values := []struct {
		name  string
		value int
	}{
		{"supported_goldens", metrics.SupportedGoldens}, {"passed_goldens", metrics.PassedGoldens},
		{"determinism_comparisons", metrics.DeterminismComparisons}, {"equal_determinism_results", metrics.EqualDeterminismResults},
		{"required_bindings", metrics.RequiredBindings}, {"resolved_or_blocked_bindings", metrics.ResolvedOrBlockedBindings},
		{"degradations", metrics.Degradations}, {"machine_visible_degradations", metrics.MachineVisibleDegradations},
		{"human_visible_degradations", metrics.HumanVisibleDegradations}, {"preinstall_degradations", metrics.PreinstallDegradations},
		{"rendered_secrets", metrics.RenderedSecrets}, {"permission_widenings", metrics.PermissionWidenings},
		{"unresolved_required_values", metrics.UnresolvedRequiredValues}, {"false_enforcement_claims", metrics.FalseEnforcementClaims},
		{"critical_approval_bypasses", metrics.CriticalApprovalBypasses}, {"false_passes", metrics.FalsePasses},
	}
	for _, item := range values {
		if item.value < 0 {
			findings = append(findings, finding(FindingInvalidMetric, "", fmt.Sprintf("%s=%d", item.name, item.value), "a non-negative count"))
		}
	}
	if metrics.SupportedGoldens <= 0 || metrics.PassedGoldens != metrics.SupportedGoldens {
		findings = append(findings, finding(FindingGoldenCoverage, DomainAdapterProfiles, ratio(metrics.PassedGoldens, metrics.SupportedGoldens), "100% supported adapter/profile goldens passing"))
	}
	if metrics.DeterminismComparisons <= 0 || metrics.EqualDeterminismResults != metrics.DeterminismComparisons {
		findings = append(findings, finding(FindingDeterminism, DomainDeterminism, ratio(metrics.EqualDeterminismResults, metrics.DeterminismComparisons), "100% byte/hash comparisons equal"))
	}
	if metrics.RequiredBindings <= 0 || metrics.ResolvedOrBlockedBindings != metrics.RequiredBindings {
		findings = append(findings, finding(FindingBindingCoverage, DomainBindings, ratio(metrics.ResolvedOrBlockedBindings, metrics.RequiredBindings), "100% required bindings resolved or explicitly blocked"))
	}
	for channel, visible := range map[string]int{
		"machine":    metrics.MachineVisibleDegradations,
		"human":      metrics.HumanVisibleDegradations,
		"preinstall": metrics.PreinstallDegradations,
	} {
		if visible != metrics.Degradations {
			findings = append(findings, finding(FindingHiddenDegradation, DomainAdapterProfiles, fmt.Sprintf("%s=%d/%d", channel, visible, metrics.Degradations), "100% degradation visibility"))
		}
	}
	zeroTolerance := []struct {
		code   FindingCode
		domain Domain
		name   string
		value  int
	}{
		{FindingSecretExposure, DomainSecrets, "rendered secrets", metrics.RenderedSecrets},
		{FindingPermissionWidening, DomainPermissions, "permission widenings", metrics.PermissionWidenings},
		{FindingUnresolvedValue, DomainSchemas, "unresolved required values", metrics.UnresolvedRequiredValues},
		{FindingFalseEnforcement, DomainPermissions, "false enforcement claims", metrics.FalseEnforcementClaims},
		{FindingApprovalBypass, DomainPermissions, "critical approval bypasses", metrics.CriticalApprovalBypasses},
		{FindingFalsePass, DomainDoctor, "false passes", metrics.FalsePasses},
	}
	for _, invariant := range zeroTolerance {
		if invariant.value != 0 {
			findings = append(findings, finding(invariant.code, invariant.domain, fmt.Sprintf("%s=%d", invariant.name, invariant.value), "0"))
		}
	}
	return findings
}

func finding(code FindingCode, domain Domain, observed, expected string) Finding {
	return Finding{Code: code, Domain: domain, Observed: observed, Expected: expected}
}

func ratio(numerator, denominator int) string {
	return fmt.Sprintf("%d/%d", numerator, denominator)
}

func compareFindings(left, right Finding) int {
	if difference := strings.Compare(string(left.Code), string(right.Code)); difference != 0 {
		return difference
	}
	if difference := strings.Compare(string(left.Domain), string(right.Domain)); difference != 0 {
		return difference
	}
	return strings.Compare(left.Observed, right.Observed)
}
