package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"unicode/utf8"
)

const maxResultBytes = 48 << 10

type tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Call        func(ctx context.Context, s *session, args map[string]any) (string, error)
}

// allTools lists every tool the server exposes. Names and argument names match
// the OpenCode plugin so shared prompts keep working.
func allTools() []tool {
	var tools []tool
	tools = append(tools, fileTools()...)
	tools = append(tools, artifactTools()...)
	tools = append(tools, boardTools()...)
	tools = append(tools, workReadTools()...)
	tools = append(tools, authorityTools()...)
	return tools
}

func findTool(name string) (tool, bool) {
	for _, t := range allTools() {
		if t.Name == name {
			return t, true
		}
	}
	return tool{}, false
}

// callTool runs one tool and returns its text plus whether it failed. Tool
// failures are results with isError set, never JSON-RPC errors.
func (s *session) callTool(ctx context.Context, name string, args map[string]any) (string, bool) {
	t, ok := findTool(name)
	if !ok {
		return fmt.Sprintf("unknown tool: %s", name), true
	}
	text, err := s.invoke(ctx, t, args)
	if err != nil {
		return s.redact(err.Error()), true
	}
	return s.finalize(text, args), false
}

func (s *session) invoke(ctx context.Context, t tool, args map[string]any) (text string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			text, err = "", fmt.Errorf("%s failed: internal error", t.Name)
		}
	}()
	return t.Call(ctx, s, args)
}

// finalize scrubs live tokens, attaches a stopped heartbeat's status for the
// task, and bounds the size of the result.
func (s *session) finalize(text string, args map[string]any) string {
	text = s.redact(text)
	if task, _ := args["task_id"].(string); task != "" {
		if status, ok := s.vault.maintenanceStatus(task); ok {
			text = attachField(text, "maintenance", status)
		}
	}
	return boundText(text)
}

func (s *session) redact(text string) string {
	for _, token := range s.vault.tokens() {
		text = strings.ReplaceAll(text, token, "[redacted]")
	}
	return text
}

func boundText(text string) string {
	if len(text) <= maxResultBytes {
		return text
	}
	cut := maxResultBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + fmt.Sprintf("\n…[truncated: %d bytes omitted; narrow the query]", len(text)-cut)
}

func attachField(text, key string, value any) string {
	decoded, err := decodeJSON(text)
	if err != nil {
		return text
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return text
	}
	object[key] = value
	encoded, err := encodeJSON(object)
	if err != nil {
		return text
	}
	return encoded
}

// cleanOutput turns CLI stdout into a token-free result. Non-JSON output is
// returned trimmed; token scrubbing still applies in finalize.
func cleanOutput(stdout string) string {
	text := strings.TrimSpace(stdout)
	decoded, err := decodeJSON(text)
	if err != nil {
		return text
	}
	encoded, err := encodeJSON(stripTokens(decoded))
	if err != nil {
		return text
	}
	return encoded
}

func decodeJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return value, nil
}

func encodeJSON(value any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// stripTokens removes claim and lease tokens from every object in value.
func stripTokens(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "claim_token" || key == "lease_token" {
				delete(typed, key)
				continue
			}
			typed[key] = stripTokens(child)
		}
		return typed
	case []any:
		for i := range typed {
			typed[i] = stripTokens(typed[i])
		}
		return typed
	}
	return value
}

// workPath normalises a workspace-relative path the way the CLI's lease keys
// are compared: forward slashes, cleaned, case-folded on darwin and windows.
func workPath(value string) string {
	normalized := strings.TrimPrefix(path.Clean(strings.ReplaceAll(value, "\\", "/")), "./")
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}
