package sdd

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestFilesToBackup(t *testing.T) {
	adapter := claude.NewAdapter()
	paths := FilesToBackup("/home/test", adapter)

	if len(paths) == 0 {
		t.Error("expected non-empty backup paths")
	}

	hasPrompt := false
	hasSkill := false
	for _, p := range paths {
		if strings.HasSuffix(p, "CLAUDE.md") {
			hasPrompt = true
		}
		if strings.Contains(p, "bootstrap") {
			hasSkill = true
		}
	}
	if !hasPrompt {
		t.Error("expected CLAUDE.md in backup paths")
	}
	if !hasSkill {
		t.Error("expected skill files in backup paths")
	}
}
