package sdd

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

func TestFilesToBackup(t *testing.T) {
	adapter := opencode.NewAdapter()
	paths := FilesToBackup("/home/test", adapter)

	if len(paths) == 0 {
		t.Error("expected non-empty backup paths")
	}

	hasPrompt := false
	hasSkill := false
	for _, p := range paths {
		if strings.HasSuffix(p, "AGENTS.md") {
			hasPrompt = true
		}
		if strings.Contains(p, "bootstrap") {
			hasSkill = true
		}
	}
	if !hasPrompt {
		t.Error("expected AGENTS.md in backup paths")
	}
	if !hasSkill {
		t.Error("expected skill files in backup paths")
	}
}
