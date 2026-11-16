package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
)

func runProtocol(t *testing.T, s *session, input string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := serveWith(context.Background(), s, strings.NewReader(input), &out); err != nil {
		t.Fatalf("serveWith: %v", err)
	}
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response is not JSON %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestProtocolHandshakeListAndErrors(t *testing.T) {
	s := newTestSession(t, cortexFake(t))
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/unknown","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"cortex_ia_nope"}}`,
		`{"jsonrpc":"2.0","id":"x",`,
	}, "\n") + "\n"
	responses := runProtocol(t, s, input)
	if len(responses) != 6 {
		t.Fatalf("want 6 responses (notifications silent), got %d: %v", len(responses), responses)
	}
	init := responses[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" {
		t.Fatalf("protocol version not echoed: %v", init)
	}
	if info := init["serverInfo"].(map[string]any); info["name"] != hostbridge.MCPServerName {
		t.Fatalf("server name = %v", info["name"])
	}
	if _, ok := responses[1]["result"].(map[string]any); !ok {
		t.Fatalf("ping result missing: %v", responses[1])
	}
	tools := responses[2]["result"].(map[string]any)["tools"].([]any)
	foundClaim := false
	for _, raw := range tools {
		tool := raw.(map[string]any)
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok || schema["type"] != "object" {
			t.Fatalf("tool %v lacks an object inputSchema", tool["name"])
		}
		if tool["name"] == "cortex_ia_work_claim" {
			foundClaim = true
		}
	}
	if !foundClaim {
		t.Fatal("tools/list lacks cortex_ia_work_claim")
	}
	if errObj := responses[3]["error"].(map[string]any); errObj["code"] != float64(errMethodNotFound) {
		t.Fatalf("unknown method error = %v", errObj)
	}
	if resp := responses[4]["result"].(map[string]any); resp["isError"] != true {
		t.Fatalf("unknown tool must be isError: %v", resp)
	}
	malformed := responses[5]
	if malformed["id"] != nil || malformed["error"].(map[string]any)["code"] != float64(errParseError) {
		t.Fatalf("malformed line error = %v", malformed)
	}
}

func TestReadLineAcceptsLinesLongerThanScannerLimit(t *testing.T) {
	long := strings.Repeat("a", 200<<10)
	s := newTestSession(t, cortexFake(t))
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cortex_ia_content_hash","arguments":{"content":"` + long + `"}}}` + "\n"
	responses := runProtocol(t, s, input)
	result := responses[0]["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("long line rejected: %v", result)
	}
}
