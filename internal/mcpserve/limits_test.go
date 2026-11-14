package mcpserve

import (
	"strings"
	"testing"
)

func TestOversizedResultIsTruncated(t *testing.T) {
	big := strings.Repeat("é", 40<<10)
	runner := &fakeRunner{respond: func([]string, string) (string, error) { return big, nil }}
	s := newTestSession(t, runner)
	text, isError := call(t, s, "cortex_ia_board_list", map[string]any{})
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if len(text) > maxResultBytes+200 || !strings.Contains(text, "[truncated:") {
		t.Fatalf("result not truncated: len=%d", len(text))
	}
	if !strings.HasPrefix(text, "éé") {
		t.Fatal("truncation corrupted leading UTF-8")
	}
}

func TestParseServeArgs(t *testing.T) {
	for _, args := range [][]string{{}, {"--host"}, {"--host", "bogus"}, {"--host", "claude", "--host", "agy"}, {"--port", "1"}} {
		if _, err := ParseServeArgs(args); err == nil {
			t.Fatalf("ParseServeArgs(%v) accepted invalid input", args)
		}
	}
	host, err := ParseServeArgs([]string{"--host=agy"})
	if err != nil || string(host) != "agy" {
		t.Fatalf("ParseServeArgs(--host=agy) = %q, %v", host, err)
	}
}
