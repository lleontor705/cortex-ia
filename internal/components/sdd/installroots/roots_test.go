package installroots

import (
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestKiroSeparatesWorkflowAndExternalConfigRoots(t *testing.T) {
	external := filepath.Join(t.TempDir(), "User")
	roots, err := Resolve("kiro", t.TempDir(), external)
	if err != nil {
		t.Fatal(err)
	}
	if roots.Workflow.Scope != ir.ScopeWorkflowRoot || roots.Workflow.Relative != ".kiro" {
		t.Fatalf("workflow root = %+v", roots.Workflow)
	}
	if roots.ExternalConfig == nil || roots.ExternalConfig.Scope != ir.ScopeExternalConfig || roots.ExternalConfig.Absolute != external {
		t.Fatalf("external root = %+v", roots.ExternalConfig)
	}
	if roots.ExternalConfig.Absolute == roots.Workflow.Relative {
		t.Fatal("Kiro roots were collapsed")
	}
}

func TestResolveRejectsRelativeKiroExternalConfig(t *testing.T) {
	if _, err := Resolve("kiro", t.TempDir(), "relative/User"); err == nil {
		t.Fatal("relative external root accepted")
	}
}

func TestResolveRejectsUnsafeTypedRootFragments(t *testing.T) {
	for _, adapter := range []string{"../escape", "/absolute", `C:\absolute`, `\\server\share`, "source-shaped/SKILL.md"} {
		t.Run(adapter, func(t *testing.T) {
			if _, err := Resolve(adapter, t.TempDir(), ""); err == nil {
				t.Fatalf("unsafe adapter root %q accepted", adapter)
			}
		})
	}
}

func TestResolvePreservesExternalRootWithoutRebasing(t *testing.T) {
	home := t.TempDir()
	external := filepath.Join(t.TempDir(), "User")
	roots, err := Resolve("kiro", home, external)
	if err != nil {
		t.Fatal(err)
	}
	if roots.ExternalConfig == nil || roots.ExternalConfig.Absolute != external {
		t.Fatalf("external root was rebased: %+v", roots.ExternalConfig)
	}
}

func TestResolveRejectsKiroExternalRootCollision(t *testing.T) {
	home := t.TempDir()
	if _, err := Resolve("kiro", home, filepath.Join(home, ".kiro")); err == nil {
		t.Fatal("external Kiro settings root collides with workflow root")
	}
}
