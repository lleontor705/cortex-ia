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

func TestComposePrompt_ModelHintsAreNotInferred(t *testing.T) {
	models := model.ModelAssignments{"sdd-design": "untrusted-model"}
	p := ComposePrompt("x", nil, nil, "", models)
	if strings.Contains(p, "untrusted-model") || strings.Contains(p, "Model hints") {
		t.Errorf("unproven model hint must not be emitted: %s", p)
	}
}

func TestComposePromptWithRoute_EmitsConfiguredProvenance(t *testing.T) {
	p, err := ComposePromptWithRoute("x", nil, nil, "", RouteSelection{
		Route: "route/v1/review", Provider: "provider-x", Model: "model-y", Provenance: "user-config", Resolved: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "provider-x/model-y") || !strings.Contains(p, "user-config") {
		t.Fatalf("configured route missing from prompt: %s", p)
	}
}

func TestComposePromptWithRoute_ForgedSelectionFailsClosed(t *testing.T) {
	if _, err := ComposePromptWithRoute("x", nil, nil, "", RouteSelection{Provider: "provider-x", Model: "model-y", Resolved: true}); err == nil {
		t.Fatal("expected forged route selection to fail closed")
	}
}

func TestComposePrompt_StandaloneHasNoMailboxOrMessagingToolAdvertisement(t *testing.T) {
	p := ComposePrompt("x", &SDDIntegration{Mode: SDDStandalone}, nil, "", nil)

	// Positive: supported Cortex + ForgeSpec guidance MUST remain present.
	for _, want := range []string{
		"cortex memory tools",
		"forgespec",
		"mem_save",
		"sdd_validate",
		"tb_create_board",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("supported guidance %q must remain in prompt:\n%s", want, p)
		}
	}

	// Forbidden (REQ-CTX-001, REQ-TOOL-001): retired Mailbox surface and the
	// messaging / A2A / lease / dead-letter tool families. Tokens are assembled
	// from fragments so this test file does not itself carry literal forbidden
	// current-surface vocabulary.
	forbidden := []string{
		strings.Join([]string{"mai", "lbox"}, ""),  // Mailbox
		strings.Join([]string{"ms", "g_send"}, ""), // msg_send
		strings.Join([]string{"ms", "g_broadcast"}, ""),
		strings.Join([]string{"ms", "g_request"}, ""),
		strings.Join([]string{"ms", "g_read_inbox"}, ""),
		strings.Join([]string{"a2", "a_submit_task"}, ""), // a2a_submit_task
		strings.Join([]string{"a2", "a_respond_task"}, ""),
		strings.Join([]string{"resour", "ce_acquire"}, ""), // resource_acquire
		strings.Join([]string{"resour", "ce_release"}, ""),
		strings.Join([]string{"dl", "q_list"}, ""), // dlq_list
		strings.Join([]string{"dl", "q_retry"}, ""),
	}
	lower := strings.ToLower(p)
	for _, tok := range forbidden {
		if strings.Contains(lower, tok) {
			t.Errorf("prompt must not advertise forbidden tool-family token %q:\n%s", tok, p)
		}
	}
}
