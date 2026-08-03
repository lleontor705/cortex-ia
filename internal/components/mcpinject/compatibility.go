package mcpinject

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/services"
)

var ErrIncompatibleService = errors.New("external service compatibility blocked")

type CompatibilityState string

const (
	CompatibilityQualified CompatibilityState = "qualified"
	CompatibilityDegraded  CompatibilityState = "degraded"
	CompatibilityBlocked   CompatibilityState = "blocked"
)

type InstalledService struct {
	Version      ir.Version
	Capabilities map[string]ir.Version
}

type CompatibilityFinding struct {
	Code         string
	Service      services.Owner
	CapabilityID string
	Observed     string
	Expected     string
	Remediation  string
	Blocking     bool
}

type CompatibilityResult struct {
	State    CompatibilityState
	Service  services.Owner
	Findings []CompatibilityFinding
}

// AssessCompatibility compares observed external-service facts with the
// configured contract. It reports only compatibility; authority and mutable
// lifecycle state remain in their authoritative external services.
func AssessCompatibility(contract services.ServiceContract, installed InstalledService) CompatibilityResult {
	result := CompatibilityResult{State: CompatibilityQualified, Service: contract.Owner}
	if err := services.ValidateContracts([]services.ServiceContract{contract}); err != nil {
		return blocked(result, CompatibilityFinding{
			Code: "service.contract.invalid", Service: contract.Owner, Observed: err.Error(),
			Expected: "a valid external-service ownership contract", Remediation: "repair the generated ownership manifest", Blocking: true,
		})
	}

	result = assessVersion(result, "service.version", "", installed.Version, contract.Versions)
	for _, required := range contract.RequiredCapabilities {
		observed, found := installed.Capabilities[required.ID]
		if !found {
			result = blocked(result, CompatibilityFinding{
				Code: "service.capability.missing", Service: contract.Owner, CapabilityID: required.ID,
				Observed: "unavailable", Expected: required.Versions.String(),
				Remediation: "upgrade or configure the authoritative upstream service capability", Blocking: true,
			})
			continue
		}
		result = assessVersion(result, "service.capability.version", required.ID, observed, required.Versions)
	}
	return result
}

// InjectCompatible blocks before mutation on unsupported service facts and
// carries any conservative forward-version degradation into the result.
func InjectCompatible(homeDir string, adapter agents.Adapter, tmpl ServerTemplates, installed InstalledService) (InjectionResult, error) {
	compatibility := AssessCompatibility(tmpl.Service, installed)
	if compatibility.State == CompatibilityBlocked {
		return InjectionResult{Compatibility: compatibility}, fmt.Errorf("%w: %s", ErrIncompatibleService, summarizeFindings(compatibility.Findings))
	}
	result, err := Inject(homeDir, adapter, tmpl)
	result.Compatibility = compatibility
	return result, err
}

func assessVersion(result CompatibilityResult, code, capabilityID string, observed ir.Version, supported ir.VersionRange) CompatibilityResult {
	finding := CompatibilityFinding{
		Code: code, Service: result.Service, CapabilityID: capabilityID, Observed: observed.String(), Expected: supported.String(),
		Remediation: "install a qualified version or provide passing compatibility evidence",
	}
	if observed.Major == 0 || observed.Major != supported.Minimum.Major || compareVersion(observed, supported.Minimum) < 0 {
		finding.Blocking = true
		return blocked(result, finding)
	}
	if compareVersion(observed, supported.MaximumTested) > 0 {
		if result.State != CompatibilityBlocked {
			result.State = CompatibilityDegraded
		}
		result.Findings = append(result.Findings, finding)
	}
	return result
}

func blocked(result CompatibilityResult, finding CompatibilityFinding) CompatibilityResult {
	result.State = CompatibilityBlocked
	result.Findings = append(result.Findings, finding)
	return result
}

func compareVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}

func summarizeFindings(findings []CompatibilityFinding) string {
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, fmt.Sprintf("%s observed=%s expected=%s", finding.Code, finding.Observed, finding.Expected))
	}
	return strings.Join(parts, "; ")
}
