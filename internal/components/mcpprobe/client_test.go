package mcpprobe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
)

func TestClientRejectsAbsentAndMalformedCapabilityEndpoints(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "absent endpoint", status: http.StatusNotFound},
		{name: "malformed response", status: http.StatusOK, body: `{"schema_version":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			_, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background())
			if err == nil {
				t.Fatal("ProbeForgeSpec() error = nil")
			}
		})
	}
}

func TestClientDecodesVersionedCapabilityEvidence(t *testing.T) {
	now := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	body := `{
		"schema_version":"1.0.0","server_version":"2.0.0","protocol_version":"1.0.0","probe_status":"qualified",
		"capabilities":[{
			"id":"forgespec/task-cas","version":"1.0.0","provider":"forgespec","provider_version":"1.4.0",
			"interval":{"minimum":"1.0.0","maximum_tested":"1.0.0"},"evidence_class":"executable-probe",
			"evidence_ref":"probe://forgespec/task-cas","observed_at":"` + now.Add(-time.Minute).Format(time.RFC3339) + `",
			"fresh_until":"` + now.Add(time.Hour).Format(time.RFC3339) + `","confidence":1,"probe_id":"probe/forgespec/capabilities","enforcement":"mcp"
		}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/capabilities" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	snapshot, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background())
	if err != nil {
		t.Fatalf("ProbeForgeSpec() error = %v", err)
	}
	if len(snapshot.Capabilities) != 1 || snapshot.Capabilities[0].EvidenceRef == "" || snapshot.Capabilities[0].Version.String() != "1.0.0" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestClientRejectsNonHTTPBaseURL(t *testing.T) {
	_, err := NewClient("file:///tmp/capabilities", http.DefaultClient).ProbeForgeSpec(context.Background())
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientValidatesRawPublishedCapabilitiesBeforeTranslation(t *testing.T) {
	response := publishedResponse()
	server := capabilityServer(t, response)
	defer server.Close()

	snapshot, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background())
	if err != nil {
		t.Fatalf("ProbeForgeSpec() error = %v", err)
	}
	if snapshot.ProbeStatus != forgespec.ProbeQualified {
		t.Fatalf("probe status = %q, want qualified", snapshot.ProbeStatus)
	}
}

func TestClientRejectsRawPublishedCapabilitiesWithoutDirectV1Mode(t *testing.T) {
	response := publishedResponse()
	response.Modes = []string{"legacy"}
	server := capabilityServer(t, response)
	defer server.Close()

	if _, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background()); err == nil || !strings.Contains(err.Error(), "direct-v1 mode") {
		t.Fatalf("ProbeForgeSpec() error = %v, want direct-v1 mode failure", err)
	}
}

func TestClientRejectsRawPublishedCapabilitiesWithIncompleteP0(t *testing.T) {
	response := publishedResponse()
	response.Capabilities = response.Capabilities[:1]
	server := capabilityServer(t, response)
	defer server.Close()

	if _, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background()); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("ProbeForgeSpec() error = %v, want incomplete capability failure", err)
	}
}

func TestClientPreservesNormalizedCapabilitySnapshotTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":"1.0.0","server_version":"2.0.0","protocol_version":"1.0.0","probe_status":"unavailable"}`))
	}))
	defer server.Close()

	snapshot, err := NewClient(server.URL, server.Client()).ProbeForgeSpec(context.Background())
	if err != nil {
		t.Fatalf("ProbeForgeSpec() error = %v", err)
	}
	if snapshot.ProbeStatus != forgespec.ProbeUnavailable {
		t.Fatalf("probe status = %q, want unavailable", snapshot.ProbeStatus)
	}
}

func publishedResponse() forgespec.PublishedCapabilityResponse {
	capabilities := make([]forgespec.PublishedCapability, 0, len(forgespec.RequiredP0Capabilities()))
	for _, requirement := range forgespec.RequiredP0Capabilities() {
		id := strings.TrimPrefix(string(requirement.ID), "forgespec/")
		if requirement.ID == "forgespec/capabilities" {
			id = "forgespec.capabilities"
		}
		capabilities = append(capabilities, forgespec.PublishedCapability{
			ID:        id,
			Supported: forgespec.PublishedVersionRange{MinInclusive: "1.0.0", MaxExclusive: "2.0.0"},
			Selected:  "1.0.0",
		})
	}
	return forgespec.PublishedCapabilityResponse{
		Server: forgespec.ServerInfo{Name: "forgespec-mcp", Version: "1.4.0", APIVersion: "1.0.0"},
		Modes:  []string{"legacy", "direct-v1"}, Capabilities: capabilities,
		Compatibility: forgespec.PublishedCompatibility{Compatible: true, SelectedMode: "direct-v1"},
	}
}

func capabilityServer(t *testing.T, response forgespec.PublishedCapabilityResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}
