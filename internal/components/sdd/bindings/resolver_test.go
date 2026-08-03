package bindings

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/tools"
)

func TestResolveBlocksRequiredBindingFailures(t *testing.T) {
	tests := []struct {
		name     string
		request  Request
		bindings []tools.Binding
		code     ErrorCode
	}{
		{
			name:    "missing required binding",
			request: validRequest(),
			code:    ErrorMissingRequired,
		},
		{
			name:    "incompatible input schema",
			request: validRequest(),
			bindings: []tools.Binding{
				func() tools.Binding {
					binding := validBinding("binding/forgespec-claim", 10)
					binding.Input.Ref = "schema/task-claim-v2"
					return binding
				}(),
			},
			code: ErrorIncompatibleRequired,
		},
		{
			name:    "incompatible provider version",
			request: func() Request { request := validRequest(); request.ProviderVersion = version("2.0.0"); return request }(),
			bindings: []tools.Binding{
				validBinding("binding/forgespec-claim", 10),
			},
			code: ErrorIncompatibleRequired,
		},
		{
			name:    "ambiguous error mapping",
			request: validRequest(),
			bindings: []tools.Binding{
				func() tools.Binding {
					binding := validBinding("binding/forgespec-claim", 10)
					binding.Errors = append(binding.Errors, tools.ErrorMapping{ProviderCode: "not_found", SemanticCode: "unavailable"})
					return binding
				}(),
			},
			code: ErrorIncompatibleRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.request, tt.bindings)
			var blocked *BlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("Resolve() error = %v, want *BlockedError", err)
			}
			if blocked.Code != tt.code || !blocked.Blocking {
				t.Fatalf("Resolve() blocked = %+v, want code %q and blocking", blocked, tt.code)
			}
		})
	}
}

func TestResolveRejectsAmbiguousCompatibleBindingsIndependentOfOrder(t *testing.T) {
	first := validBinding("binding/forgespec-primary", 10)
	second := validBinding("binding/forgespec-secondary", 10)

	for _, catalog := range [][]tools.Binding{{first, second}, {second, first}} {
		_, err := Resolve(validRequest(), catalog)
		var blocked *BlockedError
		if !errors.As(err, &blocked) || blocked.Code != ErrorAmbiguousRequired {
			t.Fatalf("Resolve() error = %v, want ambiguous required binding", err)
		}
		if want := []ir.SemanticID{"binding/forgespec-primary", "binding/forgespec-secondary"}; !reflect.DeepEqual(blocked.Candidates, want) {
			t.Fatalf("BlockedError.Candidates = %v, want %v", blocked.Candidates, want)
		}
	}
}

func TestResolveExplicitSelectionPrecedesNumericPrecedence(t *testing.T) {
	request := validRequest()
	request.ExplicitBinding = "binding/forgespec-audited"
	catalog := []tools.Binding{
		validBinding("binding/forgespec-default", 10),
		validBinding("binding/forgespec-audited", 20),
	}

	result, err := Resolve(request, catalog)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Binding.ID != request.ExplicitBinding || result.Selection != SelectionExplicit {
		t.Fatalf("Resolve() result = %+v, want explicit binding", result)
	}
}

func TestResolveReportsAndBlocksPermissionWidening(t *testing.T) {
	request := validRequest()
	request.RenderedPermission = tools.PermissionScope{
		Effects:   []tools.Effect{tools.EffectNetwork, tools.EffectRead, tools.EffectNetwork},
		Resources: []string{"service:forgespec", "repository:*", "service:forgespec"},
	}

	result, err := Resolve(request, []tools.Binding{validBinding("binding/forgespec-claim", 10)})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Code != ErrorPermissionWidening {
		t.Fatalf("Resolve() error = %v, want permission widening", err)
	}
	wantCanonical := tools.PermissionScope{Effects: []tools.Effect{tools.EffectNetwork}, Resources: []string{"service:forgespec"}}
	wantRendered := tools.PermissionScope{Effects: []tools.Effect{tools.EffectNetwork, tools.EffectRead}, Resources: []string{"repository:*", "service:forgespec"}}
	if !reflect.DeepEqual(result.Permission.Canonical, wantCanonical) || !reflect.DeepEqual(result.Permission.Rendered, wantRendered) {
		t.Fatalf("Resolve() permission report = %+v", result.Permission)
	}
	if !reflect.DeepEqual(result.Permission.AddedEffects, []tools.Effect{tools.EffectRead}) || !reflect.DeepEqual(result.Permission.AddedResources, []string{"repository:*"}) {
		t.Fatalf("Resolve() widening delta = %+v", result.Permission)
	}
}

