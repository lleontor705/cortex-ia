package mcpserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const (
	maxHashBytes      = 1 << 20
	maxDiscoveryBytes = 128 << 10
	discoveryHeading  = "# Cortex-IA Project Discovery"
)

func fileTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_content_hash",
			Description: "Compute the lowercase SHA-256 and byte length of exact UTF-8 content (max 1 MiB). Read-only: no normalization, storage or network. Larger input fails.",
			Schema:      schemaObject(props("content", schemaString("Exact content to hash")), "content"),
			Call:        contentHash,
		},
		{
			Name:        "cortex_ia_discovery_write",
			Description: "Write the project .cortex-ia/discovery.md report (discovery role only). Content must start with \"# Cortex-IA Project Discovery\" and be at most 128 KiB. Returns path, bytes and sha256.",
			Schema:      schemaObject(props("content", schemaString("Discovery report starting with \"# Cortex-IA Project Discovery\"")), "content"),
			Call:        discoveryWrite,
		},
	}
}

func contentHash(_ context.Context, _ *session, args map[string]any) (string, error) {
	content, ok := args["content"].(string)
	if !ok {
		return "", errors.New("content is required")
	}
	if len(content) > maxHashBytes {
		return "", errors.New("content hash input exceeds 1 MiB (1048576 UTF-8 bytes)")
	}
	if !utf8.ValidString(content) {
		return "", errors.New("content hash requires well-formed Unicode for exact UTF-8 encoding")
	}
	sum := sha256.Sum256([]byte(content))
	return encodeJSON(map[string]any{"sha256": hex.EncodeToString(sum[:]), "byte_length": len(content)})
}

func discoveryWrite(_ context.Context, s *session, args map[string]any) (string, error) {
	raw, ok := args["content"].(string)
	if !ok {
		return "", errors.New("content is required")
	}
	content := strings.TrimSpace(raw)
	output := content + "\n"
	if !strings.HasPrefix(content, discoveryHeading) {
		return "", fmt.Errorf("discovery report must start with the canonical heading %q", discoveryHeading)
	}
	if len(output) == 0 || len(output) > maxDiscoveryBytes {
		return "", errors.New("discovery report must contain 1-131072 UTF-8 bytes")
	}
	directory := filepath.Join(s.workspace, ".cortex-ia")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	if _, err := filemerge.WriteFileAtomic(filepath.Join(directory, "discovery.md"), []byte(output), 0o644); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(output))
	return encodeJSON(map[string]any{
		"artifact": ".cortex-ia/discovery.md",
		"bytes":    len(output),
		"sha256":   hex.EncodeToString(sum[:]),
	})
}
