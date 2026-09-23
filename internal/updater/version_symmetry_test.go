package updater

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type symmetryTransport struct {
	status int
	body   string
}

func (t symmetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Status:     fmt.Sprintf("%d %s", t.status, http.StatusText(t.status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}

// newSymmetryClient serves a canned /releases/latest payload through an in-memory
// transport so the symmetry assertions never touch the network.
func newSymmetryClient(tag string) *Client {
	body := fmt.Sprintf(
		`{"tag_name":%q,"name":%q,"published_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/r","assets":[]}`,
		tag, tag,
	)
	c := New("symmetry/repo")
	c.HTTPClient = &http.Client{Transport: symmetryTransport{status: http.StatusOK, body: body}}
	return c
}

func TestVersionSymmetry(t *testing.T) {
	cleanup := SetTrustedKeysForTesting([]TrustedKey{{ID: "sym-key", PublicKey: make([]byte, ed25519.PublicKeySize)}})
	defer cleanup()

	t.Run("dirty build is not announced an update", func(t *testing.T) {
		rel, hasUpdate, err := newSymmetryClient("v0.5.0").CheckLatest(context.Background(), "v0.4.50+dirty")
		if !errors.Is(err, ErrNonCanonicalVersion) {
			t.Fatalf("expected ErrNonCanonicalVersion, got %v", err)
		}
		if !errors.Is(err, ErrInvalidVersionFormat) {
			t.Errorf("typed sentinel must wrap the canonical parse cause, got %v", err)
		}
		if hasUpdate {
			t.Error("dirty build must not be announced an update")
		}
		if rel != nil && rel.TagName != "v0.5.0" {
			t.Errorf("unexpected release payload: %+v", rel)
		}
	})

	t.Run("pre-release suffix is not announced an update", func(t *testing.T) {
		_, hasUpdate, err := newSymmetryClient("v0.5.0").CheckLatest(context.Background(), "v0.4.50-rc.1")
		if !errors.Is(err, ErrNonCanonicalVersion) {
			t.Fatalf("expected ErrNonCanonicalVersion, got %v", err)
		}
		if hasUpdate {
			t.Error("pre-release current version must not be announced an update")
		}
	})

	t.Run("non-canonical latest fails closed against canonical current", func(t *testing.T) {
		_, hasUpdate, err := newSymmetryClient("nightly").CheckLatest(context.Background(), "v0.4.9")
		if !errors.Is(err, ErrNonCanonicalVersion) {
			t.Fatalf("expected ErrNonCanonicalVersion, got %v", err)
		}
		if hasUpdate {
			t.Error("non-canonical candidate must never be announced as an update")
		}
	})

	t.Run("canonical pair still compares strictly", func(t *testing.T) {
		rel, hasUpdate, err := newSymmetryClient("v0.5.0").CheckLatest(context.Background(), "v0.4.9")
		if err != nil {
			t.Fatalf("unexpected error for canonical pair: %v", err)
		}
		if !hasUpdate {
			t.Fatal("expected an update for v0.4.9 -> v0.5.0")
		}
		if rel == nil || rel.TagName != "v0.5.0" {
			t.Fatalf("unexpected release: %+v", rel)
		}
	})

	t.Run("dev and unknown report no update without error", func(t *testing.T) {
		for _, current := range []string{"dev", "DEV", "unknown", ""} {
			_, hasUpdate, err := newSymmetryClient("v0.5.0").CheckLatest(context.Background(), current)
			if err != nil {
				t.Errorf("current %q: expected nil error, got %v", current, err)
			}
			if hasUpdate {
				t.Errorf("current %q: expected hasUpdate=false", current)
			}
		}
	})

	t.Run("apply keeps rejecting what check refuses", func(t *testing.T) {
		err := newSymmetryClient("v0.5.0").ApplyUpdate(context.Background(), "v0.4.50+dirty", &Release{TagName: "v0.5.0"})
		if err == nil {
			t.Fatal("apply must reject a non-canonical current version")
		}
	})
}
