package renderers

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type rendererFunc struct {
	target TargetID
	bundle Bundle
	err    error
}

func (r rendererFunc) Target() TargetID { return r.target }

func (r rendererFunc) Render(context.Context, ResolvedWorkflow) (Bundle, error) {
	return r.bundle, r.err
}

func TestRenderEnforcesTargetAndValidatesRendererOutput(t *testing.T) {
	t.Run("target mismatch", func(t *testing.T) {
		_, err := Render(context.Background(), rendererFunc{target: "codex"}, resolvedWorkflow())
		var validationErr *ValidationError
		if !errors.As(err, &validationErr) || validationErr.ID != ErrorRendererTargetMismatch {
			t.Fatalf("Render() error = %v, want ID %q", err, ErrorRendererTargetMismatch)
		}
	})

	t.Run("invalid output", func(t *testing.T) {
		asset := validAsset()
		asset.Content = []byte("model={{ model }}")
		_, err := Render(context.Background(), rendererFunc{target: "claude", bundle: Bundle{Assets: []Asset{asset}}}, resolvedWorkflow())
		var validationErr *ValidationError
		if !errors.As(err, &validationErr) || validationErr.ID != ErrorUnresolvedVariable {
			t.Fatalf("Render() error = %v, want ID %q", err, ErrorUnresolvedVariable)
		}
	})
}

func TestValidateBundleProducesStableOwnedAssetOrder(t *testing.T) {
	resolved := resolvedWorkflow()
	input := Bundle{Assets: []Asset{
		{
			Path:        "skills/validate.md",
			SemanticID:  "asset/skill/validate",
			Kind:        AssetSkill,
			Content:     []byte("validate"),
			Mode:        0o644,
			Permissions: []string{"tool/read", "filesystem/read", "tool/read"},
		},
		{
			Path:        "agents/implement.md",
			SemanticID:  "asset/agent/implement",
			Kind:        AssetAgent,
			Content:     []byte("implement"),
			Mode:        0o600,
			Permissions: []string{"filesystem/read"},
			Extensions:  []ir.SemanticID{"claude/hooks"},
		},
	}}

	first, err := ValidateBundle(resolved, input)
	if err != nil {
		t.Fatalf("ValidateBundle() error = %v", err)
	}
	second, err := ValidateBundle(resolved, input)
	if err != nil {
		t.Fatalf("ValidateBundle() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated validation differed:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if got := []string{first.Assets[0].Path, first.Assets[1].Path}; !reflect.DeepEqual(got, []string{"agents/implement.md", "skills/validate.md"}) {
		t.Fatalf("asset order = %v", got)
	}
	if got := first.Assets[1].Permissions; !reflect.DeepEqual(got, []string{"filesystem/read", "tool/read"}) {
		t.Fatalf("permissions = %v", got)
	}
	if first.Assets[0].Mode != fs.FileMode(0o600) || first.Assets[1].Mode != fs.FileMode(0o644) {
		t.Fatalf("modes changed: %#o %#o", first.Assets[0].Mode, first.Assets[1].Mode)
	}

	input.Assets[0].Content[0] = 'X'
	input.Assets[0].Permissions[0] = "network/write"
	if string(first.Assets[1].Content) != "validate" || first.Assets[1].Permissions[1] != "tool/read" {
		t.Fatal("validated bundle aliases renderer-owned input")
	}
}

func TestValidateBundleRejectsUnresolvedOrUnauthorizedOutputWithSemanticIDs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ResolvedWorkflow, *Asset)
		wantID  ir.SemanticID
		wantRef ir.SemanticID
	}{
		{
			name: "unresolved mustache variable",
			mutate: func(_ *ResolvedWorkflow, asset *Asset) {
				asset.Content = []byte("model: {{ model }}")
			},
			wantID:  ErrorUnresolvedVariable,
			wantRef: "asset/agent/implement",
		},
		{
			name: "unresolved shell variable",
			mutate: func(_ *ResolvedWorkflow, asset *Asset) {
				asset.Content = []byte("root=${WORKSPACE}")
			},
			wantID:  ErrorUnresolvedVariable,
			wantRef: "asset/agent/implement",
		},
		{
			name: "permission widening",
			mutate: func(_ *ResolvedWorkflow, asset *Asset) {
				asset.Permissions = append(asset.Permissions, "network/write")
			},
			wantID:  ErrorPermissionWidening,
			wantRef: "asset/agent/implement",
		},
		{
			name: "unsupported asset kind",
			mutate: func(_ *ResolvedWorkflow, asset *Asset) {
				asset.Kind = AssetKind("runtime-state")
			},
			wantID:  ErrorUnsupportedAsset,
			wantRef: "asset/agent/implement",
		},
		{
			name: "undeclared extension",
			mutate: func(_ *ResolvedWorkflow, asset *Asset) {
				asset.Extensions = []ir.SemanticID{"claude/experimental-hook"}
			},
			wantID:  ErrorUndeclaredExtension,
			wantRef: "asset/agent/implement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := resolvedWorkflow()
			asset := validAsset()
			tt.mutate(&resolved, &asset)

			_, err := ValidateBundle(resolved, Bundle{Assets: []Asset{asset}})
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("ValidateBundle() error = %v, want *ValidationError", err)
			}
			if validationErr.ID != tt.wantID || validationErr.SemanticID != tt.wantRef {
				t.Fatalf("ValidateBundle() error = %+v, want ID=%q semantic ID=%q", validationErr, tt.wantID, tt.wantRef)
			}
		})
	}
}

