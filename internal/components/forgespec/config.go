package forgespec

import "github.com/lleontor705/cortex-ia/internal/components/mcpinject"

// Templates returns the MCP server templates for forgespec-mcp.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "forgespec",

		SeparateFileJSON: []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "forgespec-mcp"
  ]
}
`),

		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "forgespec": {
      "command": "npx",
      "args": [
        "-y",
        "forgespec-mcp"
      ]
    }
  }
}
`),

		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "forgespec": {
      "type": "local",
      "command": [
        "npx",
        "-y",
        "forgespec-mcp"
      ],
      "enabled": true
    }
  }
}
`),

		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "forgespec": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "forgespec-mcp"
      ]
    }
  }
}
`),

		TOMLCommand: "npx",
		TOMLArgs:    []string{"-y", "forgespec-mcp"},
	}
}
