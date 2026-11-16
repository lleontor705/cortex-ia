package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
)

const (
	heartbeatInterval = 5 * time.Minute
	// The CLI's controller renewal accepts a TTL up to 30m
	// (internal/delegation/work_controller_authority.go); the claim must outlive
	// a missed beat, so a shorter TTL would expire it between renewals.
	heartbeatTTL = "30m"
	// idleRenewalWindow stops renewal once the owner has made no identity-bearing
	// call for this long. It matches the CLI stale-progress window, so a claim
	// the server stopped renewing becomes reclaimable on the same clock.
	idleRenewalWindow = 15 * time.Minute
	minTokenLen       = 8
)

// authority is the in-memory handle for one claimed task. Tokens never leave
// the process; every field is guarded by vault.mu.
type authority struct {
	sessionKey string
	claimToken string
	leases     map[string]string
	cancel     context.CancelFunc
	active     bool
	reason     string
	lastSeen   time.Time
}

type vault struct {
	mu      sync.Mutex
	entries map[string]*authority
}

func newVault() *vault {
	return &vault{entries: map[string]*authority{}}
}

func (v *vault) get(task string) *authority {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.entries[task]
}

func (v *vault) put(task string, a *authority) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.entries[task] = a
}

// remove drops the handle and stops its heartbeat.
func (v *vault) remove(task string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if a := v.entries[task]; a != nil {
		stopLocked(a, "removed")
	}
	delete(v.entries, task)
}

// stop ends the heartbeat but keeps the handle so later results can report it.
func (v *vault) stop(task, reason string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if a := v.entries[task]; a != nil {
		stopLocked(a, reason)
	}
}

func stopLocked(a *authority, reason string) {
	a.active = false
	a.reason = reason
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

func (v *vault) stopAll() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, a := range v.entries {
		stopLocked(a, "server_stopped")
	}
}

func (v *vault) ownedBy(sessionKey string) []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var tasks []string
	for task, a := range v.entries {
		if a.sessionKey == sessionKey {
			tasks = append(tasks, task)
		}
	}
	sort.Strings(tasks)
	return tasks
}

func (v *vault) isActive(a *authority) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return a.active
}

// touch records an owner call for the task. Calls from other sessions do not
// count, and read-only calls carry no identity so they never reach here.
func (v *vault) touch(task, sessionKey string, now time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if a := v.entries[task]; a != nil && a.sessionKey == sessionKey {
		a.lastSeen = now
	}
}

// ownerIdle reports whether the handle's owner has gone silent for the window.
// It is the server's evidence of owner inactivity for reconcile.
func (v *vault) ownerIdle(task string, now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	a := v.entries[task]
	return a != nil && now.Sub(a.lastSeen) >= idleRenewalWindow
}

// stopIfIdle ends renewal for a handle whose owner has been silent for the
// window. It reports whether it stopped the handle.
func (v *vault) stopIfIdle(task string, a *authority, now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.entries[task] != a || !a.active || now.Sub(a.lastSeen) < idleRenewalWindow {
		return false
	}
	stopLocked(a, "idle_window_exceeded: no owner call within the idle window; renewal stopped and the claim will expire")
	return true
}

func (v *vault) leaseToken(a *authority, path string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return a.leases[path]
}

func (v *vault) setLease(a *authority, path, token string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	a.leases[path] = token
}

func (v *vault) dropLease(a *authority, path string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(a.leases, path)
}

func (v *vault) clearLeases(a *authority) {
	v.mu.Lock()
	defer v.mu.Unlock()
	a.leases = map[string]string{}
}

func (v *vault) leasePaths(a *authority) []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	paths := make([]string, 0, len(a.leases))
	for p := range a.leases {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// tokens lists every live secret so results can be scrubbed of them.
func (v *vault) tokens() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for _, a := range v.entries {
		if len(a.claimToken) >= minTokenLen {
			out = append(out, a.claimToken)
		}
		for _, token := range a.leases {
			if len(token) >= minTokenLen {
				out = append(out, token)
			}
		}
	}
	return out
}

// maintenanceStatus reports a heartbeat that stopped on its own failure.
func (v *vault) maintenanceStatus(task string) (map[string]any, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	a := v.entries[task]
	if a == nil || a.active {
		return nil, false
	}
	return map[string]any{"active": false, "reason": a.reason}, true
}

// startHeartbeat renews the claim and its lease set until stopped or until a
// renewal is not confirmed by the CLI.
func (s *session) startHeartbeat(task string, a *authority) {
	ctx, cancel := context.WithCancel(s.ctx)
	s.vault.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.cancel = cancel
	a.active = true
	a.reason = ""
	s.vault.mu.Unlock()
	go s.heartbeat(ctx, task, a)
}

func (s *session) heartbeat(ctx context.Context, task string, a *authority) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !s.beat(ctx, task, a) {
			return
		}
	}
}

// beat runs one maintenance tick and reports whether renewal should continue.
func (s *session) beat(ctx context.Context, task string, a *authority) bool {
	if s.vault.stopIfIdle(task, a, s.now()) {
		fmt.Fprintf(os.Stderr, "cortex-ia mcp: stopped renewing claim %s after %s without owner calls\n", task, idleRenewalWindow)
		return false
	}
	if _, err := s.renewAuthority(ctx, task, a); err != nil {
		if ctx.Err() != nil {
			return false
		}
		s.vault.mu.Lock()
		if s.vault.entries[task] == a {
			stopLocked(a, "maintenance_failed_manual_reconciliation_required: "+err.Error())
		}
		s.vault.mu.Unlock()
		return false
	}
	return true
}

type controllerReceipt struct {
	TaskID     string `json:"task_id"`
	Owner      string `json:"owner"`
	ExpiresAt  string `json:"expires_at"`
	LeaseCount int    `json:"lease_count"`
}

// renewAuthority renews the claim and its full lease set in one CLI call. The
// zero receipt means the handle was replaced, so the caller renewed nothing.
func (s *session) renewAuthority(ctx context.Context, task string, a *authority) (controllerReceipt, error) {
	s.vault.mu.Lock()
	if s.vault.entries[task] != a {
		s.vault.mu.Unlock()
		return controllerReceipt{}, nil
	}
	owner := hostbridge.Owner(a.sessionKey)
	leases := make(map[string]string, len(a.leases))
	for p, token := range a.leases {
		leases[p] = token
	}
	stdin, err := json.Marshal(map[string]any{"claim_token": a.claimToken, "leases": leases})
	s.vault.mu.Unlock()
	if err != nil {
		return controllerReceipt{}, err
	}
	argv := []string{"work", "controller-renew", task, "--owner", owner, "--authority", "@stdin", "--ttl", heartbeatTTL}
	stdout, err := s.runner.Run(ctx, argv, string(stdin))
	if err != nil {
		return controllerReceipt{}, fmt.Errorf("controller renewal failed: %w", err)
	}
	var receipt controllerReceipt
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &receipt); err != nil {
		return controllerReceipt{}, errors.New("controller renewal unconfirmed")
	}
	expires, parseErr := time.Parse(time.RFC3339Nano, receipt.ExpiresAt)
	if receipt.TaskID != task || receipt.Owner != owner || receipt.LeaseCount != len(leases) || parseErr != nil || !expires.After(s.now()) {
		return controllerReceipt{}, errors.New("controller renewal unconfirmed")
	}
	return receipt, nil
}

func (v *vault) reason(a *authority) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return a.reason
}
