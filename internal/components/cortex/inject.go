package cortex

import (
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
)

// Inject injects the cortex MCP server config into the given agent.
func Inject(homeDir string, adapter agents.Adapter) (mcpinject.InjectionResult, error) {
	return mcpinject.Inject(homeDir, adapter, Templates())
}
