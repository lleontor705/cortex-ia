package cortex

import (
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/services"
)

// Contract declares Cortex as the sole external authority for durable memory
// and evidence. cortex-ia only renders and checks its MCP configuration.
func Contract() services.ServiceContract {
	version := ir.MustParseVersion("1.0.0")
	return services.ServiceContract{
		SchemaVersion: version, Owner: services.OwnerCortex, Authority: services.AuthorityExternalService,
		Versions: ir.VersionRange{Minimum: version, MaximumTested: version}, ExternalDependency: true,
		Responsibilities: []services.Responsibility{
			services.ResponsibilityMemory, services.ResponsibilityEvidence, services.ResponsibilityProvenance, services.ResponsibilityRelationships,
		},
	}
}

// Templates returns the MCP server templates for cortex.
// Cortex is a Go binary: command is "cortex", not npx.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "cortex", Service: Contract(),

		// Claude Code: ~/.claude/mcp/cortex.json
		SeparateFileJSON: []byte(`{
  "command": "cortex",
  "args": [
    "mcp"
  ]
}
`),

		// Default mcpServers overlay
		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "cortex": {
      "command": "cortex",
      "args": [
        "mcp"
      ]
    }
  }
}
`),

		// OpenCode: uses "mcp" key with type "local"
		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "cortex": {
      "type": "local",
      "command": [
        "cortex",
        "mcp"
      ],
      "enabled": true
    }
  }
}
`),

		// VS Code: uses "servers" key
		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "cortex": {
      "type": "stdio",
      "command": "cortex",
      "args": [
        "mcp"
      ]
    }
  }
}
`),

		// Codex: TOML format
		TOMLCommand: "cortex",
		TOMLArgs:    []string{"mcp"},
	}
}
