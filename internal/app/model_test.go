package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
)

func TestModelParseListAndDoctor(t *testing.T) {
	asJSON, err := parseModelList([]string{"--json"})
	if err != nil || !asJSON {
		t.Fatalf("parseModelList(--json) = (%v, %v), want (true, nil)", asJSON, err)
	}
	if _, err := parseModelList([]string{"plan"}); err == nil {
		t.Fatal("parseModelList(plan) must reject positional arguments")
	}
	doctorJSON, err := parseModelDoctor([]string{"--json"})
	if err != nil || !doctorJSON {
		t.Fatalf("parseModelDoctor(--json) = (%v, %v), want (true, nil)", doctorJSON, err)
	}
	if _, err := parseModelDoctor([]string{"--dry-run"}); err == nil {
		t.Fatal("parseModelDoctor(--dry-run) must reject flags outside the grammar")
	}
}

func TestModelParseGet(t *testing.T) {
	agent, asJSON, err := parseModelGet([]string{"plan", "--json"})
	if err != nil || agent != "plan" || !asJSON {
		t.Fatalf("parseModelGet(plan --json) = (%q, %v, %v)", agent, asJSON, err)
	}
	for _, args := range [][]string{nil, {"a", "b"}, {"plan", "--model-x"}} {
		if _, _, err := parseModelGet(args); err == nil {
			t.Fatalf("parseModelGet(%v) must fail", args)
		}
	}
}

func TestModelParseSet(t *testing.T) {
	spec, err := parseModelSet([]string{"plan", "anthropic/claude-sonnet-4-5", "--effort", "high", "--json", "--dry-run"})
	if err != nil {
		t.Fatalf("parseModelSet: %v", err)
	}
	if spec.agent != "plan" || spec.ref != "anthropic/claude-sonnet-4-5" || spec.effort != "high" || !spec.asJSON || !spec.dryRun {
		t.Fatalf("parseModelSet produced unexpected spec: %+v", spec)
	}
	desired, err := spec.desired()
	if err != nil {
		t.Fatalf("desired: %v", err)
	}
	if got := desired.Compact(); got != "anthropic/claude-sonnet-4-5#high" {
		t.Fatalf("compact reference = %q, want anthropic/claude-sonnet-4-5#high", got)
	}

	override, err := parseModelSet([]string{"plan", "openai/gpt-5.2#low", "--effort", "high"})
	if err != nil {
		t.Fatalf("parseModelSet override: %v", err)
	}
	overridden, err := override.desired()
	if err != nil {
		t.Fatalf("desired override: %v", err)
	}
	if got := overridden.Compact(); got != "openai/gpt-5.2#high" {
		t.Fatalf("--effort override = %q, want openai/gpt-5.2#high", got)
	}

	for _, args := range [][]string{
		{"plan"},
		{"plan", "a/b", "extra"},
		{"plan", "a/b", "--effort"},
		{"plan", "a/b", "--effort", "--json"},
		{"plan", "a/b", "--temperature", "0.2"},
	} {
		if _, err := parseModelSet(args); err == nil {
			t.Fatalf("parseModelSet(%v) must fail", args)
		}
	}
}

func TestModelParseUnset(t *testing.T) {
	spec, err := parseModelUnset([]string{"plan", "--dry-run", "--json"})
	if err != nil || spec.agent != "plan" || !spec.dryRun || !spec.asJSON {
		t.Fatalf("parseModelUnset = (%+v, %v)", spec, err)
	}
	for _, args := range [][]string{nil, {"a", "b"}, {"plan", "--model-x"}} {
		if _, err := parseModelUnset(args); err == nil {
			t.Fatalf("parseModelUnset(%v) must fail", args)
		}
	}
}

func TestModelDesiredRejectsMalformedReference(t *testing.T) {
	for _, ref := range []string{"anthropic/", "claude sonnet", "anthropic/claude#"} {
		_, err := (modelSetSpec{agent: "plan", ref: ref}).desired()
		var invalid *modelmgr.InvalidDesiredError
		if !errors.As(err, &invalid) {
			t.Fatalf("ref %q: want typed *InvalidDesiredError, got %v", ref, err)
		}
	}
}

func TestModelPreflightRegression(t *testing.T) {
	documented := [][]string{
		{"model"},
		{"model", "list", "--json"},
		{"model", "get", "plan", "--json"},
		{"model", "set", "plan", "anthropic/claude-sonnet-4-5", "--effort", "high", "--json", "--dry-run"},
		{"model", "unset", "plan", "--dry-run"},
		{"model", "doctor", "--json"},
		{"model", "catalog", "--json", "--provider", "openrouter"},
	}
	for _, args := range documented {
		if err := preflightCLI(args); err != nil {
			t.Fatalf("documented grammar %v rejected by preflight: %v", args, err)
		}
	}

	retired := [][]string{
		{"model", "set", "plan", "x", "--model-y"},
		{"model", "list", "--models"},
		{"model", "--model"},
		{"model", "catalog", "--model-y"},
	}
	for _, args := range retired {
		var retiredErr RetiredSurfaceError
		if err := preflightCLI(args); !errors.As(err, &retiredErr) {
			t.Fatalf("args %v: want RetiredSurfaceError, got %v", args, err)
		}
	}

	if strings.Contains(modelUsage, "--model") {
		t.Fatal("model usage names a flag with the retired --model prefix")
	}
}

