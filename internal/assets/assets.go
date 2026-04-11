// Package assets provides embedded filesystem access to all injectable content:
// skills, orchestrator prompts, conventions, commands, and protocols.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:skills all:generic all:opencode all:gga
var FS embed.FS

// Read returns the content of an embedded asset file.
func Read(path string) (string, error) {
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return "", fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	return string(data), nil
}

// ReadBytes returns the raw bytes of an embedded asset file.
func ReadBytes(path string) ([]byte, error) {
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return nil, fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	return data, nil
}

// ListDir returns all entries in an embedded directory.
func ListDir(path string) ([]fs.DirEntry, error) {
	return fs.ReadDir(FS, path)
}
