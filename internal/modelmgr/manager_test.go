package modelmgr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const highRef = "anthropic/claude-sonnet-4-5#high"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func configPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "opencode.jsonc")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestDesiredShapeMatrix(t *testing.T) {
	valid := []struct{ ref, effort, compact string }{
		{"anthropic/claude-sonnet-4-5", "", "anthropic/claude-sonnet-4-5"},
		{highRef, "", highRef},
		{"anthropic/claude-sonnet-4-5#low", "high", highRef},
	}
	for _, tc := range valid {
		desired, err := ParseDesired("plan", tc.ref, tc.effort)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.ref, err)
		}
		if got := desired.Compact(); got != tc.compact {
			t.Fatalf("%s: compact = %q, want %q", tc.ref, got, tc.compact)
		}
	}
	for _, ref := range []string{"", "anthropic/", "/claude", "claude sonnet", "anthropic/claude#", "anthropic/cla ude", "anthropic/cla#de#x"} {
		_, err := ParseDesired("plan", ref, "")
		var invalid *InvalidDesiredError
		if !errors.As(err, &invalid) {
			t.Fatalf("%q: want InvalidDesiredError, got %v", ref, err)
		}
	}
}

func TestInvalidDesiredFailsBeforeFileAccess(t *testing.T) {
	home := t.TempDir()
	manager := New(home)
	if _, err := manager.Set(Desired{Agent: "plan", Provider: "anthropic"}, nil); err == nil {
		t.Fatal("expected shape validation error")
	}
	if _, err := os.Stat(configPath(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid set must not create config, stat err = %v", err)
	}
}

func TestDecodeModelRefRoundTrip(t *testing.T) {
	expanded := map[string]any{"providerID": "anthropic", "model": "claude-sonnet-4-5", "variant": "high"}
	for _, value := range []any{highRef, expanded} {
		desired, err := DecodeModelRef("plan", value)
		if err != nil {
			t.Fatalf("%v: %v", value, err)
		}
		if desired.Compact() != highRef {
			t.Fatalf("%v: compact = %q, want %q", value, desired.Compact(), highRef)
		}
	}
	for name, value := range map[string]any{
		"non-string":    42,
		"unknown":       map[string]any{"providerID": "a", "model": "b", "temperature": 1},
		"empty-variant": map[string]any{"providerID": "a", "model": "b", "variant": ""},
	} {
		if _, err := DecodeModelRef("plan", value); err == nil {
			t.Fatalf("%s: expected malformed config error", name)
		}
	}
}

func TestDigestStableAcrossRuns(t *testing.T) {
	desired, err := ParseDesired("plan", highRef, "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := desired.Digest()
	if err != nil {
		t.Fatal(err)
	}
	second, err := desired.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.HasPrefix(first, "amdv1:") {
		t.Fatalf("unstable digest %q vs %q", first, second)
	}
	plain, _ := ParseDesired("plan", "anthropic/claude-sonnet-4-5", "")
	plainDigest, _ := plain.Digest()
	if plainDigest == first {
		t.Fatal("variant must change the digest")
	}
}

func TestSetPreservesCommentsAndMembers(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, "{\n  // keep me\n  \"agents\": {\n    \"Backend\": { \"description\": \"api\" }\n  }\n}\n")
	writeFile(t, filepath.Join(home, ".config", "opencode", "agents", "reviewer.md"),
		"---\nmodel: openai/gpt-5.2\n---\nbody\n")

	manager := New(home)
	if entry, err := manager.Get("backend", nil); err != nil || entry.Agent != "Backend" {
		t.Fatalf("case-folded resolution failed: %+v, %v", entry, err)
	}

	desired, _ := ParseDesired("PLAN", highRef, "")
	result, err := manager.Set(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Agent != "plan" || result.Value != highRef {
		t.Fatalf("unexpected set result %+v", result)
	}

	content := readFile(t, path)
	for _, want := range []string{"// keep me", `"description": "api"`, `"model"`, highRef} {
		if !strings.Contains(content, want) {
			t.Fatalf("config lost %q: %s", want, content)
		}
	}
	for _, banned := range []string{"temperature", "top_p", "maxSteps", "permission", "request"} {
		if strings.Contains(content, banned) {
			t.Fatalf("deprecated field %q emitted: %s", banned, content)
		}
	}

	reviewer, err := manager.Get("REVIEWER", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reviewer.Source != SourceMarkdown || reviewer.MarkdownPin != "openai/gpt-5.2" {
		t.Fatalf("unexpected markdown projection %+v", reviewer)
	}
	pinned, _ := ParseDesired("reviewer", "anthropic/claude-sonnet-4-5", "")
	pinnedResult, err := manager.Set(pinned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pinnedResult.Warning, "openai/gpt-5.2") || pinnedResult.Action != "set" {
		t.Fatalf("markdown pin not disclosed: %+v", pinnedResult)
	}
}

func TestSetDisclosesPreviousValue(t *testing.T) {
	home := t.TempDir()
	manager := New(home)
	first, _ := ParseDesired("plan", "openai/gpt-5.2", "")
	if _, err := manager.Set(first, nil); err != nil {
		t.Fatal(err)
	}
	second, _ := ParseDesired("plan", highRef, "")
	result, err := manager.Set(second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Previous != "openai/gpt-5.2" || result.Value != highRef {
		t.Fatalf("previous value not disclosed: %+v", result)
	}
}

func TestUnknownAgentConflictListsSources(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home), `{"agents": {"Backend": {}}}`)
	manager := New(home)
	_, err := manager.Get("ghost", nil)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Kind != ConflictUnknownAgent {
		t.Fatalf("want unknown agent conflict, got %v", err)
	}
	joined := strings.Join(conflict.Sources, " | ")
	for _, want := range []string{"builtins [build plan general explore]", "config agents [Backend]", "markdown agents []"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("sources missing %q: %v", want, conflict.Sources)
		}
	}
}

func TestConfigSpellingWinsMarkdownCaseCollision(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	writeFile(t, path, `{"agents": {"Backend": {"description": "api"}}}`)
	writeFile(t, filepath.Join(home, ".config", "opencode", "agents", "backend.md"),
		"---\nmodel: openai/gpt-5.2\n---\nbody\n")

	manager := New(home)
	entry, err := manager.Get("BACKEND", nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Agent != "Backend" {
		t.Fatalf("config spelling lost to markdown filename: %+v", entry)
	}

	desired, _ := ParseDesired("backend", highRef, "")
	result, err := manager.Set(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Agent != "Backend" || result.Value != highRef {
		t.Fatalf("set resolved to a foreign spelling: %+v", result)
	}
	content := readFile(t, path)
	if strings.Contains(content, `"backend"`) {
		t.Fatalf("set introduced a case-ambiguous member: %s", content)
	}
	if _, err := manager.Get("Backend", nil); err != nil {
		t.Fatalf("follow-up operation failed closed: %v", err)
	}
}

func TestUnsetPrunesCreatedEntryAndKeepsOthers(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	manager := New(home)
	desired, _ := ParseDesired("build", "openai/gpt-5.2", "")
	if _, err := manager.Set(desired, nil); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Unset("BUILD", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Previous != "openai/gpt-5.2" || result.Action != "unset" {
		t.Fatalf("unexpected unset result %+v", result)
	}
	if content := readFile(t, path); strings.Contains(content, `"build"`) {
		t.Fatalf("emptied entry not pruned: %s", content)
	}

	writeFile(t, path, `{"agents": {"plan": {"description": "kept", "model": "openai/gpt-5.2"}}}`)
	if _, err := manager.Unset("plan", nil); err != nil {
		t.Fatal(err)
	}
	content := readFile(t, path)
	if !strings.Contains(content, `"description": "kept"`) {
		t.Fatalf("unrelated member lost: %s", content)
	}
	if strings.Contains(content, `"model"`) {
		t.Fatalf("model member not removed: %s", content)
	}
}
