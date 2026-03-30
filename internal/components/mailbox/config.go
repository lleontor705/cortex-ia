package mailbox

import "github.com/lleontor705/cortex-ia/internal/components/mcpinject"

// Templates returns the MCP server templates for agent-mailbox-mcp.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "agent-mailbox",

		SeparateFileJSON: []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "agent-mailbox-mcp"
  ]
}
`),

		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "agent-mailbox": {
      "command": "npx",
      "args": [
        "-y",
        "agent-mailbox-mcp"
      ]
    }
  }
}
`),

		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "agent-mailbox": {
      "type": "local",
      "command": [
        "npx",
        "-y",
        "agent-mailbox-mcp"
      ],
      "enabled": true
    }
  }
}
`),

		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "agent-mailbox": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "agent-mailbox-mcp"
      ]
    }
  }
}
`),

		TOMLCommand: "npx",
		TOMLArgs:    []string{"-y", "agent-mailbox-mcp"},
	}
}
