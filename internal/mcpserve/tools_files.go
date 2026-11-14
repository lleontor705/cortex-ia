package mcpserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
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
			Description: "Compute lowercase SHA-256 of exact UTF-8 content (at most 1 MiB). No newline/Unicode normalization, persistence, or network access. Returns only sha256 and byte_length.",
			Schema:      schemaObject(props("content", schemaString("Exact content to hash")), "content"),
			Call:        contentHash,
		},
		{
			Name:        "cortex_ia_openspec_validate",
			Description: "Validate OpenSpec planning artifacts from the active workspace without exposing a shell.",
			Schema: schemaObject(props(
				"relative_directory", schemaString("Directory under the workspace to validate"),
				"workflow", schemaEnum("Planning workflow", "sdd-lite", "sdd-full", "decision-map"),
				"phase", schemaEnum("Planning phase", "integrated", "propose", "spec", "design", "tasks", "chart", "resolve"),
			), "relative_directory", "workflow", "phase"),
			Call: openspecValidate,
		},
		{
			Name:        "cortex_ia_change_archive",
			Description: "Close an approved SDD change after durable task approval and fingerprint verification (Planner only).",
			Schema: schemaObject(props(
				"board_id", schemaString("Board that owns the change"),
				"change_id", schemaString("SDD change identifier"),
				"workflow", schemaEnum("SDD workflow", "sdd-lite", "sdd-full"),
				"spec_plane", schemaEnum("Specification plane", "cortex", "openspec", "hybrid"),
			), "board_id", "change_id", "workflow", "spec_plane"),
			Call: changeArchive,
		},
		{
			Name:        "cortex_ia_openspec_write",
			Description: "Write a Markdown planning artifact under openspec/changes/. Returns the written path, byte count and SHA-256.",
			Schema: schemaObject(props(
				"relative_path", schemaString("Markdown path under openspec/changes/"),
				"content", schemaString("Exact UTF-8 file content"),
			), "relative_path", "content"),
			Call: openspecWrite,
		},
		{
			Name:        "cortex_ia_discovery_write",
			Description: "Write the project-local .cortex-ia/discovery.md profile. Returns the artifact path, byte count and SHA-256.",
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

func openspecValidate(ctx context.Context, s *session, args map[string]any) (string, error) {
	dir, err := reqString(args, "relative_directory")
	if err != nil {
		return "", err
	}
	workflow, err := reqString(args, "workflow")
	if err != nil {
		return "", err
	}
	phase, err := reqString(args, "phase")
	if err != nil {
		return "", err
	}
	argv := []string{"openspec", "validate", dir, "--workflow", workflow, "--phase", phase, "--json", "--project", s.workspace}
	stdout, runErr := s.runner.Run(ctx, argv, "")
	if runErr != nil {
		trimmed := strings.TrimSpace(stdout)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			return trimmed, nil
		}
		return "", runErr
	}
	return cleanOutput(stdout), nil
}

func changeArchive(ctx context.Context, s *session, args map[string]any) (string, error) {
	board, err := reqString(args, "board_id")
	if err != nil {
		return "", err
	}
	change, err := reqString(args, "change_id")
	if err != nil {
		return "", err
	}
	workflow, err := reqString(args, "workflow")
	if err != nil {
		return "", err
	}
	specPlane, err := reqString(args, "spec_plane")
	if err != nil {
		return "", err
	}
	argv := []string{"work", "archive", "--board", board, "--project", s.workspace, "--change", change, "--workflow", workflow, "--spec-plane", specPlane}
	stdout, err := s.runner.Run(ctx, argv, "")
	if err != nil {
		return "", err
	}
	return cleanOutput(stdout), nil
}

func openspecWrite(_ context.Context, s *session, args map[string]any) (string, error) {
	relPath, err := reqString(args, "relative_path")
	if err != nil {
		return "", err
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", errors.New("content is required")
	}
	const denied = "OpenSpec writes must target a Markdown file under openspec/changes/"
	relative := strings.ReplaceAll(relPath, "\\", "/")
	clean := path.Clean(relative)
	if (strings.HasPrefix(relative, "openspec/changes/") && !strings.HasPrefix(clean, "openspec/changes/")) ||
		strings.Contains(relative, "../") || strings.Contains(relative, "/..") || strings.HasPrefix(relative, "../") || relative == ".." {
		return "", errors.New(denied)
	}
	normalized := clean
	if !strings.HasPrefix(clean, "openspec/changes/") {
		normalized = path.Join("openspec/changes", clean)
	}
	if !strings.HasPrefix(normalized, "openspec/changes/") || !strings.HasSuffix(normalized, ".md") || strings.Contains(normalized, "..") {
		return "", errors.New(denied)
	}
	target := filepath.Join(s.workspace, filepath.FromSlash(normalized))
	allowedRoot := filepath.Join(s.workspace, "openspec", "changes") + string(filepath.Separator)
	if !strings.HasPrefix(foldCase(target), foldCase(allowedRoot)) {
		return "", errors.New("OpenSpec path escaped its managed root")
	}
	if !utf8.ValidString(content) {
		return "", errors.New("OpenSpec write requires well-formed Unicode for exact UTF-8 encoding")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if _, err := filemerge.WriteFileAtomic(target, []byte(content), 0o644); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(content))
	return encodeJSON(map[string]any{"written": clean, "bytes": len(content), "sha256": hex.EncodeToString(sum[:])})
}

// foldCase compares paths case-insensitively on the platforms whose default
// filesystems are case-insensitive.
func foldCase(value string) string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
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
