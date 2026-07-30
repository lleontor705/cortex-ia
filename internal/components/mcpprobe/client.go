// Package mcpprobe probes versioned MCP service capability endpoints without
// inferring support from package names, installed tools, or documentation.
package mcpprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, client *http.Client) Client {
	return Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (client Client) ProbeForgeSpec(ctx context.Context) (forgespec.CapabilitySnapshot, error) {
	parsed, err := url.Parse(client.baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("capability endpoint must use an absolute http or https URL")
	}
	httpClient := client.http
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/capabilities", nil)
	if err != nil {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("build capability probe: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("probe ForgeSpec capabilities: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("ForgeSpec capability endpoint returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	var snapshot forgespec.CapabilitySnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("decode ForgeSpec capability snapshot: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return forgespec.CapabilitySnapshot{}, fmt.Errorf("validate ForgeSpec capability snapshot: %w", err)
	}
	return snapshot, nil
}
