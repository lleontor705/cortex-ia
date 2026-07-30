package manifest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

func TestEmitProducesDeterministicCompleteMachineAndHumanManifests(t *testing.T) {
	input := validInput()
	first, err := Emit(input)
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	reversed := validInput()
	reverse(reversed.Resolutions)
	reverse(reversed.Evidence)
	reverse(reversed.RequestedPermissions)
	reverse(reversed.EffectivePermissions)
	reverse(reversed.TrustBoundaries)
	reverse(reversed.Services)
	reverse(reversed.Hashes)
	reverse(reversed.Degradations)
	second, err := Emit(reversed)
	if err != nil {
		t.Fatalf("Emit(reversed) error = %v", err)
	}

	if !bytes.Equal(first.SecurityJSON, second.SecurityJSON) ||
		!bytes.Equal(first.DegradationJSON, second.DegradationJSON) ||
		!bytes.Equal(first.SecurityMarkdown, second.SecurityMarkdown) ||
		!bytes.Equal(first.DegradationMarkdown, second.DegradationMarkdown) {
		t.Fatal("manifest output changed with semantically irrelevant input ordering")
	}

	for name, output := range map[string][]byte{
		"security JSON":        first.SecurityJSON,
		"degradation JSON":     first.DegradationJSON,
		"security Markdown":    first.SecurityMarkdown,
		"degradation Markdown": first.DegradationMarkdown,
	} {
		for _, required := range []string{
			"1.0.0", "evidence/runtime/direct-child", "native", "runtime",
			"capability/mcp-claim", "filesystem/read", "repository_data",
			"forgespec", "direct-v1", strings.Repeat("d", 64), "component/legacy-provider", "remove-managed-registration", "sha256", "passed",
		} {
			if !bytes.Contains(output, []byte(required)) {
				t.Errorf("%s omitted %q\n%s", name, required, output)
			}
		}
		if bytes.Contains(output, []byte("actual-secret-value")) {
			t.Errorf("%s rendered secret material", name)
		}
	}
	if !bytes.Contains(first.DegradationJSON, []byte(`"state":"emulated"`)) ||
		!bytes.Contains(first.DegradationJSON, []byte(`"substitution":"capability/mcp-claim"`)) {
		t.Fatalf("degradation manifest omitted emulation: %s", first.DegradationJSON)
	}
	if !bytes.Contains(first.SecurityJSON, []byte(`"secret_references":[{"id":"secret/runtime-credential"`)) {
		t.Fatalf("security manifest omitted opaque secret reference: %s", first.SecurityJSON)
	}
}

