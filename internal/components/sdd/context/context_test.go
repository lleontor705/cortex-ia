package context

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestAssemblePreservesTrustClassesAndFixedPrecedence(t *testing.T) {
	input := Input{
		Sections: []Section{
			{ID: "context/remote", Class: ir.TrustRemoteUntrusted, Content: "remote data"},
			{ID: "context/schema", Class: ir.TrustTrustedSchema, Content: "output schema"},
			{ID: "context/secret", Class: ir.TrustSecretReference, Secret: &SecretReference{ID: "secret/runtime-token", Provider: "operator-environment"}},
			{ID: "context/tool", Class: ir.TrustToolOutput, Content: "tool data"},
			{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: "root policy", Controls: Controls{Authority: []string{"route"}, Permissions: []string{"filesystem/read"}, Approvals: []string{"destructive"}, Destinations: []string{"orchestrator"}, StopConditions: []string{"blocked"}}},
			{ID: "context/peer", Class: ir.TrustPeerMessage, Content: "peer data"},
			{ID: "context/repository", Class: ir.TrustRepositoryData, Content: "repository data"},
			{ID: "context/operator", Class: ir.TrustOperatorInput, Content: "typed work packet"},
		},
		Limits: Limits{MaxBytes: 4096, MaxSectionBytes: 512, OnOverflow: OverflowFail},
	}

	got, err := Assemble(input)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	wantClasses := []ir.TrustClass{
		ir.TrustTrustedPolicy,
		ir.TrustOperatorInput,
		ir.TrustTrustedSchema,
		ir.TrustRepositoryData,
		ir.TrustToolOutput,
		ir.TrustPeerMessage,
		ir.TrustRemoteUntrusted,
		ir.TrustSecretReference,
	}
	classes := make([]ir.TrustClass, len(got.Sections))
	for index := range got.Sections {
		classes[index] = got.Sections[index].Class
	}
	if !reflect.DeepEqual(classes, wantClasses) {
		t.Fatalf("trust precedence = %v, want %v", classes, wantClasses)
	}
	if !reflect.DeepEqual(got.Controls, input.Sections[4].Controls) {
		t.Fatalf("effective controls = %+v, want trusted policy controls %+v", got.Controls, input.Sections[4].Controls)
	}
	if got.Bytes != len(got.Prompt) || !strings.Contains(string(got.Prompt), `"class":"remote_untrusted"`) {
		t.Fatalf("compiled prompt does not account for its complete trust-labelled bytes: bytes=%d prompt=%s", got.Bytes, got.Prompt)
	}
	if got.Sections[7].Secret == nil || got.Sections[7].Secret.ID != "secret/runtime-token" || got.Sections[7].Content != "" {
		t.Fatalf("secret reference was not preserved opaquely: %+v", got.Sections[7])
	}
}

func TestAssembleRejectsControlClaimsOutsideTrustedPolicy(t *testing.T) {
	classes := []ir.TrustClass{
		ir.TrustTrustedSchema,
		ir.TrustOperatorInput,
		ir.TrustRepositoryData,
		ir.TrustToolOutput,
		ir.TrustPeerMessage,
		ir.TrustRemoteUntrusted,
		ir.TrustSecretReference,
	}
	controls := []struct {
		name   string
		claims Controls
	}{
		{name: "authority", claims: Controls{Authority: []string{"become-admin"}}},
		{name: "permissions", claims: Controls{Permissions: []string{"network/any"}}},
		{name: "approvals", claims: Controls{Approvals: []string{"self-approve"}}},
		{name: "destinations", claims: Controls{Destinations: []string{"external"}}},
		{name: "stop conditions", claims: Controls{StopConditions: []string{"never-stop"}}},
	}

	for _, class := range classes {
		for _, claim := range controls {
			t.Run(string(class)+"/"+claim.name, func(t *testing.T) {
				section := Section{ID: "context/untrusted", Class: class, Content: "data", Controls: claim.claims}
				if class == ir.TrustSecretReference {
					section.Content = ""
					section.Secret = &SecretReference{ID: "secret/runtime-token", Provider: "operator-environment"}
				}
				_, err := Assemble(Input{Sections: []Section{section}, Limits: Limits{MaxBytes: 1024, MaxSectionBytes: 512}})
				var validationErr *ValidationError
				if !errors.As(err, &validationErr) || validationErr.Code != ErrorAuthorityBoundary {
					t.Fatalf("Assemble() error = %v, want authority boundary error", err)
				}
			})
		}
	}
}