func TestModelReceiptJSONDisclosesFacts(t *testing.T) {
	receipt := &install.ModelReceipt{
		Agent:      "plan",
		Action:     "set",
		ConfigPath: "/home/u/.config/opencode/opencode.jsonc",
		Previous:   "openai/gpt-5.2",
		Value:      "anthropic/claude-sonnet-4-5#high",
		Changed:    true,
		Managed:    true,
		BackupID:   "bk_123",
		Warnings:   []string{"markdown agent reviewer.md frontmatter also pins openai/gpt-5.2"},
	}
	encoded, err := modelJSONBytes(receipt)
	if err != nil {
		t.Fatalf("modelJSONBytes: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("receipt JSON is not decodable: %v", err)
	}
	for _, key := range []string{"action", "previous", "config_path", "backup_id", "warnings"} {
		if _, present := document[key]; !present {
			t.Fatalf("receipt JSON is missing %q: %s", key, encoded)
		}
	}
	if document["previous"] != "openai/gpt-5.2" {
		t.Fatalf("receipt JSON previous = %v, want openai/gpt-5.2", document["previous"])
	}
}

func TestModelDoctorTemplateGuardHolds(t *testing.T) {
	report := modelmgr.New(t.TempDir()).Doctor(modelmgr.DoctorOptions{
		Template:   modelDoctorTemplate,
		RunCommand: func(string, ...string) ([]byte, error) { return nil, errors.New("opencode2 unavailable") },
	})
	var guard *modelmgr.Finding
	for i := range report.Findings {
		if report.Findings[i].Check == modelmgr.CheckTemplateGuard {
			guard = &report.Findings[i]
		}
	}
	if guard == nil {
		t.Fatal("doctor report is missing the template guard finding")
	}
	if guard.Severity != modelmgr.SeverityOK {
		t.Fatalf("embedded opencode.jsonc declares installer-unsafe keys: %s", guard.Message)
	}
}

func TestModelConflictFailsClosed(t *testing.T) {
	conflict := &modelmgr.ConflictError{Agent: "ghost", Kind: modelmgr.ConflictUnknownAgent, Sources: []string{"builtins"}}
	err := wrapModelConflict(`model set "ghost"`, conflict)
	if err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("typed conflict must render as fail-closed: %v", err)
	}
	if !errors.As(err, &conflict) {
		t.Fatalf("wrapped error must preserve the typed conflict: %v", err)
	}
	plain := errors.New("run install first")
	if got := wrapModelConflict("model set", plain); !errors.Is(got, plain) {
		t.Fatalf("non-conflict errors must pass through unchanged: %v", got)
	}
}

func TestModelUnknownSubcommandNamesValidActions(t *testing.T) {
	err := runModel([]string{"frobnicate"})
	if err == nil {
		t.Fatal("runModel(frobnicate) must fail")
	}
	for _, action := range []string{"list", "get", "set", "unset", "doctor", "catalog"} {
		if !strings.Contains(err.Error(), action) {
			t.Fatalf("error %q does not name the valid action %q", err, action)
		}
	}
	if err := runModel(nil); err == nil {
		t.Fatal("runModel with no subcommand must return usage")
	}
}

func TestModelCatalogReceipts(t *testing.T) {
	catalog := modelmgr.Catalog{Truncated: true, Source: modelmgr.CatalogSourceDaemon, Entries: []modelmgr.CatalogEntry{
		{Provider: "anthropic", Model: "claude-sonnet-4-5", Variants: []string{"high", "low"}},
		{Provider: "openrouter", Model: "anthropic/claude-sonnet-4.5", Variants: []string{"xhigh"}},
	}}
	if spec, err := parseModelCatalog([]string{"--json", "--provider", "openrouter"}); err != nil || !spec.asJSON || spec.provider != "openrouter" {
		t.Fatalf("parseModelCatalog(--json --provider openrouter) = (%+v, %v)", spec, err)
	}
	for _, args := range [][]string{{"plan"}, {"--provider"}, {"--effort", "high"}, {"--json", "extra"}} {
		if _, err := parseModelCatalog(args); err == nil || !strings.Contains(err.Error(), "usage: cortex-ia model catalog") {
			t.Fatalf("parseModelCatalog(%v) = %v, want usage failure", args, err)
		}
	}
	text, textErr := captureStdout(func() error { return renderModelCatalog(catalog, modelCatalogSpec{}) })
	asJSON, jsonErr := captureStdout(func() error { return renderModelCatalog(catalog, modelCatalogSpec{asJSON: true}) })
	if textErr != nil || jsonErr != nil {
		t.Fatalf("capture catalog receipts: %v %v", textErr, jsonErr)
	}
	requireContains(t, text, "anthropic", "openrouter", "variants: high, low", "variants: xhigh", "source: daemon-api", "Truncated")
	requireContains(t, asJSON, `"entries"`, `"truncated"`, `"source"`)
	filtered := func(provider string) []modelmgr.CatalogEntry { return filterModelCatalog(catalog, provider).Entries }
	if got := filtered("openrouter"); len(got) != 1 || got[0].Model != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("--provider openrouter filter = %+v", got)
	}
	if got := filtered("OpenRouter"); len(got) != 0 {
		t.Fatalf("--provider must match case-sensitively, got %+v", got)
	}
	for _, forbidden := range []string{"settings", "headers", "body", "secret"} {
		if strings.Contains(strings.ToLower(asJSON), forbidden) {
			t.Fatalf("catalog JSON leaks %q: %s", forbidden, asJSON)
		}
	}
}
