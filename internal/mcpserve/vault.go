package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

const (
	heartbeatInterval = 5 * time.Minute
	// The CLI's controller renewal accepts at most 15m; a longer TTL fails every beat.
	heartbeatTTL = "30m"
	minTokenLen  = 8
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
		if err := s.renewAuthority(ctx, task, a); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.vault.mu.Lock()
			if s.vault.entries[task] == a {
				stopLocked(a, "maintenance_failed_manual_reconciliation_required: "+err.Error())
			}
			s.vault.mu.Unlock()
			return
		}
	}
}

type controllerReceipt struct {
	TaskID     string `json:"task_id"`
	Owner      string `json:"owner"`
	ExpiresAt  string `json:"expires_at"`
	LeaseCount int    `json:"lease_count"`
}

func (s *session) renewAuthority(ctx context.Context, task string, a *authority) error {
	s.vault.mu.Lock()
	if s.vault.entries[task] != a {
		s.vault.mu.Unlock()
		return nil
	}
	owner := hostbridge.Owner(a.sessionKey)
	leases := make(map[string]string, len(a.leases))
	for p, token := range a.leases {
		leases[p] = token
	}
	stdin, err := json.Marshal(map[string]any{"claim_token": a.claimToken, "leases": leases})
	s.vault.mu.Unlock()
	if err != nil {
		return err
	}
	argv := []string{"work", "controller-renew", task, "--owner", owner, "--authority", "@stdin", "--ttl", heartbeatTTL}
	stdout, err := s.runner.Run(ctx, argv, string(stdin))
	if err != nil {
		return fmt.Errorf("controller renewal failed: %w", err)
	}
	var receipt controllerReceipt
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &receipt); err != nil {
		return errors.New("controller renewal unconfirmed")
	}
	expires, parseErr := time.Parse(time.RFC3339Nano, receipt.ExpiresAt)
	if receipt.TaskID != task || receipt.Owner != owner || receipt.LeaseCount != len(leases) || parseErr != nil || !expires.After(s.now()) {
		return errors.New("controller renewal unconfirmed")
	}
	return nil
}

func (v *vault) reason(a *authority) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return a.reason
}
