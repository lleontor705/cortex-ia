package agents

import (
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/agents/vscode"
)

// NewDefaultRegistry returns a registry with all four supported agent adapters.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(claude.NewAdapter())
	r.Register(opencode.NewAdapter())
	r.Register(vscode.NewAdapter())
	r.Register(codex.NewAdapter())
	return r
}
