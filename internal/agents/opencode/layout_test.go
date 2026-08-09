package opencode

import "testing"

func TestNativeLayoutSeparatesDiscoveredAndCortexAssets(t *testing.T) {
	layout := NativeLayout()

	if layout.ConfigRoot != ".config/opencode" {
		t.Fatalf("ConfigRoot = %q", layout.ConfigRoot)
	}
	if layout.WorkflowRoot != ".cortex-ia/opencode" {
		t.Fatalf("WorkflowRoot = %q", layout.WorkflowRoot)
	}
	for _, path := range []string{
		".config/opencode/opencode.json",
		".config/opencode/opencode.jsonc",
		".config/opencode/AGENTS.md",
		".config/opencode/agents/implement.md",
		".config/opencode/commands/implement.md",
		".config/opencode/skills/implement/SKILL.md",
	} {
		if !layout.IsNativePath(path) {
			t.Errorf("IsNativePath(%q) = false", path)
		}
	}
	for _, path := range []string{
		".config/opencode/composition.json",
		".config/opencode/roles/implement.md",
		".config/opencode/skills/_shared/sdd-phase-contract.md",
		".cortex-ia/opencode/composition.json",
	} {
		if layout.IsNativePath(path) {
			t.Errorf("IsNativePath(%q) = true", path)
		}
	}
}

func TestNativeLayoutRecognizesHomeRelativeWorkflowPaths(t *testing.T) {
	layout := NativeLayout()
	if !layout.IsWorkflowPath(".cortex-ia/opencode/contracts/phase-envelope.json") {
		t.Fatal("expected OpenCode workflow contract to be home-relative")
	}
	for _, path := range []string{".cortex-ia/opencode-other/file", ".cortex-ia/claude/file", "../.cortex-ia/opencode/file"} {
		if layout.IsWorkflowPath(path) {
			t.Errorf("IsWorkflowPath(%q) = true", path)
		}
	}
}
