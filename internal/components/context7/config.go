package context7

import "github.com/lleontor705/cortex-ia/internal/components/mcpinject"

// Templates returns the MCP server templates for Context7.
// Context7 is available as both a remote HTTP endpoint and an npx package.
// Remote is preferred for agents that support it (lower latency).
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "context7",

		// Claude Code: ~/.claude/mcp/context7.json
		SeparateFileJSON: []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "@upstash/context7-mcp"
  ]
}
`),

		// Cursor, Windsurf, Gemini: mcpServers overlay
		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ]
    }
  }
}
`),

		// OpenCode: uses remote MCP (no npx needed)
		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "context7": {
      "type": "remote",
      "url": "https://mcp.context7.com/mcp",
      "enabled": true
    }
  }
}
`),

		// VS Code: uses "servers" key with HTTP remote
		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "context7": {
      "type": "http",
      "url": "https://mcp.context7.com/mcp"
    }
  }
}
`),

		// Antigravity: uses serverUrl for HTTP remote
		AntigravityOverlayJSON: []byte(`{
  "mcpServers": {
    "context7": {
      "serverUrl": "https://mcp.context7.com/mcp"
    }
  }
}
`),

		// Codex: TOML format (npx-based since no remote TOML support)
		TOMLCommand: "npx",
		TOMLArgs:    []string{"-y", "@upstash/context7-mcp"},
	}
}
