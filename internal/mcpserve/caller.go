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

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
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
	// bound is the caller binding consumed for the tool call being served.
	// serveWith handles one line at a time, so one slot is enough.
	bound *hostbridge.Caller
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

// bindCall consumes the hook binding written for this exact tool call. Calls
// without a task, or without a binding, leave bound nil.
func (s *session) bindCall(tool string, args map[string]any) error {
	s.bound = nil
	task := optString(args, "task_id")
	if task == "" {
		return nil
	}
	caller, err := hostbridge.ConsumeCaller(s.stateHome, task, tool, s.now())
	if errors.Is(err, hostbridge.ErrCallerUnbound) {
		return nil
	}
	if err != nil {
		return err
	}
	s.bound = &caller
	return nil
}

// boundCaller returns the caller bound to the call being served, if it is for taskID.
func (s *session) boundCaller(taskID string) (hostbridge.Caller, bool) {
	if s.bound == nil || s.bound.TaskID != taskID {
		return hostbridge.Caller{}, false
	}
	return *s.bound, true
}

// sessionKeyFor resolves the caller's session key for taskID from the binding
// of the current call. Only the agy host may fall back to its process-wide key.
func (s *session) sessionKeyFor(taskID string) (string, error) {
	if caller, ok := s.boundCaller(taskID); ok {
		return caller.SessionKey, nil
	}
	if s.host == hostbridge.HostAGY {
		return s.agyKey, nil
	}
	return "", fmt.Errorf("%w; start the role agent from the installed agent files so the cortex-ia hook binds the caller", hostbridge.ErrCallerUnbound)
}

// statusSessionKey labels a read-only status view. Read-only calls carry no
// binding, so the view reports the handle this server holds for the task; the
// mutating tools still require their own bound identity.
func (s *session) statusSessionKey(taskID string) string {
	if a := s.vault.get(taskID); a != nil {
		return a.sessionKey
	}
	key, _ := s.sessionKeyFor(taskID)
	return key
}

// callerRole returns the role recorded by the call's binding, or "" when none.
func (s *session) callerRole(taskID string) string {
	caller, ok := s.boundCaller(taskID)
	if !ok {
		return ""
	}
	return caller.Role
}
