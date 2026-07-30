// Package services defines compatibility and ownership contracts for external
// workflow services. It does not implement ForgeSpec, Cortex, or runtime APIs.
package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type Owner string

const (
	OwnerForgeSpec Owner = "forgespec"
	OwnerCortex    Owner = "cortex"
	OwnerRuntime   Owner = "runtime-native"
)

type Authority string

const (
	AuthorityExternalService Authority = "external-service"
	AuthorityTransportOnly   Authority = "transport-only"
)

type Responsibility string

const (
	ResponsibilityContracts         Responsibility = "sdd-contracts"
	ResponsibilityTaskDependencies  Responsibility = "task-dependencies"
	ResponsibilityTaskReadiness     Responsibility = "task-readiness"
	ResponsibilityTaskClaim         Responsibility = "task-claim"
	ResponsibilityTaskStatus        Responsibility = "task-status"
	ResponsibilityMemory            Responsibility = "durable-memory"
	ResponsibilityEvidence          Responsibility = "evidence"
	ResponsibilityProvenance        Responsibility = "provenance"
	ResponsibilityRelationships     Responsibility = "knowledge-relationships"
	ResponsibilityDispatchTransport Responsibility = "direct-child-dispatch-transport"
)

type VersionInterval = ir.VersionRange

// CrossServiceID deliberately contains identity only. Mutable status, payload,
// evidence, or lifecycle data remains in the authoritative owning service.
type CrossServiceID struct {
	Owner Owner  `json:"owner"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
}

type CapabilityRequirement struct {
	ID       string          `json:"id"`
	Versions VersionInterval `json:"versions"`
	Upstream bool            `json:"upstream"`
}

type ServiceContract struct {
	SchemaVersion        ir.Version              `json:"schema_version"`
	Owner                Owner                   `json:"owner"`
	Authority            Authority               `json:"authority"`
	Versions             VersionInterval         `json:"versions"`
	Responsibilities     []Responsibility        `json:"responsibilities"`
	RequiredCapabilities []CapabilityRequirement `json:"required_capabilities,omitempty"`
	References           []CrossServiceID        `json:"references,omitempty"`
	ExternalDependency   bool                    `json:"external_dependency"`
}

// CompatibilityMatrix supplies evidence-backed external version intervals.
// Keeping them out of package constants prevents cortex-ia from fabricating
// qualification claims for independently released upstream services.
type CompatibilityMatrix struct {
	ForgeSpec                   VersionInterval
	ForgeSpecTransactionalClaim VersionInterval
	Cortex                      VersionInterval
	Runtime                     VersionInterval
}

// CanonicalContracts returns fresh values so callers cannot mutate package
// authority. The caller must supply externally qualified version intervals.
func CanonicalContracts(compatibility CompatibilityMatrix) []ServiceContract {
	return []ServiceContract{
		{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			Owner:         OwnerForgeSpec,
			Authority:     AuthorityExternalService,
			Versions:      compatibility.ForgeSpec,
			Responsibilities: []Responsibility{
				ResponsibilityContracts,
				ResponsibilityTaskDependencies,
				ResponsibilityTaskReadiness,
				ResponsibilityTaskClaim,
				ResponsibilityTaskStatus,
			},
			RequiredCapabilities: []CapabilityRequirement{{
				ID:       "forgespec.capabilities",
				Versions: compatibility.ForgeSpecTransactionalClaim,
				Upstream: true,
			}},
			ExternalDependency: true,
		},
		{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			Owner:         OwnerCortex,
			Authority:     AuthorityExternalService,
			Versions:      compatibility.Cortex,
			Responsibilities: []Responsibility{
				ResponsibilityMemory,
				ResponsibilityEvidence,
				ResponsibilityProvenance,
				ResponsibilityRelationships,
			},
			ExternalDependency: true,
		},
		{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			Owner:         OwnerRuntime,
			Authority:     AuthorityTransportOnly,
			Versions:      compatibility.Runtime,
			Responsibilities: []Responsibility{
				ResponsibilityDispatchTransport,
			},
			ExternalDependency: false,
		},
	}
}

// ValidateContracts prevents a local/runtime mirror from becoming a second
// mutable authority and requires every responsibility to have exactly one owner.
func ValidateContracts(contracts []ServiceContract) error {
	if len(contracts) == 0 {
		return errors.New("at least one service contract is required")
	}
	owners := make(map[Owner]struct{}, len(contracts))
	responsibilities := make(map[Responsibility]Owner)
	for _, contract := range contracts {
		if !canonicalOwner(contract.Owner) {
			return fmt.Errorf("owner %q cannot hold workflow authority", contract.Owner)
		}
		if _, exists := owners[contract.Owner]; exists {
			return fmt.Errorf("duplicate service owner %q", contract.Owner)
		}
		owners[contract.Owner] = struct{}{}
		wantAuthority := AuthorityExternalService
		wantExternal := true
		if contract.Owner == OwnerRuntime {
			wantAuthority = AuthorityTransportOnly
			wantExternal = false
		}
		if contract.Authority != wantAuthority {
			return fmt.Errorf("service %q authority must be %q", contract.Owner, wantAuthority)
		}
		if contract.SchemaVersion.Major == 0 || !validVersionInterval(contract.Versions) {
			return fmt.Errorf("service %q has incomplete compatibility interval", contract.Owner)
		}
		if contract.ExternalDependency != wantExternal {
			return fmt.Errorf("service %q external dependency declaration is invalid", contract.Owner)
		}
		if len(contract.Responsibilities) == 0 {
			return fmt.Errorf("service %q has no responsibilities", contract.Owner)
		}
		for _, responsibility := range contract.Responsibilities {
			if previous, exists := responsibilities[responsibility]; exists {
				return fmt.Errorf("responsibility %q has authorities %q and %q", responsibility, previous, contract.Owner)
			}
			responsibilities[responsibility] = contract.Owner
		}
		for _, reference := range contract.References {
			if !canonicalOwner(reference.Owner) || strings.TrimSpace(reference.Kind) == "" || strings.TrimSpace(reference.ID) == "" {
				return fmt.Errorf("service %q has invalid cross-service reference", contract.Owner)
			}
		}
		for _, capability := range contract.RequiredCapabilities {
			if strings.TrimSpace(capability.ID) == "" || !validVersionInterval(capability.Versions) {
				return fmt.Errorf("service %q has incomplete capability requirement", contract.Owner)
			}
		}
	}
	return nil
}

func canonicalOwner(owner Owner) bool {
	return owner == OwnerForgeSpec || owner == OwnerCortex || owner == OwnerRuntime
}

func versionInterval(minimum, maximum string) VersionInterval {
	return VersionInterval{Minimum: ir.MustParseVersion(minimum), MaximumTested: ir.MustParseVersion(maximum)}
}

func validVersionInterval(interval VersionInterval) bool {
	minimum, maximum := interval.Minimum, interval.MaximumTested
	if minimum.Major == 0 || maximum.Major != minimum.Major || maximum.Minor < minimum.Minor {
		return false
	}
	return maximum.Minor != minimum.Minor || maximum.Patch >= minimum.Patch
}
