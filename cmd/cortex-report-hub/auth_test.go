package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthComparesSecretsForTokenHeaders(t *testing.T) {
	hub := &HubServer{authUser: "admin", authPassword: "s3cret-value"}
	h := hub.requireAuth(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	cases := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{"bearer_match", "Authorization", "Bearer s3cret-value", http.StatusOK},
		{"cortex_key_match", "X-Cortex-Key", "s3cret-value", http.StatusOK},
		{"bearer_wrong", "Authorization", "Bearer s3cret-valu", http.StatusUnauthorized},
		{"cortex_key_prefix", "X-Cortex-Key", "s3cret", http.StatusUnauthorized},
		{"cortex_key_empty", "X-Cortex-Key", "", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
			req.Header.Set(tc.header, tc.value)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