func TestResolveEmitsNormalizedContractsErrorsAndEvidence(t *testing.T) {
	binding := validBinding("binding/forgespec-claim", 10)
	binding.Errors = []tools.ErrorMapping{
		{ProviderCode: "timeout", SemanticCode: "unavailable"},
		{ProviderCode: "not_found", SemanticCode: "missing"},
	}
	binding.Evidence = []tools.Evidence{
		{Kind: "message-id", Schema: contract("schema/message-id", "1.0.0")},
		{Kind: "audit-id", Schema: contract("schema/audit-id", "1.0.0")},
	}

	result, err := Resolve(validRequest(), []tools.Binding{binding})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Selection != SelectionPrecedence || result.Binding.ID != binding.ID {
		t.Fatalf("Resolve() result = %+v, want precedence selection", result)
	}
	if result.Input.Ref != "schema/task-claim-input" || result.Output.Ref != "schema/task-claim-output" {
		t.Fatalf("Resolve() contracts = %v -> %v", result.Input, result.Output)
	}
	if result.ErrorMappings[0].ProviderCode != "not_found" || result.Evidence[0].Kind != "audit-id" {
		t.Fatalf("Resolve() normalized contracts = errors %v, evidence %v", result.ErrorMappings, result.Evidence)
	}
}

func TestResolveAllowsMissingOptionalBindingAsExplicitUnsupportedResult(t *testing.T) {
	request := validRequest()
	request.Required = false

	result, err := Resolve(request, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Status != StatusUnsupported || result.SemanticID != request.SemanticID {
		t.Fatalf("Resolve() result = %+v, want unsupported", result)
	}
}

func validRequest() Request {
	return Request{
		SemanticID:      "tool/tasks-claim",
		Required:        true,
		ProviderVersion: version("1.4.0"),
		Input:           SchemaRequirement{Kind: tools.ContractSchema, Ref: "schema/task-claim-input", Versions: versions("1.0.0", "1.2.0")},
		Output:          SchemaRequirement{Kind: tools.ContractSchema, Ref: "schema/task-claim-output", Versions: versions("1.0.0", "1.2.0")},
		CanonicalPermission: tools.PermissionScope{
			Effects:   []tools.Effect{tools.EffectNetwork},
			Resources: []string{"service:forgespec"},
		},
		RenderedPermission: tools.PermissionScope{
			Effects:   []tools.Effect{tools.EffectNetwork},
			Resources: []string{"service:forgespec"},
		},
	}
}

func validBinding(id string, precedence uint16) tools.Binding {
	return tools.Binding{
		SchemaVersion: version("1.0.0"),
		ID:            ir.SemanticID(id),
		SemanticID:    "tool/tasks-claim",
		Provider:      "forgespec-mcp",
		TargetTool:    "tb_claim",
		Precedence:    precedence,
		Input:         contract("schema/task-claim-input", "1.1.0"),
		Output:        contract("schema/task-claim-output", "1.0.0"),
		Permission: tools.PermissionScope{
			Effects:   []tools.Effect{tools.EffectNetwork},
			Resources: []string{"service:forgespec"},
		},
		Enforcement: tools.EnforcementMCP,
		Versions:    versions("1.0.0", "1.9.0"),
		Errors:      []tools.ErrorMapping{{ProviderCode: "not_found", SemanticCode: "missing"}},
		Evidence:    []tools.Evidence{{Kind: "attempt-id", Schema: contract("schema/attempt-id", "1.0.0")}},
	}
}

func contract(ref, schemaVersion string) tools.ContractRef {
	return tools.ContractRef{Kind: tools.ContractSchema, Ref: ref, Version: version(schemaVersion)}
}

func versions(minimum, maximum string) ir.VersionRange {
	return ir.VersionRange{Minimum: version(minimum), MaximumTested: version(maximum)}
}

func version(value string) ir.Version { return ir.MustParseVersion(value) }
