package mcpserve

import (
	"context"
	"strings"
	"testing"
	"time"
)

// claimWithClock opens a claim with the real clock, then hands the server a
// controllable clock anchored at the claim time.
func claimWithClock(t *testing.T) (*session, *fakeRunner, *authority, *time.Time) {
	t.Helper()
	runner := cortexFake(t)
	base := runner.respond
	runner.respond = func(argv []string, stdin string) (string, error) {
		if hasPrefix(argv, "work", "controller-renew") {
			return `{"task_id":"` + testTask + `","owner":"opencode-session:` + testSessionKey +
				`","expires_at":"` + futureExpiry + `","lease_count":1}`, nil
		}
		return base(argv, stdin)
	}
	s := newTestSession(t, runner)
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)
	if _, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatal("claim setup failed")
	}
	a := s.vault.get(testTask)
	if a == nil {
		t.Fatal("claim handle missing")
	}
	clock := a.lastSeen
	s.now = func() time.Time { return clock }
	return s, runner, a, &clock
}

func renewCount(runner *fakeRunner) int {
	n := 0
	for _, c := range runner.recorded() {
		if hasPrefix(c.argv, "work", "controller-renew") {
			n++
		}
	}
	return n
}

func TestIdleClaimStopsRenewingAfterWindow(t *testing.T) {
	s, runner, a, clock := claimWithClock(t)
	base := *clock
	ctx := context.Background()

	*clock = base.Add(idleRenewalWindow - time.Minute)
	if !s.beat(ctx, testTask, a) {
		t.Fatal("renewal stopped before the idle window elapsed")
	}
	if s.vault.ownerIdle(testTask, *clock) {
		t.Fatal("owner reported idle before the window")
	}

	*clock = base.Add(idleRenewalWindow + time.Minute)
	before := renewCount(runner)
	if s.beat(ctx, testTask, a) {
		t.Fatal("idle claim kept renewing")
	}
	if renewCount(runner) != before {
		t.Fatal("renewal sent after the idle window")
	}
	if !s.vault.ownerIdle(testTask, *clock) {
		t.Fatal("owner must read as inactive for reconcile after the window")
	}
	if !strings.Contains(s.vault.reason(a), "idle_window_exceeded") {
		t.Fatalf("stop reason = %q", s.vault.reason(a))
	}
}

func TestOwnerCallResetsIdleWindow(t *testing.T) {
	s, _, a, clock := claimWithClock(t)
	base := *clock

	*clock = base.Add(10 * time.Minute)
	s.vault.touch(testTask, testSessionKey, *clock)
	*clock = base.Add(20 * time.Minute)
	if !s.beat(context.Background(), testTask, a) {
		t.Fatal("owner call inside the window must keep renewal alive")
	}
	if s.vault.ownerIdle(testTask, *clock) {
		t.Fatal("owner read as idle right after a call")
	}
}

func TestCompactStatusReportsMaintenance(t *testing.T) {
	s, _, a, clock := claimWithClock(t)
	base := *clock

	text, isError := call(t, s, "cortex_ia_work_status", map[string]any{"task_id": testTask})
	if isError {
		t.Fatalf("status failed: %s", text)
	}
	if !strings.Contains(text, `"maintenance"`) || !strings.Contains(text, `"active": true`) {
		t.Fatalf("compact status lacks active maintenance: %s", text)
	}
	assertNoSecrets(t, text)

	*clock = base.Add(idleRenewalWindow + time.Minute)
	s.beat(context.Background(), testTask, a)
	text, _ = call(t, s, "cortex_ia_work_status", map[string]any{"task_id": testTask})
	if !strings.Contains(text, `"active": false`) || !strings.Contains(text, "idle_window_exceeded") {
		t.Fatalf("compact status hides stopped renewal: %s", text)
	}
	assertNoSecrets(t, text)
}
