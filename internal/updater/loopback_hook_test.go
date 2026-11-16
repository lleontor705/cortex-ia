package updater

import (
	"net/url"
	"testing"
)

// Test binaries admit loopback origins so httptest servers can stand in for GitHub.
// Production binaries never execute this init, so loopback stays rejected there.
func init() {
	loopbackAllowed = true
}

func TestLoopbackOriginsRejectedWithoutTestHook(t *testing.T) {
	loopbackAllowed = false
	t.Cleanup(func() { loopbackAllowed = true })

	for _, raw := range []string{
		"http://127.0.0.1:8080/cortex-ia.tar.gz",
		"https://localhost:8443/cortex-ia.tar.gz",
		"http://localhost/cortex-ia.tar.gz",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if IsAllowedURL(u) {
			t.Errorf("IsAllowedURL(%q) = true without the test hook; want false", raw)
		}
	}
}