func TestValidateBundleRequiresTargetNamespacedDeclaredExtensions(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*ResolvedWorkflow)
		wantID ir.SemanticID
	}{
		{
			name: "extension is not target namespaced",
			modify: func(resolved *ResolvedWorkflow) {
				resolved.Extensions = []ExtensionDeclaration{{ID: "codex/hooks"}}
			},
			wantID: ErrorInvalidExtension,
		},
		{
			name: "duplicate declaration",
			modify: func(resolved *ResolvedWorkflow) {
				resolved.Extensions = append(resolved.Extensions, ExtensionDeclaration{ID: "claude/hooks"})
			},
			wantID: ErrorInvalidExtension,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := resolvedWorkflow()
			tt.modify(&resolved)
			_, err := ValidateBundle(resolved, Bundle{Assets: []Asset{validAsset()}})
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.ID != tt.wantID {
				t.Fatalf("ValidateBundle() error = %v, want ID %q", err, tt.wantID)
			}
		})
	}
}

func TestValidateBundleRejectsUnstablePathsModesAndDuplicateAssets(t *testing.T) {
	tests := []struct {
		name   string
		assets []Asset
		wantID ir.SemanticID
	}{
		{name: "traversal path", assets: []Asset{func() Asset { a := validAsset(); a.Path = "../agent.md"; return a }()}, wantID: ErrorInvalidAsset},
		{name: "absolute path", assets: []Asset{func() Asset { a := validAsset(); a.Path = "/agent.md"; return a }()}, wantID: ErrorInvalidAsset},
		{name: "zero mode", assets: []Asset{func() Asset { a := validAsset(); a.Mode = 0; return a }()}, wantID: ErrorInvalidAsset},
		{name: "special mode", assets: []Asset{func() Asset { a := validAsset(); a.Mode = fs.ModeSetuid | 0o755; return a }()}, wantID: ErrorInvalidAsset},
		{name: "duplicate path", assets: []Asset{validAsset(), func() Asset { a := validAsset(); a.SemanticID = "asset/agent/second"; return a }()}, wantID: ErrorDuplicateAsset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateBundle(resolvedWorkflow(), Bundle{Assets: tt.assets})
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.ID != tt.wantID {
				t.Fatalf("ValidateBundle() error = %v, want ID %q", err, tt.wantID)
			}
		})
	}
}

func resolvedWorkflow() ResolvedWorkflow {
	return ResolvedWorkflow{
		Target:             "claude",
		Profile:            "portable-flat",
		AllowedAssetKinds:  []AssetKind{AssetSkill, AssetAgent},
		AllowedPermissions: []string{"tool/read", "filesystem/read"},
		Extensions:         []ExtensionDeclaration{{ID: "claude/hooks", Optional: true}},
	}
}

func validAsset() Asset {
	return Asset{
		Path:        "agents/implement.md",
		SemanticID:  "asset/agent/implement",
		Kind:        AssetAgent,
		Content:     []byte("implement"),
		Mode:        0o644,
		Permissions: []string{"filesystem/read"},
		Extensions:  []ir.SemanticID{"claude/hooks"},
	}
}
