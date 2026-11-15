package targets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

const (
	opencodeContractsRef = "~/.cortex-ia/opencode/contracts"
	globalProtocolHeader = "# Global protocol"
)

var autoinvokeLine = regexp.MustCompile(`^(?:[\w./-]*/)?autoinvoke:\s*false$`)

// splitFrontmatter splits a leading ---delimited YAML block from the body.
func splitFrontmatter(data []byte) (front string, body string, ok bool) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", "", false
	}
	rest := text[len("---\n"):]
	if strings.HasPrefix(rest, "---\n") {
		return "", rest[len("---\n"):], true
	}
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return rest[:i], rest[i+len("\n---\n"):], true
	}
	if strings.HasSuffix(rest, "\n---") {
		return rest[:len(rest)-len("\n---")], "", true
	}
	return "", "", false
}

// frontmatterValue returns a top-level scalar from YAML frontmatter with any
// surrounding quotes removed.
func frontmatterValue(front, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(front, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		v := strings.TrimSpace(line[len(prefix):])
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			if u, err := strconv.Unquote(v); err == nil {
				return u
			}
			return v[1 : len(v)-1]
		}
		if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
			return v[1 : len(v)-1]
		}
		return v
	}
	return ""
}

// yamlQuote renders s as a double-quoted YAML scalar.
func yamlQuote(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}

// rewriteContractsPath points OpenCode contract references at the host's own
// contract root.
func rewriteContractsPath(text, target string) string {
	return strings.ReplaceAll(text, opencodeContractsRef, "~/.cortex-ia/"+target+"/contracts")
}

func rewriteContractsBytes(data []byte, target string) []byte {
	return bytes.ReplaceAll(data, []byte(opencodeContractsRef), []byte("~/.cortex-ia/"+target+"/contracts"))
}

// rewriteAutoinvoke replaces (or with an empty replacement, removes) the
// autoinvoke:false marker in a skill's frontmatter. A replacement is appended
// at column zero so it stays valid when the marker was nested under metadata.
func rewriteAutoinvoke(src []byte, replacement string) []byte {
	front, body, ok := splitFrontmatter(src)
	if !ok {
		return src
	}
	lines := strings.Split(front, "\n")
	kept := make([]string, 0, len(lines)+1)
	found := false
	for _, ln := range lines {
		if autoinvokeLine.MatchString(strings.TrimSpace(ln)) {
			found = true
			continue
		}
		kept = append(kept, ln)
	}
	if !found {
		return src
	}
	if replacement != "" {
		kept = append(kept, replacement)
	}
	return []byte("---\n" + strings.Join(kept, "\n") + "\n---\n" + body)
}

// composeRolePrompt joins a host preamble, a role body and the global protocol
// document. Every role carries the global document so subagents see the
// universal rules as well as the orchestrator.
func composeRolePrompt(preamble, body string, globalDoc []byte) string {
	var sb strings.Builder
	sb.WriteString(preamble)
	sb.WriteString("\n\n")
	sb.WriteString(strings.Trim(body, "\n"))
	sb.WriteString("\n\n")
	sb.WriteString(globalProtocolHeader)
	sb.WriteString("\n\n")
	sb.WriteString(strings.TrimRight(string(globalDoc), "\n"))
	sb.WriteString("\n")
	return sb.String()
}

// contractFiles maps every embedded shared contract to its host destination.
func contractFiles(inventory []assets.File, home string, target TargetID) ([]File, error) {
	var out []File
	for _, f := range inventory {
		if f.Kind != assets.KindShared {
			continue
		}
		data, err := assets.ReadBytes(f.Path)
		if err != nil {
			return nil, err
		}
		dest := filepath.Join(home, ".cortex-ia", string(target), "contracts", path.Base(f.Path))
		out = append(out, File{Path: dest, Content: rewriteContractsBytes(data, string(target))})
	}
	return out, nil
}

// roleParts loads an embedded role prompt and returns its description and body.
func roleParts(role string) (description, body string, err error) {
	data, err := assets.ReadBytes("agents/" + role + ".md")
	if err != nil {
		return "", "", fmt.Errorf("read role %s: %w", role, err)
	}
	front, body, ok := splitFrontmatter(data)
	if !ok {
		return "", "", fmt.Errorf("role %s: missing frontmatter", role)
	}
	description = frontmatterValue(front, "description")
	if description == "" {
		return "", "", fmt.Errorf("role %s: empty description", role)
	}
	return description, body, nil
}

// commandParts returns an embedded command's description and body.
func commandParts(data []byte) (description, body string, err error) {
	front, body, ok := splitFrontmatter(data)
	if !ok {
		return "", "", fmt.Errorf("command: missing frontmatter")
	}
	return frontmatterValue(front, "description"), strings.TrimLeft(body, "\n"), nil
}
