package mcpserve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

// session carries everything one server process needs. The server cannot see
// which agent is calling, so identity comes from the pre-tool hook's binding
// file for the task; the agy host may fall back to one process-wide key.
type session struct {
	ctx         context.Context
	host        hostbridge.Host
	stateHome   string
	workspace   string
	runner      Runner
	agyKey      string
	interval    time.Duration
	now         func() time.Time
	vault       *vault
	projections map[string]projectionEntry
}

type projectionEntry struct {
	revision   int64
	projection map[string]any
}

func newSession(ctx context.Context, host hostbridge.Host) (*session, error) {
	stateHome, err := resolveStateHome()
	if err != nil {
		return nil, err
	}
	workspace := os.Getenv("CLAUDE_PROJECT_DIR")
	if workspace == "" {
		workspace, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve workspace: %w", err)
		}
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	s := newSessionWith(ctx, host, stateHome, workspace, execRunner{dir: workspace})
	if host == hostbridge.HostAGY {
		var raw [6]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, fmt.Errorf("generate agy session key: %w", err)
		}
		s.agyKey = "agy-mcp-" + hex.EncodeToString(raw[:])
	}
	return s, nil
}

func newSessionWith(ctx context.Context, host hostbridge.Host, stateHome, workspace string, runner Runner) *session {
	return &session{
		ctx:         ctx,
		host:        host,
		stateHome:   stateHome,
		workspace:   workspace,
		runner:      runner,
		interval:    heartbeatInterval,
		now:         time.Now,
		vault:       newVault(),
		projections: map[string]projectionEntry{},
	}
}

// resolveStateHome mirrors cortexStateHome in internal/app/work.go.
func resolveStateHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME")); value != "" {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve CORTEX_IA_HOME: %w", err)
		}
		return filepath.Clean(absolute), nil
	}
	return os.UserHomeDir()
}

// sessionKeyFor resolves the caller's session key for taskID from the fresh
// hook binding. Only the agy host may fall back to its process-wide key.
func (s *session) sessionKeyFor(taskID string) (string, error) {
	caller, err := hostbridge.ResolveCaller(s.stateHome, taskID, s.now())
	if err == nil {
		return caller.SessionKey, nil
	}
	if !errors.Is(err, hostbridge.ErrCallerUnbound) {
		return "", err
	}
	if s.host == hostbridge.HostAGY {
		return s.agyKey, nil
	}
	return "", fmt.Errorf("%w; start the role agent from the installed agent files so the cortex-ia hook binds the caller", err)
}

// callerRole returns the role recorded by a fresh binding, or "" when none.
func (s *session) callerRole(taskID string) string {
	caller, err := hostbridge.ResolveCaller(s.stateHome, taskID, s.now())
	if err != nil {
		return ""
	}
	return caller.Role
}
