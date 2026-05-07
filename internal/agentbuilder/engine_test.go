package agentbuilder

import (
	"context"
	"errors"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestNewEngine(t *testing.T) {
	cases := []struct {
		id     model.AgentID
		expect bool
	}{
		{model.AgentClaudeCode, true},
		{model.AgentOpenCode, true},
		{model.AgentGeminiCLI, true},
		{model.AgentCodex, true},
		{model.AgentCursor, false},
		{"made-up", false},
	}
	for _, tc := range cases {
		got := NewEngine(tc.id)
		if (got != nil) != tc.expect {
			t.Errorf("NewEngine(%q) ok=%v, want %v", tc.id, got != nil, tc.expect)
		}
		if got != nil && got.Agent() != tc.id {
			t.Errorf("NewEngine(%q).Agent() = %v", tc.id, got.Agent())
		}
	}
}

func TestMockEngine(t *testing.T) {
	e := &MockEngine{
		AgentIDVal:  model.AgentClaudeCode,
		Output:      "mock output",
		IsAvailable: true,
	}
	if e.Agent() != model.AgentClaudeCode {
		t.Errorf("Agent() = %v", e.Agent())
	}
	if !e.Available() {
		t.Errorf("Available() = false")
	}
	out, err := e.Generate(context.Background(), "any")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "mock output" {
		t.Errorf("output = %q", out)
	}
}

func TestMockEngine_Error(t *testing.T) {
	e := &MockEngine{Err: errors.New("boom")}
	_, err := e.Generate(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
