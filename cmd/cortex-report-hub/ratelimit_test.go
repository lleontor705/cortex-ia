package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestLimiter(perMinute int, now *time.Time) *rateLimiter {
	l := newRateLimiter(perMinute, false)
	l.now = func() time.Time { return *now }
	return l
}

func TestRateLimiterAllowsBurstThenDenies(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(30, &clock)
	for i := range 30 {
		if !l.allow("10.0.0.1") {
			t.Fatalf("request %d within the 30/min budget was denied", i+1)
		}
	}
	if l.allow("10.0.0.1") {
		t.Fatal("31st request inside one minute must be denied")
	}
}

func TestRateLimiterIsPerClient(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(30, &clock)
	for range 30 {
		l.allow("10.0.0.1")
	}
	if !l.allow("10.0.0.2") {
		t.Fatal("an exhausted client must not throttle another client")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(30, &clock)
	for range 30 {
		l.allow("10.0.0.1")
	}
	clock = clock.Add(2 * time.Second) // 0.5 tokens/s refills one token in two seconds
	if !l.allow("10.0.0.1") {
		t.Fatal("one token must refill after two seconds")
	}
	if l.allow("10.0.0.1") {
		t.Fatal("only one token refilled; the next request must be denied")
	}
}

func TestRateLimiterBoundsTrackedClients(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(30, &clock)
	l.maxClients = 2
	l.allow("10.0.0.1")
	l.allow("10.0.0.2")
	if l.allow("10.0.0.3") {
		t.Fatal("a new client must be denied while the table is full of active buckets")
	}
	clock = clock.Add(time.Minute) // both buckets refill completely and become evictable
	if !l.allow("10.0.0.3") {
		t.Fatal("idle buckets must be evicted to admit a new client")
	}
	if len(l.buckets) > l.maxClients {
		t.Fatalf("tracked clients = %d, want at most %d", len(l.buckets), l.maxClients)
	}
}

func TestClientKeyUsesRightmostForwardedFor(t *testing.T) {
	l := newRateLimiter(30, true)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
	r.RemoteAddr = "10.1.2.3:4444"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.7")
	if got := l.clientKey(r); got != "198.51.100.7" {
		t.Fatalf("clientKey = %q, want the proxy-appended entry", got)
	}

	direct := newRateLimiter(30, false)
	if got := direct.clientKey(r); got != "10.1.2.3" {
		t.Fatalf("without trusted proxy clientKey = %q, want RemoteAddr host", got)
	}
}

func TestRateLimitMiddlewareExemptsHealth(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(1, &clock)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := l.middleware("/health", ok)

	for range 5 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("health probe throttled with status %d", rec.Code)
		}
	}
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = %d, %d; want 200 then 429", first.Code, second.Code)
	}
}
