package agents

import (
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

// NewDefaultRegistry returns a registry with the supported OpenCode agent adapter.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(opencode.NewAdapter())
	return r
}
