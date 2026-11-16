// Package hostbridge holds the contract shared by the non-OpenCode host
// integrations: how a host session becomes a work-authority owner, how a hook
// tells the MCP server who is calling, and which role may call which tool.
package hostbridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/components/filemerge"
)

type Host string

const (
	HostClaude Host = "claude"
	HostAGY    Host = "agy"
)

// MCPServerName is the server key written into every host's MCP config. The
// tool names it serves keep the cortex_ia_ prefix so prompt text matches them
// verbatim on every host.
const MCPServerName = "cortex_ia"

// CallerMaxAge bounds how long a hook-written binding authorizes the MCP call
// it was written for.
const CallerMaxAge = 60 * time.Second

var (
	sessionKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)
	unsafeKeyRunes    = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
)

func ParseHost(value string) (Host, error) {
	switch Host(strings.ToLower(strings.TrimSpace(value))) {
	case HostClaude:
		return HostClaude, nil
	case HostAGY:
		return HostAGY, nil
	}
	return "", fmt.Errorf("unknown host %q (valid: claude, agy)", value)
}

// SessionKey derives the host session identity used as the work-authority
// session id. agentID is empty for the main thread.
func SessionKey(host Host, sessionID, agentID string) (string, error) {
	sessionID = strings.Trim(unsafeKeyRunes.ReplaceAllString(strings.TrimSpace(sessionID), "-"), "-")
	agentID = strings.Trim(unsafeKeyRunes.ReplaceAllString(strings.TrimSpace(agentID), "-"), "-")
	if sessionID == "" {
		return "", errors.New("host session id is required")
	}
	prefix := "cc-"
	if host == HostAGY {
		prefix = "agy-"
	}
	key := prefix + sessionID
	if agentID != "" {
		key += "-" + agentID
	}
	if !sessionKeyPattern.MatchString(key) {
		return "", errors.New("host session id cannot form a valid session key")
	}
	return key, nil
}

// Owner is the claim owner string for a session key. The opencode-session
// prefix is the historical name of the host-session namespace that
// internal/delegation verifies leases against; every host shares it.
func Owner(sessionKey string) string { return "opencode-session:" + sessionKey }

// Caller is the non-secret identity a pre-tool hook records for one restricted
// MCP call. A binding is keyed by (task, tool) and consumed by the server on
// the call it was written for; no host payload carries a call id the MCP
// server can see, so the tool name is the narrowest key both sides share.
type Caller struct {
	Host       Host      `json:"host"`
	SessionKey string    `json:"session_key"`
	Role       string    `json:"role,omitempty"`
	TaskID     string    `json:"task_id"`
	Tool       string    `json:"tool"`
	BoundAt    time.Time `json:"bound_at"`
}

func callerPath(stateHome, taskID, tool string) string {
	sum := sha256.Sum256([]byte(taskID + "\x00" + tool))
	return filepath.Join(stateHome, ".cortex-ia", "run", "callers", hex.EncodeToString(sum[:])+".json")
}

// BindCaller records the caller for the next call of caller.Tool on caller.TaskID.
func BindCaller(stateHome string, caller Caller) error {
	if strings.TrimSpace(caller.TaskID) == "" || strings.TrimSpace(caller.Tool) == "" || !sessionKeyPattern.MatchString(caller.SessionKey) {
		return errors.New("caller binding requires a task id, a tool name and a valid session key")
	}
	raw, err := json.Marshal(caller)
	if err != nil {
		return err
	}
	path := callerPath(stateHome, caller.TaskID, caller.Tool)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_, err = filemerge.WriteFileAtomic(path, raw, 0o600)
	return err
}

// ErrCallerUnbound means no fresh hook binding exists for the task and tool.
var ErrCallerUnbound = errors.New("CALLER_UNBOUND: no fresh host caller binding for this call; the cortex-ia pre-tool hook did not run for this call")

// ConsumeCaller returns the fresh binding for (taskID, tool) and deletes it, so
// one hook run authorizes exactly one server call.
func ConsumeCaller(stateHome, taskID, tool string, now time.Time) (Caller, error) {
	path := callerPath(stateHome, taskID, tool)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Caller{}, ErrCallerUnbound
	}
	if err != nil {
		return Caller{}, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Caller{}, fmt.Errorf("consume caller binding: %w", err)
	}
	var caller Caller
	if err := json.Unmarshal(raw, &caller); err != nil {
		return Caller{}, fmt.Errorf("decode caller binding: %w", err)
	}
	if caller.TaskID != taskID || caller.Tool != tool || !sessionKeyPattern.MatchString(caller.SessionKey) {
		return Caller{}, ErrCallerUnbound
	}
	if age := now.Sub(caller.BoundAt); age < 0 || age > CallerMaxAge {
		return Caller{}, ErrCallerUnbound
	}
	return caller, nil
}
