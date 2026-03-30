package agents

import (
	"github.com/lleontor705/cortex-ia/internal/agents/antigravity"
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/cursor"
	"github.com/lleontor705/cortex-ia/internal/agents/gemini"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/agents/vscode"
	"github.com/lleontor705/cortex-ia/internal/agents/windsurf"
)

// NewDefaultRegistry returns a registry with all 8 supported agent adapters.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(claude.NewAdapter())
	r.Register(opencode.NewAdapter())
	r.Register(gemini.NewAdapter())
	r.Register(cursor.NewAdapter())
	r.Register(vscode.NewAdapter())
	r.Register(codex.NewAdapter())
	r.Register(windsurf.NewAdapter())
	r.Register(antigravity.NewAdapter())
	return r
}
