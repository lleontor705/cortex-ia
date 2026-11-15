package delegation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDetectHighRiskPaths(t *testing.T) {
	cases := []struct {
		paths    []string
		expected bool
	}{
		{paths: []string{"src/domain/user.go", "docs/readme.md"}, expected: false},
		{paths: []string{"src/auth/login.go"}, expected: true},
		{paths: []string{"src/crypto/token.go"}, expected: true},
		{paths: []string{"db/migrations/001_init.sql"}, expected: true},
		{paths: []string{"infra/secrets.yaml"}, expected: true},
		{paths: []string{"pkg/oauth/client.go"}, expected: true},
	}

	for _, c := range cases {
		got := DetectHighRiskPaths(c.paths)
		if got != c.expected {
			t.Errorf("DetectHighRiskPaths(%v) = %v; want %v", c.paths, got, c.expected)
		}
	}
}

func retiredSpecKitContract() *SDDContract {
	return &SDDContract{
		Version:   1,
		Workflow:  "sdd-lite",
		ChangeID:  "001-user-lockout",
		SpecPlane: "speckit",
		Pins: []ContractPin{{
			Transport: "workspace_file",
			Project:   "workspace",
			Locator:   ".specify/specs/001-user-lockout/spec.md",
			SHA256:    strings.Repeat("a", 64),
		}},
		RequirementIDs: []string{"REQ-USER-001"},
	}
}

func TestEncodeContractRejectsRetiredSpecKitPlane(t *testing.T) {
	if _, err := encodeContract(retiredSpecKitContract()); err == nil {
		t.Fatal("encodeContract accepted the retired speckit specification plane")
	}

	for _, plane := range []string{"openspec", "cortex", "hybrid"} {
		contract := retiredSpecKitContract()
		contract.SpecPlane = plane
		if plane == "cortex" {
			contract.Pins[0].Transport = "cortex_mcp"
		}
		if _, err := encodeContract(contract); err != nil {
			t.Fatalf("encodeContract rejected supported plane %q: %v", plane, err)
		}
	}
}

func TestDecodeContractRejectsRetiredSpecKitPlane(t *testing.T) {
	raw := `{"version":1,"workflow":"sdd-lite","change_id":"001-user-lockout","spec_plane":"speckit",` +
		`"pins":[{"transport":"workspace_file","project":"workspace","locator":"spec.md","sha256":"` +
		strings.Repeat("a", 64) + `"}],"requirement_ids":["REQ-USER-001"]}`
	if _, err := decodeContract(raw); err == nil {
		t.Fatal("decodeContract accepted the retired speckit specification plane")
	}
}

func TestProjectSpecKitStateIsFailClosedStub(t *testing.T) {
	err := (&Store{}).ProjectSpecKitState(context.Background(), "workspace", "001-user-lockout", "board")
	if !errors.Is(err, ErrSpecKitRetired) {
		t.Fatalf("ProjectSpecKitState error = %v, want ErrSpecKitRetired", err)
	}
}
