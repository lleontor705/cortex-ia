// Package forgespec injects the ForgeSpec MCP server configuration for SDD artifact persistence.
package forgespec

import (
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
)

// Inject injects the forgespec MCP server config into the given agent.
func Inject(homeDir string, adapter agents.Adapter) (mcpinject.InjectionResult, error) {
	return mcpinject.Inject(homeDir, adapter, Templates())
}
