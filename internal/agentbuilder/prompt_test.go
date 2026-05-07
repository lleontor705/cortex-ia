package agentbuilder

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestComposePrompt_BaseSchema(t *testing.T) {
	p := ComposePrompt("review go code", nil, nil, "", nil)
	for _, want := range []string{"# <Title>", "## Description", "## Trigger", "## Instructions"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing %q in prompt", want)
		}
	}
}

func TestComposePrompt_PersonaInjected(t *testing.T) {
	p := ComposePrompt("x", nil, nil, model.PersonaMentor, nil)
	if !strings.Contains(p, "Mentor") {
		t.Errorf("persona not injected: %s", p)
	}
}

func TestComposePrompt_SDDStandalone(t *testing.T) {
	p := ComposePrompt("x", &SDDIntegration{Mode: SDDStandalone}, nil, "", nil)
	if !strings.Contains(p, "Standalone") {
		t.Errorf("SDD standalone not injected: %s", p)
	}
}

func TestComposePrompt_SDDPhaseSupport(t *testing.T) {
	p := ComposePrompt("x", &SDDIntegration{Mode: SDDPhaseSupport, Phase: "sdd-design"}, nil, "", nil)
	if !strings.Contains(p, "Phase-support") {
		t.Errorf("SDD phase-support not injected")
	}
	if !strings.Contains(p, "sdd-design") {
		t.Errorf("phase name not injected")
	}
}

func TestComposePrompt_NoSDD(t *testing.T) {
	p := ComposePrompt("x", &SDDIntegration{Mode: SDDNone}, nil, "", nil)
	if strings.Contains(p, "## SDD integration") {
		t.Errorf("SDD section should be omitted when mode=none")
	}
}

func TestComposePrompt_TargetsListed(t *testing.T) {
	p := ComposePrompt("x", nil, []model.AgentID{model.AgentClaudeCode, model.AgentOpenCode}, "", nil)
	if !strings.Contains(p, "claude-code") || !strings.Contains(p, "opencode") {
		t.Errorf("targets not listed: %s", p)
	}
}

func TestComposePrompt_ReferencesCortexNotEngram(t *testing.T) {
	p := ComposePrompt("x", nil, nil, "", nil)
	if !strings.Contains(p, "cortex MCP") {
		t.Error("prompt should reference cortex MCP")
	}
	if strings.Contains(strings.ToLower(p), "engram") {
		t.Errorf("prompt should never mention Engram (cortex-ia identity): %s", p)
	}
}

func TestComposePrompt_ModelHints(t *testing.T) {
	models := model.ModelAssignments{"sdd-design": "anthropic/claude-opus-4"}
	p := ComposePrompt("x", nil, nil, "", models)
	if !strings.Contains(p, "anthropic/claude-opus-4") {
		t.Errorf("model hint not injected: %s", p)
	}
}
