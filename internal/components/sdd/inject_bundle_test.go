package sdd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestCompileInjectionBundlePreviewsProfileAndDegradationBeforeMutation(t *testing.T) {
	homeDir := t.TempDir()
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	compilation := compiledInjectionFixture(now, ProfilePortableSequential, nil)

	compiled, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
		Compilation: compilation,
		Renderer:    injectionFixtureRenderer{},
		AllowedAssetKinds: []renderers.AssetKind{
			renderers.AssetInstruction,
		},
	})
	if err != nil {
		t.Fatalf("CompileInjectionBundle() error = %v", err)
	}

	if compiled.Profile != ProfilePortableSequential {
		t.Fatalf("profile = %q, want %q", compiled.Profile, ProfilePortableSequential)
	}
	wantDegradations := []string{"delegation/direct-child: no fresh proven capability fact"}
	if !reflect.DeepEqual(compiled.Degradations, wantDegradations) {
		t.Fatalf("degradations = %v, want %v", compiled.Degradations, wantDegradations)
	}
	if compiled.Fingerprint != compilation.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", compiled.Fingerprint, compilation.Fingerprint)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".fixture", "workflow.md")); !os.IsNotExist(err) {
		t.Fatalf("compilation mutated install target before apply: %v", err)
	}
}

func TestInjectCompiledBundleUsesOneDeterministicIdempotentPath(t *testing.T) {
	homeDir := t.TempDir()
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	compilation := compiledInjectionFixture(now, ProfilePortableFlat, []capability.CapabilityFact{qualifiedProfileFact(directChildDelegation, now)})
	compiled, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
		Compilation:       compilation,
		Renderer:          injectionFixtureRenderer{},
		AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction},
	})
	if err != nil {
		t.Fatalf("CompileInjectionBundle() error = %v", err)
	}

	first, err := InjectCompiledBundle(homeDir, compiled)
	if err != nil {
		t.Fatalf("InjectCompiledBundle() first error = %v", err)
	}
	if !first.Changed || first.Profile != ProfilePortableFlat || len(first.Degradations) != 0 {
		t.Fatalf("first result = %+v", first)
	}
	wantPath := filepath.Join(homeDir, ".fixture", "workflow.md")
	if !reflect.DeepEqual(first.Files, []string{wantPath}) {
		t.Fatalf("files = %v, want one compiled bundle path %v", first.Files, []string{wantPath})
	}
	content, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "profile=portable-flat\n" {
		t.Fatalf("content = %q", content)
	}

	second, err := InjectCompiledBundle(homeDir, compiled)
	if err != nil {
		t.Fatalf("InjectCompiledBundle() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("unchanged compiled bundle was not idempotent: %+v", second)
	}
	if !reflect.DeepEqual(first.Files, second.Files) || first.Fingerprint != second.Fingerprint {
		t.Fatalf("stable result changed: first=%+v second=%+v", first, second)
	}
}

func TestCompileInjectionBundleRejectsProfileOutsideCompiledInput(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	compilation := compiledInjectionFixture(now, ProfileNativeAdvanced, nil)
	_, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
		Compilation:       compilation,
		Renderer:          injectionFixtureRenderer{},
		AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction},
	})
	if err == nil || !strings.Contains(err.Error(), "compiled profile") {
		t.Fatalf("CompileInjectionBundle() error = %v, want compiled profile mismatch", err)
	}
}

type injectionFixtureRenderer struct{}

func (injectionFixtureRenderer) Target() renderers.TargetID { return "fixture" }

func (injectionFixtureRenderer) Render(_ context.Context, resolved renderers.ResolvedWorkflow) (renderers.Bundle, error) {
	return renderers.Bundle{Assets: []renderers.Asset{{
		Path:       ".fixture/workflow.md",
		SemanticID: "fixture/workflow",
		Kind:       renderers.AssetInstruction,
		Content:    []byte("profile=" + resolved.Profile + "\n"),
		Mode:       0o644,
	}}}, nil
}

func compiledInjectionFixture(now time.Time, profile WorkflowProfile, facts []capability.CapabilityFact) compiler.Result {
	fingerprint := strings.Repeat("a", 64)
	return compiler.Result{
		Fingerprint: fingerprint,
		Normalized: compiler.NormalizedInput{
			Workflow:       ir.WorkflowIR{ID: "workflow/fixture", Version: ir.MustParseVersion("1.0.0")},
			Catalog:        capability.Catalog{Facts: facts},
			Target:         "fixture",
			Profile:        string(profile),
			EvaluationTime: now.Format(time.RFC3339Nano),
		},
	}
}
