package cortex

import "github.com/lleontor705/cortex-ia/internal/components/mcpinject"

// Templates returns the MCP server templates for cortex.
// Cortex is a Go binary: command is "cortex", not npx.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "cortex",

		// Claude Code: ~/.claude/mcp/cortex.json
		SeparateFileJSON: []byte(`{
  "command": "cortex",
  "args": [
    "mcp"
  ]
}
`),

		// Cursor, Windsurf, Gemini: mcpServers overlay
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

		// Antigravity: uses mcpServers (same as default)
		AntigravityOverlayJSON: nil,

		// Codex: TOML format
		TOMLCommand: "cortex",
		TOMLArgs:    []string{"mcp"},
	}
}