func TestAssembleEnforcesLimitsWithoutDroppingMandatoryLayers(t *testing.T) {
	mandatory := []Section{
		{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: strings.Repeat("p", 1000)},
		{ID: "context/schema", Class: ir.TrustTrustedSchema, Content: strings.Repeat("s", 1000)},
	}

	t.Run("fails when mandatory layers exceed limit", func(t *testing.T) {
		_, err := Assemble(Input{Sections: mandatory, Limits: Limits{MaxBytes: 1500, MaxSectionBytes: 1200, OnOverflow: OverflowDegrade}})
		var validationErr *ValidationError
		if !errors.As(err, &validationErr) || validationErr.Code != ErrorSizeLimit {
			t.Fatalf("Assemble() error = %v, want size limit error", err)
		}
	})

	t.Run("degrades only optional layers", func(t *testing.T) {
		input := Input{
			Sections: append([]Section{
				{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: strings.Repeat("p", 32)},
				{ID: "context/schema", Class: ir.TrustTrustedSchema, Content: strings.Repeat("s", 32)},
			},
				Section{ID: "context/repository", Class: ir.TrustRepositoryData, Content: strings.Repeat("r", 32)},
				Section{ID: "context/remote", Class: ir.TrustRemoteUntrusted, Content: strings.Repeat("u", 1000)},
			),
			Limits: Limits{MaxBytes: 800, MaxSectionBytes: 1200, OnOverflow: OverflowDegrade},
		}
		got, err := Assemble(input)
		if err != nil {
			t.Fatalf("Assemble() error = %v", err)
		}
		if len(got.Sections) != 3 || got.Sections[0].ID != "context/policy" || got.Sections[1].ID != "context/schema" {
			t.Fatalf("mandatory sections were removed or reordered: %+v", got.Sections)
		}
		if len(got.Degradations) != 1 || got.Degradations[0].SectionID != "context/remote" {
			t.Fatalf("degradations = %+v, want remote section omitted first", got.Degradations)
		}
	})

	t.Run("degrades an optional section that exceeds its own limit", func(t *testing.T) {
		got, err := Assemble(Input{
			Sections: []Section{
				{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: "root"},
				{ID: "context/remote", Class: ir.TrustRemoteUntrusted, Content: strings.Repeat("u", 65)},
			},
			Limits: Limits{MaxBytes: 1024, MaxSectionBytes: 64, OnOverflow: OverflowDegrade},
		})
		if err != nil {
			t.Fatalf("Assemble() error = %v", err)
		}
		if len(got.Sections) != 1 || got.Sections[0].ID != "context/policy" {
			t.Fatalf("oversized optional section was retained: %+v", got.Sections)
		}
		if len(got.Degradations) != 1 || got.Degradations[0].SectionID != "context/remote" {
			t.Fatalf("degradations = %+v, want oversized remote section", got.Degradations)
		}
	})
}

func TestAssembleRejectsUnresolvedAndCyclicProgressiveReferences(t *testing.T) {
	tests := []struct {
		name       string
		references []Reference
		sectionRef []ir.SemanticID
		wantCode   ErrorCode
	}{
		{name: "unresolved", sectionRef: []ir.SemanticID{"procedure/missing"}, wantCode: ErrorUnresolvedReference},
		{name: "cycle", sectionRef: []ir.SemanticID{"procedure/a"}, references: []Reference{
			{ID: "procedure/a", Content: "A", References: []ir.SemanticID{"procedure/b"}},
			{ID: "procedure/b", Content: "B", References: []ir.SemanticID{"procedure/a"}},
		}, wantCode: ErrorCyclicReference},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Assemble(Input{
				Sections:   []Section{{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: "root", References: tt.sectionRef}},
				References: tt.references,
				Limits:     Limits{MaxBytes: 1024, MaxSectionBytes: 512},
			})
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.Code != tt.wantCode {
				t.Fatalf("Assemble() error = %v, want %s", err, tt.wantCode)
			}
		})
	}
}

func TestAssembleLoadsOnlyRelevantReferences(t *testing.T) {
	got, err := Assemble(Input{
		Sections: []Section{{ID: "context/policy", Class: ir.TrustTrustedPolicy, Content: "root", References: []ir.SemanticID{"procedure/apply", "procedure/verify"}}},
		References: []Reference{
			{ID: "procedure/apply", Content: "apply details", References: []ir.SemanticID{"procedure/shared"}},
			{ID: "procedure/verify", Content: "verify details"},
			{ID: "procedure/shared", Content: "shared details"},
		},
		RelevantReferences: []ir.SemanticID{"procedure/apply"},
		Limits:             Limits{MaxBytes: 1024, MaxSectionBytes: 512},
	})
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	want := []ir.SemanticID{"procedure/apply", "procedure/shared"}
	ids := make([]ir.SemanticID, len(got.References))
	for index := range got.References {
		ids[index] = got.References[index].ID
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("loaded references = %v, want %v", ids, want)
	}
}

func TestAssembleRejectsNonOpaqueSecrets(t *testing.T) {
	tests := []Section{
		{ID: "context/secret", Class: ir.TrustSecretReference, Content: "actual-secret-value", Secret: &SecretReference{ID: "secret/runtime-token", Provider: "operator-environment"}},
		{ID: "context/secret", Class: ir.TrustSecretReference, Secret: &SecretReference{ID: "secret/runtime-token", Provider: "https://user:password@example.test"}},
		{ID: "context/secret", Class: ir.TrustSecretReference},
	}
	for index, section := range tests {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			_, err := Assemble(Input{Sections: []Section{section}, Limits: Limits{MaxBytes: 1024, MaxSectionBytes: 512}})
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.Code != ErrorOpaqueSecret {
				t.Fatalf("Assemble() error = %v, want opaque secret error", err)
			}
		})
	}
}
