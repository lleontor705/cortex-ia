package agentbuilder

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestGenerate_FullSDD(t *testing.T) {
	spec := AgentSpec{
		Engine:  model.AgentClaudeCode,
		Purpose: "Code review assistant",
		SDDMode: SDDFull,
	}

	agent, err := Generate(spec)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if agent.SkillName != "code-review-assistant" {
		t.Errorf("SkillName = %q, want %q", agent.SkillName, "code-review-assistant")
	}

	if !strings.Contains(agent.SkillContent, "SDD Integration (Full)") {
		t.Error("expected SkillContent to contain full SDD integration section")
	}

	if !strings.Contains(agent.SkillContent, "ALL SDD phases") {
		t.Error("expected SkillContent to mention all SDD phases")
	}

	if !strings.Contains(agent.SkillContent, "claude-code") {
		t.Error("expected SkillContent to reference the engine")
	}
}

func TestGenerate_PhaseSDD(t *testing.T) {
	spec := AgentSpec{
		Engine:   model.AgentGeminiCLI,
		Purpose:  "Security scanner",
		SDDMode:  SDDPhase,
		SDDPhase: "verify",
	}

	agent, err := Generate(spec)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if agent.SkillName != "security-scanner" {
		t.Errorf("SkillName = %q, want %q", agent.SkillName, "security-scanner")
	}

	if !strings.Contains(agent.SkillContent, "Phase: verify") {
		t.Error("expected SkillContent to contain the target phase")
	}

	if !strings.Contains(agent.SkillContent, "specializes in the `verify` SDD phase") {
		t.Error("expected SkillContent to describe phase specialization")
	}
}

func TestGenerate_NoSDD(t *testing.T) {
	spec := AgentSpec{
		Engine:  model.AgentCodex,
		Purpose: "Documentation writer",
		SDDMode: SDDNone,
	}

	agent, err := Generate(spec)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if !strings.Contains(agent.SkillContent, "Standalone Agent") {
		t.Error("expected SkillContent to contain standalone section")
	}

	if strings.Contains(agent.SkillContent, "SDD Integration") {
		t.Error("expected SkillContent to NOT contain SDD integration section")
	}
}

func TestGenerate_KebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix Auth Bugs", "fix-auth-bugs"},
		{"  hello   world  ", "hello-world"},
		{"API Gateway Service", "api-gateway-service"},
		{"simple", "simple"},
		{"with--dashes", "with-dashes"},
		{"Special @#$ Characters!", "special-characters"},
	}

	for _, tt := range tests {
		got := toKebabCase(tt.input)
		if got != tt.want {
			t.Errorf("toKebabCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerate_EmptyPurpose(t *testing.T) {
	spec := AgentSpec{
		Engine:  model.AgentClaudeCode,
		Purpose: "",
		SDDMode: SDDNone,
	}

	_, err := Generate(spec)
	if err == nil {
		t.Fatal("expected error for empty purpose")
	}

	if !strings.Contains(err.Error(), "purpose must not be empty") {
		t.Errorf("error = %q, expected it to mention empty purpose", err.Error())
	}
}

func TestGenerate_WhitespacePurpose(t *testing.T) {
	spec := AgentSpec{
		Engine:  model.AgentClaudeCode,
		Purpose: "   ",
		SDDMode: SDDNone,
	}

	_, err := Generate(spec)
	if err == nil {
		t.Fatal("expected error for whitespace-only purpose")
	}
}

func TestGenerate_FrontmatterPresent(t *testing.T) {
	spec := AgentSpec{
		Engine:  model.AgentClaudeCode,
		Purpose: "Test agent",
		SDDMode: SDDNone,
	}

	agent, err := Generate(spec)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if !strings.HasPrefix(agent.SkillContent, "---\n") {
		t.Error("expected SkillContent to start with frontmatter")
	}

	if !strings.Contains(agent.SkillContent, "name: test-agent") {
		t.Error("expected frontmatter to contain skill name")
	}

	if !strings.Contains(agent.SkillContent, "type: agent") {
		t.Error("expected frontmatter to contain type: agent")
	}
}
