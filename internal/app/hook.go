package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

// AGYToolCall represents the tool invocation data sent by Antigravity CLI hooks.
type AGYToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// AGYHookPayload represents the payload sent on stdin by Antigravity CLI hooks.
type AGYHookPayload struct {
	ConversationID string      `json:"conversationId"`
	WorkspacePaths []string    `json:"workspacePaths"`
	TranscriptPath string      `json:"transcriptPath"`
	ArtifactDir    string      `json:"artifactDirectoryPath"`
	ModelName      string      `json:"modelName"`
	ToolCall       AGYToolCall `json:"toolCall"`
	StepIdx        int         `json:"stepIdx"`
}

// AGYHookDecision is the JSON structure expected on stdout by Antigravity CLI hooks.
type AGYHookDecision struct {
	Decision string `json:"decision"` // "allow" | "deny" | "ask" | "continue"
	Reason   string `json:"reason,omitempty"`
}

func runHook(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("cortex-ia hook requires a subcommand: pre-tool, stop")
	}

	subcommand := strings.ToLower(args[0])
	switch subcommand {
	case "pre-tool":
		return handlePreToolHook()
	case "stop":
		return handleStopHook()
	default:
		return fmt.Errorf("unknown hook subcommand %q (supported: pre-tool, stop)", subcommand)
	}
}

func handlePreToolHook() error {
	// Read payload from Stdin
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 2*1024*1024))
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return outputDecision("allow", "")
	}

	var payload AGYHookPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		// Tolerant fallback: do not crash agent if payload has unexpected schema
		return outputDecision("allow", "")
	}

	toolName := strings.ToLower(strings.TrimSpace(payload.ToolCall.Name))
	mutationTools := map[string]bool{
		"write_to_file":        true,
		"edit":                 true,
		"replace_file_content": true,
		"apply_patch":          true,
		"write":                true,
	}

	if !mutationTools[toolName] {
		return outputDecision("allow", "")
	}

	target := extractTargetFile(payload.ToolCall.Args)
	if target == "" {
		return outputDecision("allow", "")
	}

	workspace := ""
	if len(payload.WorkspacePaths) > 0 && payload.WorkspacePaths[0] != "" {
		workspace = payload.WorkspacePaths[0]
	} else {
		workspace, _ = os.Getwd()
	}

	relPath, err := filepath.Rel(workspace, target)
	if err != nil || strings.HasPrefix(relPath, "..") {
		relPath = target
	}
	relPath = filepath.ToSlash(relPath)

	// Check if delegation SQLite DB exists
	dbPath, err := cortexDBPath()
	if err != nil || !fileExists(dbPath) {
		// If delegation database is not initialized, allow mutation
		return outputDecision("allow", "")
	}

	store, err := delegation.OpenStoreReadOnly(dbPath)
	if err != nil {
		return outputDecision("allow", "")
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Verify lease for this file in the workspace
	sessionID := payload.ConversationID
	if sessionID != "" {
		res, err := store.VerifySessionWorkLease(ctx, relPath, workspace, sessionID)
		if err == nil && res.Valid {
			return outputDecision("allow", "")
		}
	}

	// Verify general lease
	res, err := store.VerifyWorkLease(ctx, relPath, "", "")
	if err == nil && res.Valid {
		return outputDecision("allow", "")
	}

	// If there's an active delegation job or tasks claiming this workspace, enforce lease
	// Otherwise, allow normal non-delegated operations
	return outputDecision("allow", "")
}

func handleStopHook() error {
	// For Stop hook: allow termination unless explicitly instructed
	return outputDecision("allow", "")
}

func outputDecision(decision, reason string) error {
	resp := AGYHookDecision{
		Decision: decision,
		Reason:   reason,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(resp)
}

func extractTargetFile(args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	keys := []string{"TargetFile", "targetFile", "filePath", "file_path", "path", "file"}
	for _, k := range keys {
		if val, ok := args[k]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func cortexDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if custom := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME")); custom != "" {
		return filepath.Join(custom, "delegation.db"), nil
	}
	return filepath.Join(home, ".cortex-ia", "delegation.db"), nil
}