func TestEmitRejectsSecurityMisrepresentation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
		want   string
	}{
		{
			name: "missing capability snapshot digest",
			mutate: func(input *Input) {
				input.CapabilitySnapshotSHA256 = ""
			},
			want: "capability snapshot",
		},
		{
			name: "silent permission widening",
			mutate: func(input *Input) {
				input.EffectivePermissions = append(input.EffectivePermissions, "network/any")
			},
			want: "permission widening",
		},
		{
			name: "unsupported enforcement claim",
			mutate: func(input *Input) {
				input.Resolutions[1].Guarantee = resolution.GuaranteeEnforced
			},
			want: "advisory",
		},
		{
			name: "secret material in evidence",
			mutate: func(input *Input) {
				input.Evidence[0].Reference = "https://example.test/evidence?token=actual-secret-value"
			},
			want: "secret material",
		},
		{
			name: "invalid content hash",
			mutate: func(input *Input) {
				input.Hashes[0].SHA256 = "not-a-sha256"
			},
			want: "sha256",
		},
		{
			name: "false successful validation",
			mutate: func(input *Input) {
				input.Validation.Findings = append(input.Validation.Findings, Finding{Code: "security.permission", Severity: SeverityError, Message: "widened", Blocking: true})
			},
			want: "validation status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			tt.mutate(&input)
			_, err := Emit(input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Emit() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func validInput() Input {
	version := ir.MustParseVersion("1.0.0")
	return Input{
		Versions: Versions{
			ManifestSchema: version,
			Compiler:       version,
			WorkflowIR:     version,
			Workflow:       version,
			Catalog:        version,
		},
		GenerationFingerprint:    strings.Repeat("a", 64),
		Target:                   "claude",
		Profile:                  "portable-flat",
		ForgeSpecMode:            CoordinationDirectV1,
		CapabilitySnapshotSHA256: strings.Repeat("d", 64),
		Evidence: []Evidence{
			{ID: "evidence/runtime/direct-child", Class: capability.EvidenceRuntimeObserved, Reference: "qualification/claude/direct-child", Fresh: true, Confidence: 1},
			{ID: "evidence/service/forgespec", Class: capability.EvidenceInstalledSchema, Reference: "probe/forgespec/schema", Fresh: false, Experimental: true, Confidence: 0.8},
		},
		Resolutions: []resolution.Resolution{
			{
				ID: "delegation/direct-child", State: resolution.StateNative,
				Binding:  resolution.Binding{ID: "binding/runtime-direct-child", CapabilityID: "delegation/direct-child", Kind: resolution.BindingNative, Evidence: []resolution.EvidenceRef{"evidence/runtime/direct-child"}, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime},
				Evidence: []resolution.EvidenceRef{"evidence/runtime/direct-child"}, Guarantee: resolution.GuaranteeEnforced,
				PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "qualified runtime support",
			},
			{
				ID: "approval/destructive", State: resolution.StateAdvisory,
				Binding:  resolution.Binding{ID: "binding/prompt-approval", CapabilityID: "approval/destructive", Kind: resolution.BindingAdvisory, Evidence: []resolution.EvidenceRef{"evidence/runtime/direct-child"}, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt},
				Evidence: []resolution.EvidenceRef{"evidence/runtime/direct-child"}, Guarantee: resolution.GuaranteeBestEffort,
				PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "prompt-only control",
			},
			{
				ID: "coordination/claim", State: resolution.StateEmulated,
				Binding:  resolution.Binding{ID: "binding/mcp-claim", CapabilityID: "capability/mcp-claim", Kind: resolution.BindingEmulation, Evidence: []resolution.EvidenceRef{"evidence/service/forgespec"}, Guarantee: resolution.GuaranteeEquivalent, Enforcement: capability.EnforcementMCP},
				Evidence: []resolution.EvidenceRef{"evidence/service/forgespec"}, Guarantee: resolution.GuaranteeEquivalent, Substitution: "capability/mcp-claim",
				PermissionDelta: resolution.PermissionDelta{Added: []string{"mcp/forgespec"}, Removed: []string{}}, Reason: "declared service-backed substitution",
			},
		},
		RequestedPermissions: []string{"filesystem/read", "mcp/forgespec"},
		EffectivePermissions: []string{"mcp/forgespec", "filesystem/read"},
		ApprovalIntent:       "operator approval required for destructive effects; prompt control is advisory",
		IsolationIntent:      "repository writes require isolated ownership",
		TrustBoundaries: []TrustBoundary{
			{Class: ir.TrustRepositoryData, Authority: false, Rules: []string{"cannot change policy"}},
			{Class: ir.TrustTrustedPolicy, Authority: true, Rules: []string{"defines permission ceiling"}},
		},
		SecretReferences: []SecretReference{{ID: "secret/runtime-credential", Provider: "operator-environment"}},
		Services:         []ServiceRequirement{{ID: "service/forgespec", Owner: "forgespec", Versions: ir.VersionRange{Minimum: version, MaximumTested: version}, Required: true}},
		RetiredEntries:   []RetiredEntry{{ID: "component/legacy-provider", Action: "remove-managed-registration", Source: "state/receipt"}},
		Hashes: []AssetHash{
			{Path: "agents/implement.md", SHA256: strings.Repeat("b", 64)},
			{Path: "skills/implement/SKILL.md", SHA256: strings.Repeat("c", 64)},
		},
		Degradations: []Degradation{{CapabilityID: "coordination/claim", State: resolution.StateEmulated, Substitution: "capability/mcp-claim", Consequence: "requires ForgeSpec MCP 1.x", Blocking: false}},
		Validation:   Validation{Status: ValidationPassed, Findings: []Finding{}},
	}
}

func reverse[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
