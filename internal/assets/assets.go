package assets

import (
	"embed"
	"fmt"
)

// FS embeds all OpenCode copy-paste assets.
//go:embed AGENTS.md all:_shared all:agents all:commands all:skills
var FS embed.FS

// Read reads the content of an embedded asset file.
func Read(name string) (string, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read asset %q: %w", name, err)
	}
	return string(data), nil
}
