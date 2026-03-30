package orchestrator

import "github.com/lleontor705/cortex-ia/internal/components/mcpinject"

// Templates returns the MCP server templates for cli-orchestrator-mcp.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "cli-orchestrator",

		SeparateFileJSON: []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "cli-orchestrator-mcp"
  ]
}
`),

		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "cli-orchestrator": {
      "command": "npx",
      "args": [
        "-y",
        "cli-orchestrator-mcp"
      ]
    }
  }
}
`),

		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "cli-orchestrator": {
      "type": "local",
      "command": [
        "npx",
        "-y",
        "cli-orchestrator-mcp"
      ],
      "enabled": true
    }
  }
}
`),

		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "cli-orchestrator": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "cli-orchestrator-mcp"
      ]
    }
  }
}
`),

		TOMLCommand: "npx",
		TOMLArgs:    []string{"-y", "cli-orchestrator-mcp"},
	}
}
