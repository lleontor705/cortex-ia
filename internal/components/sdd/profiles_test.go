package sdd

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestValidateProfileName(t *testing.T) {
	for _, ok := range []string{"cheap", "fast-iteration", "v2", "default"} {
		if err := ValidateProfileName(ok); err != nil {
			t.Errorf("expected %q to be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Cheap", "with space", "way-too-long-profile-name-exceeding-forty-chars-x"} {
		if err := ValidateProfileName(bad); err == nil {
			t.Errorf("expected %q to be invalid", bad)
		}
	}
}

func TestParseProfileSpec(t *testing.T) {
	p, err := ParseProfileSpec("cheap:openai/gpt-4o-mini")
	if err != nil {
		t.Fatalf("ParseProfileSpec: %v", err)
	}
	if p.Name != "cheap" {
		t.Errorf("name = %q, want cheap", p.Name)
	}
	if len(p.ModelAssignments) != len(ProfilePhaseOrder()) {
		t.Errorf("expected %d phase assignments, got %d", len(ProfilePhaseOrder()), len(p.ModelAssignments))
	}
	if string(p.ModelAssignments["sdd-design"]) != "openai/gpt-4o-mini" {
		t.Errorf("sdd-design = %q", p.ModelAssignments["sdd-design"])
	}
}

func TestParseProfileSpec_Invalid(t *testing.T) {
	for _, bad := range []string{"no-colon", "name:", ":provider/model", "name:badspec"} {
		if _, err := ParseProfileSpec(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestParseProfilePhaseSpec(t *testing.T) {
	name, phase, pm, err := ParseProfilePhaseSpec("cheap:sdd-design:anthropic/claude-opus-4")
	if err != nil {
		t.Fatalf("ParseProfilePhaseSpec: %v", err)
	}
	if name != "cheap" || phase != "sdd-design" || pm != "anthropic/claude-opus-4" {
		t.Errorf("got (%q, %q, %q)", name, phase, pm)
	}
}

func TestParseProfilePhaseSpec_UnknownPhase(t *testing.T) {
	if _, _, _, err := ParseProfilePhaseSpec("cheap:sdd-bogus:anthropic/x"); err == nil {
		t.Fatal("expected error for unknown phase")
	}
}

func TestUpsertProfile_Inserts(t *testing.T) {
	got := UpsertProfile(nil, model.Profile{Name: "a"})
	if len(got) != 1 {
		t.Errorf("expected 1 profile, got %d", len(got))
	}
}

func TestUpsertProfile_Replaces(t *testing.T) {
	initial := []model.Profile{
		{Name: "a", ModelAssignments: map[string]model.ClaudeModelAlias{"sdd-init": "v1"}},
	}
	got := UpsertProfile(initial, model.Profile{
		Name: "a", ModelAssignments: map[string]model.ClaudeModelAlias{"sdd-init": "v2"},
	})
	if len(got) != 1 {
		t.Errorf("expected 1 profile after upsert, got %d", len(got))
	}
	if got[0].ModelAssignments["sdd-init"] != "v2" {
		t.Errorf("upsert did not replace value")
	}
}

func TestSetProfilePhase_NewProfile(t *testing.T) {
	got := SetProfilePhase(nil, "fast", "sdd-apply", "anthropic/claude-haiku-4-5")
	if len(got) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(got))
	}
	if got[0].ModelAssignments["sdd-apply"] != "anthropic/claude-haiku-4-5" {
		t.Errorf("phase value not set: %v", got[0].ModelAssignments)
	}
	if len(got[0].ModelAssignments) != 1 {
		t.Errorf("expected only one phase set, got %d", len(got[0].ModelAssignments))
	}
}

func TestRemoveProfile(t *testing.T) {
	initial := []model.Profile{{Name: "a"}, {Name: "b"}}
	got, removed := RemoveProfile(initial, "a")
	if !removed {
		t.Error("expected removed=true")
	}
	if len(got) != 1 || got[0].Name != "b" {
		t.Errorf("got %v", got)
	}

	got, removed = RemoveProfile(got, "missing")
	if removed {
		t.Error("expected removed=false for missing profile")
	}
}

func TestProfileSummary_Uniform(t *testing.T) {
	p, _ := ParseProfileSpec("cheap:openai/gpt-4o-mini")
	s := ProfileSummary(p)
	if !contains(s, "all phases") {
		t.Errorf("expected 'all phases' summary, got %q", s)
	}
}

func TestProfileSummary_PerPhase(t *testing.T) {
	p := model.Profile{Name: "mix", ModelAssignments: map[string]model.ClaudeModelAlias{
		"sdd-init":   "openai/gpt-4o-mini",
		"sdd-design": "anthropic/claude-opus-4",
	}}
	s := ProfileSummary(p)
	if !contains(s, "phase(s) configured") {
		t.Errorf("expected per-phase summary, got %q", s)
	}
}

func TestProfileToOpenCodeAssignments_FullyQualified(t *testing.T) {
	p, _ := ParseProfileSpec("cheap:openai/gpt-4o-mini")
	got := ProfileToOpenCodeAssignments(p)
	if len(got) != len(ProfilePhaseOrder())+1 {
		t.Errorf("expected phase assignments plus apply worker split, got %d", len(got))
	}
	a := got["architect"] // sdd-design -> architect agent
	if a.Provider != "openai" || a.Model != "gpt-4o-mini" {
		t.Errorf("architect assignment = %+v", a)
	}
	if got["team-lead"].Model != "gpt-4o-mini" || got["implement"].Model != "gpt-4o-mini" {
		t.Errorf("sdd-apply should map to both team-lead and implement, got team-lead=%+v implement=%+v", got["team-lead"], got["implement"])
	}
	if _, has := got["design"]; has {
		t.Error("profile mapping leaked legacy design key instead of architect")
	}
	if _, has := got["apply"]; has {
		t.Error("profile mapping leaked legacy apply key instead of team-lead/implement")
	}
}

func TestProfileToOpenCodeAssignments_ExpandsClaudeAliases(t *testing.T) {
	p := model.Profile{Name: "x", ModelAssignments: map[string]model.ClaudeModelAlias{
		"sdd-init":   model.ModelOpus,
		"sdd-design": model.ModelSonnet,
		"sdd-apply":  model.ModelHaiku,
	}}
	got := ProfileToOpenCodeAssignments(p)
	if got["bootstrap"].Provider != "anthropic" || got["bootstrap"].Model != "claude-opus-4" {
		t.Errorf("bootstrap = %+v", got["bootstrap"])
	}
	if got["team-lead"].Model != "claude-haiku-4-5" || got["implement"].Model != "claude-haiku-4-5" {
		t.Errorf("apply aliases should map to team-lead and implement, got team-lead=%+v implement=%+v", got["team-lead"], got["implement"])
	}
}

func TestProfileToOpenCodeAssignments_DropsUnparseable(t *testing.T) {
	p := model.Profile{Name: "x", ModelAssignments: map[string]model.ClaudeModelAlias{
		"sdd-init":  "anthropic/claude-opus-4",
		"sdd-bogus": "garbage-no-slash-not-an-alias",
	}}
	got := ProfileToOpenCodeAssignments(p)
	if _, has := got["bootstrap"]; !has {
		t.Error("valid entry was dropped")
	}
	if _, has := got["bogus"]; has {
		t.Error("garbage entry leaked through")
	}
}

func TestProfileToOpenCodeAssignments_AcceptsDirectAgentKeys(t *testing.T) {
	p := model.Profile{Name: "direct", ModelAssignments: map[string]model.ClaudeModelAlias{
		"architect": "openai/gpt-5.4",
		"implement": model.ModelSonnet,
	}}
	got := ProfileToOpenCodeAssignments(p)
	if got["architect"].FormatOpenCodeModel() != "openai/gpt-5.4" {
		t.Errorf("architect = %+v", got["architect"])
	}
	if got["implement"].FormatOpenCodeModel() != "anthropic/claude-sonnet-4-6" {
		t.Errorf("implement = %+v", got["implement"])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (indexOf(s, sub) >= 0)))
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
