package installroots

import "testing"

func TestResolveSupportedWorkflowRoot(t *testing.T) {
	roots, err := Resolve("vscode", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if roots.Workflow.Relative != ".copilot" {
		t.Fatalf("workflow root = %+v", roots.Workflow)
	}
	if roots.ExternalConfig != nil {
		t.Fatalf("unexpected external root = %+v", roots.ExternalConfig)
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
