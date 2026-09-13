package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateTrust(t *testing.T) {
	expectedMsg := "Authenticated update unavailable: no trusted release key is packaged"

	t.Run("check latest rejects with exact message when trust is absent", func(t *testing.T) {
		serverCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := New("dummy/repo")
		// Point to test server to prove no request is made
		client.HTTPClient = server.Client()

		rel, hasUpdate, err := client.CheckLatest(context.Background(), "v1.0.0")
		if err == nil {
			t.Fatal("expected error when trusted roots are absent, got nil")
		}
		if err.Error() != expectedMsg {
			t.Errorf("expected exact error %q, got %q", expectedMsg, err.Error())
		}
		if !errors.Is(err, ErrNoTrustedKey) {
			t.Errorf("expected errors.Is(err, ErrNoTrustedKey) to be true")
		}
		if hasUpdate {
			t.Error("trust absence must be unavailable, never already-up-to-date")
		}
		if rel != nil {
			t.Errorf("expected nil release, got %+v", rel)
		}
		if serverCalled {
			t.Error("HTTP request was dispatched despite absent trust")
		}
	})

	t.Run("apply update rejects with exact message when trust is absent", func(t *testing.T) {
		client := New("dummy/repo")
		fakeRel := &Release{
			TagName: "v2.0.0",
			Assets:  []ReleaseAsset{{Name: "cortex-ia_2.0.0_windows_amd64.zip", DownloadURL: "http://invalid"}},
		}

		err := client.ApplyUpdate(context.Background(), "v1.0.0", fakeRel)
		if err == nil {
			t.Fatal("expected error when trusted roots are absent, got nil")
		}
		if err.Error() != expectedMsg {
			t.Errorf("expected exact error %q, got %q", expectedMsg, err.Error())
		}
		if !errors.Is(err, ErrNoTrustedKey) {
			t.Errorf("expected errors.Is(err, ErrNoTrustedKey) to be true")
		}
	})

	t.Run("no constructor bypass", func(t *testing.T) {
		// Bare struct uninitialized client
		bareClient := &Client{}
		_, _, err := bareClient.CheckLatest(context.Background(), "v1.0.0")
		if err == nil || err.Error() != expectedMsg {
			t.Errorf("expected bare client to fail with %q, got %v", expectedMsg, err)
		}

		err = bareClient.ApplyUpdate(context.Background(), "v1.0.0", &Release{TagName: "v1.0.1"})
		if err == nil || err.Error() != expectedMsg {
			t.Errorf("expected bare client ApplyUpdate to fail with %q, got %v", expectedMsg, err)
		}
	})

	t.Run("nil or mutated release rejection and dev version rejection", func(t *testing.T) {
		cleanup := SetTrustedKeysForTesting([]TrustedKey{{ID: "test-key", PublicKey: []byte("pubkey")}})
		defer cleanup()

		client := New("dummy/repo")

		// 1. Nil release
		if err := client.ApplyUpdate(context.Background(), "v1.0.0", nil); err == nil {
			t.Error("expected error for nil release")
		}

		// 2. Empty TagName (mutated)
		if err := client.ApplyUpdate(context.Background(), "v1.0.0", &Release{TagName: ""}); err == nil {
			t.Error("expected error for empty TagName")
		}

		// 3. Dev version rejection
		for _, devVer := range []string{"dev", "DEV", "", "  "} {
			err := client.ApplyUpdate(context.Background(), devVer, &Release{TagName: "v1.0.0"})
			if err == nil || !strings.Contains(err.Error(), "cannot apply update to dev or unknown version") {
				t.Errorf("expected dev/unknown version rejection for %q, got: %v", devVer, err)
			}
		}
	})
}
