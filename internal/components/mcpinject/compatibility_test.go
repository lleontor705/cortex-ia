package mcpinject_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/cortex"
	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/services"
)

func TestServiceOwnershipManifestsAgreeWithInjectedTemplates(t *testing.T) {
	tests := []struct {
		name     string
		contract services.ServiceContract
		template mcpinject.ServerTemplates
		owner    services.Owner
	}{
		{name: "ForgeSpec", contract: forgespec.Contract(), template: forgespec.Templates(), owner: services.OwnerForgeSpec},
		{name: "Cortex", contract: cortex.Contract(), template: cortex.Templates(), owner: services.OwnerCortex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := services.ValidateContracts([]services.ServiceContract{tt.contract}); err != nil {
				t.Fatalf("ValidateContracts() error = %v", err)
			}
			if tt.contract.Owner != tt.owner || tt.contract.Authority != services.AuthorityExternalService {
				t.Fatalf("ownership = (%q, %q), want (%q, %q)", tt.contract.Owner, tt.contract.Authority, tt.owner, services.AuthorityExternalService)
			}
			if !reflect.DeepEqual(tt.template.Service, tt.contract) {
				t.Fatalf("template service manifest = %+v, want %+v", tt.template.Service, tt.contract)
			}
		})
	}
}

func TestAssessCompatibilityExplicitlyQualifiesDegradesOrBlocks(t *testing.T) {
	contract := cortex.Contract()
	contract.Versions = ir.VersionRange{Minimum: ir.MustParseVersion("1.1.0"), MaximumTested: ir.MustParseVersion("1.2.0")}
	minimum := contract.Versions.Minimum

	tests := []struct {
		name      string
		installed mcpinject.InstalledService
		want      mcpinject.CompatibilityState
	}{
		{name: "qualified within tested interval", installed: mcpinject.InstalledService{Version: minimum}, want: mcpinject.CompatibilityQualified},
		{name: "degraded above tested interval", installed: mcpinject.InstalledService{Version: ir.Version{Major: minimum.Major, Minor: contract.Versions.MaximumTested.Minor + 1}}, want: mcpinject.CompatibilityDegraded},
		{name: "blocked below minimum", installed: mcpinject.InstalledService{Version: ir.Version{Major: minimum.Major}}, want: mcpinject.CompatibilityBlocked},
		{name: "blocked unknown major", installed: mcpinject.InstalledService{Version: ir.Version{Major: minimum.Major + 1}}, want: mcpinject.CompatibilityBlocked},
		{name: "blocked missing observation", installed: mcpinject.InstalledService{}, want: mcpinject.CompatibilityBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mcpinject.AssessCompatibility(contract, tt.installed)
			if got.State != tt.want {
				t.Fatalf("State = %q, want %q; findings=%+v", got.State, tt.want, got.Findings)
			}
			if tt.want != mcpinject.CompatibilityQualified && len(got.Findings) == 0 {
				t.Fatal("non-qualified result has no explicit finding")
			}
		})
	}
}

func TestForgeSpecRequiredUpstreamCapabilityBlocksWhenUnavailable(t *testing.T) {
	contract := forgespec.Contract()
	result := mcpinject.AssessCompatibility(contract, mcpinject.InstalledService{Version: contract.Versions.Minimum})
	if result.State != mcpinject.CompatibilityBlocked {
		t.Fatalf("State = %q, want %q", result.State, mcpinject.CompatibilityBlocked)
	}
	if len(result.Findings) != 1 || result.Findings[0].CapabilityID != "forgespec.capabilities" {
		t.Fatalf("Findings = %+v, want missing upstream capability", result.Findings)
	}
}

func TestInjectCompatibleBlocksBeforeMutationAndDisclosesDegradation(t *testing.T) {
	home := t.TempDir()
	adapter := opencode.NewAdapter()
	template := cortex.Templates()

	blocked, err := mcpinject.InjectCompatible(home, adapter, template, mcpinject.InstalledService{})
	if !errors.Is(err, mcpinject.ErrIncompatibleService) {
		t.Fatalf("InjectCompatible() error = %v, want ErrIncompatibleService", err)
	}
	if blocked.Changed || blocked.Compatibility.State != mcpinject.CompatibilityBlocked {
		t.Fatalf("blocked result = %+v", blocked)
	}
	if len(blocked.Files) != 0 {
		t.Fatalf("blocked files = %v, want none", blocked.Files)
	}

	contract := cortex.Contract()
	observed := ir.Version{Major: contract.Versions.MaximumTested.Major, Minor: contract.Versions.MaximumTested.Minor + 1}
	degraded, err := mcpinject.InjectCompatible(home, adapter, template, mcpinject.InstalledService{Version: observed})
	if err != nil {
		t.Fatalf("InjectCompatible() degraded error = %v", err)
	}
	if !degraded.Changed || degraded.Compatibility.State != mcpinject.CompatibilityDegraded || len(degraded.Compatibility.Findings) == 0 {
		t.Fatalf("degraded result = %+v", degraded)
	}
	wantPath := adapter.SettingsPath(home)
	if !reflect.DeepEqual(degraded.Files, []string{wantPath}) {
		t.Fatalf("files = %v, want %v", degraded.Files, []string{wantPath})
	}
}
