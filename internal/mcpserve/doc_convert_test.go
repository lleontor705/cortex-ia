package mcpserve

import (
	"strings"
	"testing"
)

// doc_convert writes a file only with output_path; that form needs task_id and
// the caller's bound implement identity, exactly like diagram_render.
func TestDocConvertOutputRequiresTaskAndLease(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)
	if text, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatalf("claim failed: %s", text)
	}

	text, isError := call(t, s, "cortex_ia_doc_convert", map[string]any{"file_path": "doc.docx", "output_path": "a.txt"})
	if !isError || !strings.Contains(text, "task_id is required") {
		t.Fatalf("output_path without task_id must fail, got isError=%v %s", isError, text)
	}

	bindImplementCall(t, s, "cortex_ia_doc_convert")
	text, isError = call(t, s, "cortex_ia_doc_convert", map[string]any{
		"task_id": testTask, "file_path": "doc.docx", "output_path": "a.txt",
	})
	if isError {
		t.Fatalf("convert under lease failed: %s", text)
	}
	assertNoSecrets(t, text)
	if _, ran := runner.lastWith("doc", "convert"); !ran {
		t.Fatal("doc convert verb never ran")
	}

	bindImplementCall(t, s, "cortex_ia_doc_convert")
	text, isError = call(t, s, "cortex_ia_doc_convert", map[string]any{
		"task_id": testTask, "file_path": "doc.docx", "output_path": "unleased.md",
	})
	if !isError || !strings.Contains(text, "LEASE_REQUIRED") {
		t.Fatalf("convert to an unleased path must fail with LEASE_REQUIRED, got isError=%v %s", isError, text)
	}
}
