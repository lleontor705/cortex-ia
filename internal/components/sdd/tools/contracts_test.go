package tools

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestSelectBindingUsesExplicitSelectionBeforePrecedence(t *testing.T) {
	bindings := []Binding{
		validBinding("tool/filesystem-read", "binding/runtime-primary", 10),
		validBinding("tool/filesystem-read", "binding/runtime-audited", 20),
	}

	got, err := SelectBinding(Requirement{
		SemanticID:      "tool/filesystem-read",
		ExplicitBinding: "binding/runtime-audited",
	}, bindings)
	if err != nil {
		t.Fatalf("SelectBinding() error = %v", err)
	}
	if got.ID != "binding/runtime-audited" {
		t.Fatalf("SelectBinding() ID = %q, want binding/runtime-audited", got.ID)
	}
}

func TestValidateBindingRejectsRetiredCoordinationProviderSurfaces(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Binding)
	}{
		{name: "mailbox provider", mutate: func(b *Binding) { b.Provider = "agent-mailbox" }},
		{name: "message tool", mutate: func(b *Binding) { b.TargetTool = "msg_send" }},
		{name: "a2a binding", mutate: func(b *Binding) { b.ID = "binding/a2a-remote" }},
		{name: "resource lease permission", mutate: func(b *Binding) { b.Permission.Resources = []string{"resource_acquire"} }},
		{name: "dead letter evidence", mutate: func(b *Binding) { b.Evidence[0].Kind = "dlq-entry" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding := validBinding("tool/filesystem-read", "binding/runtime-read", 10)
			tt.mutate(&binding)
			if err := ValidateProviderSurface(binding); err == nil {
				t.Fatal("ValidateProviderSurface() error = nil, want retired coordination surface rejection")
			}
		})
	}
}

func TestSelectBindingUsesLowestUniquePrecedenceDeterministically(t *testing.T) {
	bindings := []Binding{
		validBinding("tool/memory-save", "binding/cortex-secondary", 20),
		validBinding("tool/memory-save", "binding/cortex-primary", 10),
	}

	for _, ordered := range [][]Binding{bindings, {bindings[1], bindings[0]}} {
		got, err := SelectBinding(Requirement{SemanticID: "tool/memory-save"}, ordered)
		if err != nil {
			t.Fatalf("SelectBinding() error = %v", err)
		}
		if got.ID != "binding/cortex-primary" {
			t.Fatalf("SelectBinding() ID = %q, want binding/cortex-primary", got.ID)
		}
	}
}

func TestSelectBindingRejectsAmbiguousPrecedence(t *testing.T) {
	bindings := []Binding{
		validBinding("tool/tasks-claim", "binding/forgespec-a", 10),
		validBinding("tool/tasks-claim", "binding/forgespec-b", 10),
	}

	_, err := SelectBinding(Requirement{SemanticID: "tool/tasks-claim"}, bindings)
	if !errors.Is(err, ErrAmbiguousBinding) {
		t.Fatalf("SelectBinding() error = %v, want ErrAmbiguousBinding", err)
	}
}

func TestValidateBindingRequiresCompleteCompatibilityContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Binding)
	}{
		{name: "input schema", mutate: func(b *Binding) { b.Input = ContractRef{} }},
		{name: "output schema", mutate: func(b *Binding) { b.Output = ContractRef{} }},
		{name: "permission scope", mutate: func(b *Binding) { b.Permission = PermissionScope{} }},
		{name: "enforcement", mutate: func(b *Binding) { b.Enforcement = "" }},
		{name: "version interval", mutate: func(b *Binding) { b.Versions = VersionInterval{} }},
		{name: "error mapping", mutate: func(b *Binding) { b.Errors = nil }},
		{name: "evidence", mutate: func(b *Binding) { b.Evidence = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding := validBinding("tool/tasks-claim", "binding/forgespec-claim", 10)
			tt.mutate(&binding)
			if err := binding.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want incomplete contract error")
			}
		})
	}
}

func TestNormalizeBindingProducesStableSetOrdering(t *testing.T) {
	binding := validBinding("tool/memory-save", "binding/cortex-save", 10)
	binding.Permission = PermissionScope{
		Effects:   []Effect{EffectNetwork, EffectRead, EffectNetwork},
		Resources: []string{"memory:write", "memory:read", "memory:write"},
	}
	binding.Evidence = []Evidence{
		{Kind: "observation-id", Schema: ContractRef{Kind: ContractSchema, Ref: "urn:cortex:observation-id", Version: version("1.0.0")}},
		{Kind: "audit-id", Schema: ContractRef{Kind: ContractSchema, Ref: "urn:cortex:audit-id", Version: version("1.0.0")}},
	}

	got := binding.Normalize()
	if want := []Effect{EffectNetwork, EffectRead}; !reflect.DeepEqual(got.Permission.Effects, want) {
		t.Fatalf("Normalize().Permission.Effects = %v, want %v", got.Permission.Effects, want)
	}
	if want := []string{"memory:read", "memory:write"}; !reflect.DeepEqual(got.Permission.Resources, want) {
		t.Fatalf("Normalize().Permission.Resources = %v, want %v", got.Permission.Resources, want)
	}
	if got.Evidence[0].Kind != "audit-id" || got.Evidence[1].Kind != "observation-id" {
		t.Fatalf("Normalize().Evidence = %v, want stable kind ordering", got.Evidence)
	}
}

func validBinding(semanticID, id string, precedence uint16) Binding {
	return Binding{
		SchemaVersion: version("1.0.0"),
		ID:            ir.SemanticID(id),
		SemanticID:    ir.SemanticID(semanticID),
		Provider:      "mcp",
		TargetTool:    id,
		Precedence:    precedence,
		Input:         ContractRef{Kind: ContractSchema, Ref: "urn:test:input", Version: version("1.0.0")},
		Output:        ContractRef{Kind: ContractSchema, Ref: "urn:test:output", Version: version("1.0.0")},
		Permission: PermissionScope{
			Effects:   []Effect{EffectNetwork},
			Resources: []string{"service:invoke"},
		},
		Enforcement: EnforcementMCP,
		Versions:    VersionInterval{Minimum: version("1.0.0"), MaximumTested: version("1.9.0")},
		Errors:      []ErrorMapping{{ProviderCode: "not_found", SemanticCode: "missing"}},
		Evidence: []Evidence{{
			Kind:   "receipt-id",
			Schema: ContractRef{Kind: ContractSchema, Ref: "urn:test:receipt", Version: version("1.0.0")},
		}},
	}
}

func version(value string) ir.Version { return ir.MustParseVersion(value) }
