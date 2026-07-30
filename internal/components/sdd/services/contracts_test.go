package services

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCanonicalContractsAssignOneExternalOwnerPerResponsibility(t *testing.T) {
	contracts := CanonicalContracts(testCompatibility())
	want := map[Owner][]Responsibility{
		OwnerForgeSpec: {ResponsibilityContracts, ResponsibilityTaskDependencies, ResponsibilityTaskReadiness, ResponsibilityTaskClaim, ResponsibilityTaskStatus},
		OwnerCortex:    {ResponsibilityMemory, ResponsibilityEvidence, ResponsibilityProvenance, ResponsibilityRelationships},
		OwnerRuntime:   {ResponsibilityDispatchTransport},
	}

	if err := ValidateContracts(contracts); err != nil {
		t.Fatalf("ValidateContracts() error = %v", err)
	}
	for _, contract := range contracts {
		if !reflect.DeepEqual(contract.Responsibilities, want[contract.Owner]) {
			t.Fatalf("%s responsibilities = %v, want %v", contract.Owner, contract.Responsibilities, want[contract.Owner])
		}
		wantAuthority := AuthorityExternalService
		if contract.Owner == OwnerRuntime {
			wantAuthority = AuthorityTransportOnly
		}
		if contract.Authority != wantAuthority {
			t.Fatalf("%s authority = %q, want %q", contract.Owner, contract.Authority, wantAuthority)
		}
	}
}

func TestCanonicalContractsDeclareVersionIntervals(t *testing.T) {
	for _, contract := range CanonicalContracts(testCompatibility()) {
		if contract.SchemaVersion.Major == 0 || contract.Versions.Minimum.Major == 0 || contract.Versions.MaximumTested.Major == 0 {
			t.Fatalf("%s has incomplete schema/service version compatibility: %+v", contract.Owner, contract)
		}
	}
}

func TestCrossServiceIDsAreOpaqueReferences(t *testing.T) {
	reference := CrossServiceID{Owner: OwnerForgeSpec, Kind: "task", ID: "task-123"}
	encoded, err := json.Marshal(reference)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"owner":"forgespec","kind":"task","id":"task-123"}`; got != want {
		t.Fatalf("json.Marshal() = %s, want %s", got, want)
	}
}

func TestValidateContractsRejectsSecondMutableAuthority(t *testing.T) {
	contracts := CanonicalContracts(testCompatibility())
	contracts = append(contracts, ServiceContract{
		SchemaVersion:    versionInterval("1.0.0", "1.0.0").Minimum,
		Owner:            Owner("local-mirror"),
		Authority:        AuthorityExternalService,
		Versions:         versionInterval("1.0.0", "1.0.0"),
		Responsibilities: []Responsibility{ResponsibilityTaskStatus},
	})

	if err := ValidateContracts(contracts); err == nil {
		t.Fatal("ValidateContracts() error = nil, want duplicate authority rejection")
	}
}

func TestForgeSpecTransactionalChangesRemainExternalDependency(t *testing.T) {
	var forgeSpec ServiceContract
	for _, contract := range CanonicalContracts(testCompatibility()) {
		if contract.Owner == OwnerForgeSpec {
			forgeSpec = contract
			break
		}
	}

	if !forgeSpec.ExternalDependency {
		t.Fatal("ForgeSpec ExternalDependency = false, want true")
	}
	if len(forgeSpec.RequiredCapabilities) != 1 || forgeSpec.RequiredCapabilities[0].ID != "forgespec.capabilities" {
		t.Fatalf("ForgeSpec RequiredCapabilities = %v, want explicit capabilities negotiation dependency", forgeSpec.RequiredCapabilities)
	}
	if !forgeSpec.RequiredCapabilities[0].Upstream {
		t.Fatal("ForgeSpec transactional capability Upstream = false, want true")
	}
}

func testCompatibility() CompatibilityMatrix {
	return CompatibilityMatrix{
		ForgeSpec:                   versionInterval("1.0.0", "1.2.0"),
		ForgeSpecTransactionalClaim: versionInterval("1.1.0", "1.2.0"),
		Cortex:                      versionInterval("1.0.0", "1.3.0"),
		Runtime:                     versionInterval("1.0.0", "1.1.0"),
	}
}
