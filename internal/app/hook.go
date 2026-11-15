package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

const (
	hookPayloadLimit  = 2 << 20
	hookLeaseTimeout  = 5 * time.Second
	hookUsageMessage  = "usage: cortex-ia hook pre-tool --host <claude|agy> [--role <role>]"
	hookSubcommandPre = "pre-tool"
)

// runHook is the host pre-tool guard invoked by Claude Code and agy hooks.
// Internal failures fail closed through failHook, which exits the process
// because the host reads its verdict from the exit status and stdout.
func runHook(args []string) error {
	if len(args) == 0 || args[0] != hookSubcommandPre {
		return errors.New(hookUsageMessage)
	}
	host, role, err := parseHookFlags(args[1:])
	if err != nil {
		return fmt.Errorf("%w; %s", err, hookUsageMessage)
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, hookPayloadLimit))
	if err != nil {
		return failHook(host, fmt.Errorf("read hook payload: %w", err))
	}
	stdout, code, err := evaluateHook(host, role, raw)
	if err != nil {
		return failHook(host, err)
	}
	if len(stdout) > 0 {
		if _, err := os.Stdout.Write(stdout); err != nil {
			return failHook(host, fmt.Errorf("write hook decision: %w", err))
		}
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func parseHookFlags(args []string) (hostbridge.Host, string, error) {
	var hostValue, roleValue string
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		if name != "--host" && name != "--role" {
			return "", "", fmt.Errorf("unknown hook flag %q", args[i])
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if name == "--host" {
			hostValue = value
		} else {
			roleValue = value
		}
	}
	if strings.TrimSpace(hostValue) == "" {
		return "", "", errors.New("--host is required")
	}
	host, err := hostbridge.ParseHost(hostValue)
	if err != nil {
		return "", "", err
	}
	return host, strings.ToLower(strings.TrimSpace(roleValue)), nil
}

func evaluateHook(host hostbridge.Host, role string, raw []byte) ([]byte, int, error) {
	in, err := hostbridge.ParseHookPayload(host, role, raw)
	if err != nil {
		return nil, 0, err
	}
	home, err := cortexStateHome()
	if err != nil {
		return nil, 0, err
	}
	leases, closeStore, err := openHookLeases(home, host == hostbridge.HostAGY)
	if err != nil {
		return nil, 0, err
	}
	defer closeStore()
	decision, err := hostbridge.Decide(in, hostbridge.Deps{StateHome: home, Leases: leases, Now: time.Now})
	if err != nil {
		return nil, 0, err
	}
	stdout, code := hostbridge.RenderDecision(host, decision)
	return stdout, code, nil
}

// openHookLeases returns nil leases when no delegation database exists yet,
// so the decision layer can tell "no work authority here" from a failure.
func openHookLeases(home string, pathLevelFallback bool) (hostbridge.LeaseChecker, func(), error) {
	dbPath := delegation.DefaultDBPath(home)
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return nil, func() {}, nil
	} else if err != nil {
		return nil, func() {}, fmt.Errorf("stat delegation database: %w", err)
	}
	store, err := delegation.OpenStoreReadOnly(dbPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open delegation database: %w", err)
	}
	return hookStoreLeases{store: store, pathLevelFallback: pathLevelFallback}, func() { _ = store.Close() }, nil
}

func failHook(host hostbridge.Host, err error) error {
	stdout, code := hostbridge.RenderFailure(host, err)
	if host == hostbridge.HostClaude {
		fmt.Fprintf(os.Stderr, "cortex-ia hook error: %v\n", err)
	}
	if len(stdout) > 0 {
		_, _ = os.Stdout.Write(stdout)
	}
	os.Exit(code)
	return nil
}

type hookStoreLeases struct {
	store *delegation.Store
	// agy does not tell a hook which agent is calling, so a session-scoped miss
	// falls back to "this path is leased by some live task of this workspace".
	pathLevelFallback bool
}

func (s hookStoreLeases) VerifyLease(relPath, workspace, sessionKey string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hookLeaseTimeout)
	defer cancel()
	res, err := s.store.VerifySessionWorkLease(ctx, relPath, workspace, sessionKey)
	if err != nil {
		return false, "", err
	}
	if res.Valid || !s.pathLevelFallback {
		return res.Valid, res.Reason, nil
	}
	leased, err := s.store.HasLiveWorkspaceLease(ctx, relPath, workspace)
	if err != nil {
		return false, "", err
	}
	return leased, res.Reason, nil
}

func (s hookStoreLeases) HasActiveWork(workspace string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hookLeaseTimeout)
	defer cancel()
	return s.store.HasActiveWorkspaceWork(ctx, workspace)
}
