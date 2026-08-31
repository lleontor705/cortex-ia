package assets

import (
	"embed"
	"fmt"
)

// FS embeds every OpenCode copy-paste asset: the AGENTS.md system prompt,
// the opencode.jsonc settings template, shared workflow contracts, and the
// agent, command, skill, and plugin trees.
//
// The embedded file set is derived at runtime by walking this FS (see
// Inventory). Do not hardcode file catalogs that duplicate what the walk
// already proves.
//
//go:embed AGENTS.md opencode.jsonc all:agents all:commands all:plugins all:skills all:tui
var FS embed.FS

// Read reads the content of an embedded asset file.
func Read(name string) (string, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read asset %q: %w", name, err)
	}
	return string(data), nil
}
