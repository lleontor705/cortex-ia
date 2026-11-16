package mcpserve

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

const (
	testClaimToken = "CLAIMSECRET-0123456789abcdef"
	testLeaseToken = "LEASESECRET-0123456789abcdef"
	testSessionKey = "cc-sess1"
	testTask       = "t1"
	futureExpiry   = "2099-01-01T00:00:00Z"
)

type recordedCall struct {
	argv  []string
	stdin string
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []recordedCall
	respond func(argv []string, stdin string) (string, error)
}

func (f *fakeRunner) Run(_ context.Context, argv []string, stdin string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, recordedCall{argv: append([]string{}, argv...), stdin: stdin})
	f.mu.Unlock()
	return f.respond(argv, stdin)
}

func (f *fakeRunner) recorded() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedCall{}, f.calls...)
}

func (f *fakeRunner) lastWith(verb ...string) (recordedCall, bool) {
	calls := f.recorded()
	for i := len(calls) - 1; i >= 0; i-- {
		if hasPrefix(calls[i].argv, verb...) {
			return calls[i], true
		}
	}
	return recordedCall{}, false
}

func hasPrefix(argv []string, verb ...string) bool {
	if len(argv) < len(verb) {
		return false
	}
	for i := range verb {
		if argv[i] != verb[i] {
			return false
		}
	}
	return true
}

// cortexFake answers the CLI verbs the authority flows touch with receipts
// shaped like the real CLI's output.
func cortexFake(t *testing.T) *fakeRunner {
	t.Helper()
	return &fakeRunner{respond: func(argv []string, _ string) (string, error) {
		switch {
		case hasPrefix(argv, "work", "claim"):
			return `{"task_id":"` + testTask + `","owner":"opencode-session:` + testSessionKey + `","claim_token":"` + testClaimToken +
				`","reserved_files":[{"path":"a.txt","task_id":"` + testTask + `","lease_token":"` + testLeaseToken + `","expires_at":"` + futureExpiry + `"}]}`, nil
		case hasPrefix(argv, "work", "status"):
			return `{"task_id":"` + testTask + `","status":"in_progress","revision":3,"claim":{"owner":"opencode-session:` + testSessionKey + `","expires_at":"` + futureExpiry + `"}}`, nil
		case hasPrefix(argv, "work", "transition"):
			return `{"task_id":"` + testTask + `","status":"in_review","revision":4}`, nil
		case hasPrefix(argv, "work", "approvals"):
			return `[]`, nil
		case hasPrefix(argv, "board", "list"):
			return `[]`, nil
		}
		return `{}`, nil
	}}
}

func newTestSession(t *testing.T, runner Runner) *session {
	t.Helper()
	stateHome := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateHome)
	s := newSessionWith(context.Background(), hostbridge.HostClaude, stateHome, t.TempDir(), runner)
	s.interval = time.Hour
	return s
}

// bindCall simulates the pre-tool hook for the next call of tool on testTask.
func bindCall(t *testing.T, s *session, tool, sessionKey string) {
	t.Helper()
	err := hostbridge.BindCaller(s.stateHome, hostbridge.Caller{
		Host: hostbridge.HostClaude, SessionKey: sessionKey, TaskID: testTask, Tool: tool, BoundAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("bind caller: %v", err)
	}
}

func assertNoSecrets(t *testing.T, text string) {
	t.Helper()
	for _, secret := range []string{testClaimToken, testLeaseToken, "_token"} {
		if strings.Contains(text, secret) {
			t.Fatalf("result leaked %q: %s", secret, text)
		}
	}
}
