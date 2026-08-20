package filemerge

import (
	"fmt"
	"strings"
)

// UpsertTOMLBlock removes any existing [sectionHeader] block from the given TOML
// content and appends a fresh block with the provided content.
// This is a string-based helper (no TOML parser dependency).
func UpsertTOMLBlock(content, sectionHeader, blockContent string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	headerLine := "[" + sectionHeader + "]"

	var kept []string
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == headerLine {
			i++
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				if strings.HasPrefix(next, "[") && strings.HasSuffix(next, "]") {
					break
				}
				i++
			}
			continue
		}
		kept = append(kept, lines[i])
		i++
	}

	base := strings.TrimSpace(strings.Join(kept, "\n"))
	newBlock := headerLine + "\n" + blockContent

	if base == "" {
		return newBlock + "\n"
	}
	return base + "\n\n" + newBlock + "\n"
}

// UpsertMCPServerTOML is a convenience wrapper for upserting an MCP server block
// in Codex's config.toml format: [mcp_servers.<name>]
func UpsertMCPServerTOML(content, serverName, command string, args []string) string {
	sectionHeader := "mcp_servers." + serverName

	var sb strings.Builder
	fmt.Fprintf(&sb, "command = %q\n", command)
	if len(args) > 0 {
		sb.WriteString("args = [")
		for i, arg := range args {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q", arg)
		}
		sb.WriteString("]\n")
	}

	return UpsertTOMLBlock(content, sectionHeader, sb.String())
}

// UpsertTopLevelTOMLString inserts or replaces a top-level key = "value" pair
// in TOML content. The key is placed before the first [section] header.
func UpsertTopLevelTOMLString(content, key, value string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	lineValue := fmt.Sprintf("%s = %q", key, value)

	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	insertAt := len(cleaned)
	for i, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			insertAt = i
			break
		}
	}

	var out []string
	out = append(out, cleaned[:insertAt]...)
	out = append(out, lineValue)
	out = append(out, cleaned[insertAt:]...)

	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
