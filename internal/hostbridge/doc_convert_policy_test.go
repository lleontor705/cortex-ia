package hostbridge

import (
	"errors"
	"testing"
	"time"
)

func docConvertInput(role string, args map[string]any) HookInput {
	return HookInput{
		Host:       HostClaude,
		Role:       role,
		SessionKey: "cc-s1",
		Tool:       "mcp__cortex_ia__cortex_ia_doc_convert",
		Args:       args,
		Workspace:  "/workspace",
	}
}

// Writing doc_convert output is implement-only and binds the caller for the
// MCP server; inline reads stay open to every role without a binding.
func TestDocConvertOutputIsImplementOnlyAndBinds(t *testing.T) {
	stateHome := t.TempDir()
	deps := Deps{StateHome: stateHome, Now: time.Now}
	withOutput := map[string]any{"task_id": "t1", "file_path": "doc.docx", "output_path": "out.md"}

	if d, err := Decide(docConvertInput("investigate", withOutput), deps); err != nil || d.Verdict != Deny {
		t.Fatalf("investigate writing doc_convert output must be denied, got %+v err=%v", d, err)
	}

	if d, err := Decide(docConvertInput("implement", withOutput), deps); err != nil || d.Verdict != PassThrough {
		t.Fatalf("implement writing doc_convert output must pass, got %+v err=%v", d, err)
	}
	if _, err := ConsumeCaller(stateHome, "t1", "cortex_ia_doc_convert", time.Now()); err != nil {
		t.Fatalf("implement doc_convert with task_id must bind the caller: %v", err)
	}

	inline := map[string]any{"task_id": "t1", "file_path": "doc.docx"}
	if d, err := Decide(docConvertInput("investigate", inline), deps); err != nil || d.Verdict != PassThrough {
		t.Fatalf("inline doc_convert must stay open, got %+v err=%v", d, err)
	}
	if _, err := ConsumeCaller(stateHome, "t1", "cortex_ia_doc_convert", time.Now()); !errors.Is(err, ErrCallerUnbound) {
		t.Fatalf("inline doc_convert must not bind a caller, got err=%v", err)
	}
}
