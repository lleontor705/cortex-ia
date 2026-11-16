package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimitPerMinute is the per-client request budget for every route except /health.
const rateLimitPerMinute = 30

// rateLimiter is an in-memory, per-client token bucket. State is per process:
// a restart resets every bucket, which is acceptable for abuse throttling.
type rateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	capacity   float64
	refillRate float64 // tokens per second
	maxClients int
	trustProxy bool
	now        func() time.Time
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter allows perMinute requests per client, with a burst of the same size.
//
// trustProxy selects the client key from the rightmost X-Forwarded-For entry,
// which the hub's edge proxy appends. Without a trusted proxy in front, that
// header is client-controlled and the limit can be sidestepped, so only enable
// it behind a proxy that overwrites or appends to the header.
func newRateLimiter(perMinute int, trustProxy bool) *rateLimiter {
	return &rateLimiter{
		buckets:    make(map[string]*tokenBucket),
		capacity:   float64(perMinute),
		refillRate: float64(perMinute) / 60,
		maxClients: 10000,
		trustProxy: trustProxy,
		now:        time.Now,
	}
}

// allow spends one token for key. A new key that arrives while the table is
// full, after idle buckets are evicted, is denied so memory stays bounded.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxClients {
			l.evictIdle(now)
		}
		if len(l.buckets) >= l.maxClients {
			return false
		}
		b = &tokenBucket{tokens: l.capacity, last: now}
		l.buckets[key] = b
	}

	b.tokens = min(l.capacity, b.tokens+now.Sub(b.last).Seconds()*l.refillRate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evictIdle drops buckets that have refilled completely: they carry no state
// that a fresh bucket would not also grant.
func (l *rateLimiter) evictIdle(now time.Time) {
	for key, b := range l.buckets {
		refilled := b.tokens + now.Sub(b.last).Seconds()*l.refillRate
		if refilled >= l.capacity {
			delete(l.buckets, key)
		}
	}
}

func (l *rateLimiter) clientKey(r *http.Request) string {
	if l.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
				return last
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// middleware throttles every request except exemptPath, which is reserved for
// the platform health probe.
func (l *rateLimiter) middleware(exemptPath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == exemptPath || l.allow(l.clientKey(r)) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", "2")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	})
}
