package sdd

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestProfilePhaseOrderUsesCanonicalActiveIDs(t *testing.T) {
	want := []string{"bootstrap", "investigate", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}
	if got := ProfilePhaseOrder(); !slices.Equal(got, want) {
		t.Fatalf("ProfilePhaseOrder() = %v, want %v", got, want)
	}
	for _, legacy := range []string{"init", "explore", "sdd-init", "sdd-explore"} {
		if IsKnownPhase(legacy) {
			t.Errorf("legacy profile phase %q advertised as active", legacy)
		}
	}
}

func TestSelectWorkflowProfile_Goldens(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	directChild := qualifiedProfileFact("delegation/direct-child", now)
	isolation := qualifiedProfileFact("isolation/worktree", now)
	experimentalIsolation := isolation
	experimentalIsolation.Experimental = true
	staleIsolation := isolation
	staleIsolation.FreshUntil = now
	documentedDirectChild := directChild
	documentedDirectChild.EvidenceClass = capability.EvidenceDocumentation
	documentedDirectChild.Enforcement = capability.EnforcementPrompt

	tests := []struct {
		name   string
		input  ProfileSelectionInput
		golden string
	}{
		{
			name:   "sequential requires no delegation evidence",
			input:  ProfileSelectionInput{Now: now},
			golden: `{"profile":"portable-sequential","qualified_capabilities":[],"degradations":["delegation/direct-child: no fresh proven capability fact"]}`,
		},
		{
			name:   "documentation alone cannot prove flat delegation",
			input:  ProfileSelectionInput{Now: now, Facts: []capability.CapabilityFact{documentedDirectChild}},
			golden: `{"profile":"portable-sequential","qualified_capabilities":[],"degradations":["delegation/direct-child: no fresh proven capability fact"]}`,
		},
		{
			name:   "flat needs only proven direct child delegation",
			input:  ProfileSelectionInput{Now: now, Facts: []capability.CapabilityFact{directChild}},
			golden: `{"profile":"portable-flat","qualified_capabilities":["delegation/direct-child"],"degradations":[]}`,
		},
		{
			name: "direct child requirement alone does not claim native features",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{directChild},
				NativeCapabilities: []capability.CapabilityID{"delegation/direct-child"},
			},
			golden: `{"profile":"portable-flat","qualified_capabilities":["delegation/direct-child"],"degradations":[]}`,
		},
		{
			name: "native requires every requested capability to be qualified",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{directChild},
				NativeCapabilities: []capability.CapabilityID{"isolation/worktree"},
			},
			golden: `{"profile":"portable-flat","qualified_capabilities":["delegation/direct-child"],"degradations":["isolation/worktree: no fresh proven capability fact"]}`,
		},
		{
			name: "qualified native selects advanced",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{isolation, directChild},
				NativeCapabilities: []capability.CapabilityID{"isolation/worktree"},
			},
			golden: `{"profile":"native-advanced","qualified_capabilities":["delegation/direct-child","isolation/worktree"],"degradations":[]}`,
		},
		{
			name: "experimental native remains opt in after qualification",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{directChild, experimentalIsolation},
				NativeCapabilities: []capability.CapabilityID{"isolation/worktree"},
			},
			golden: `{"profile":"portable-flat","qualified_capabilities":["delegation/direct-child"],"degradations":["isolation/worktree: experimental capability requires explicit opt-in"]}`,
		},
		{
			name: "experimental native accepts explicit capability opt in",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{experimentalIsolation, directChild},
				NativeCapabilities: []capability.CapabilityID{"isolation/worktree"},
				ExperimentalOptIns: []capability.CapabilityID{"isolation/worktree"},
			},
			golden: `{"profile":"native-advanced","qualified_capabilities":["delegation/direct-child","isolation/worktree"],"degradations":[]}`,
		},
		{
			name: "stale evidence cannot upgrade flat to native",
			input: ProfileSelectionInput{
				Now:                now,
				Facts:              []capability.CapabilityFact{staleIsolation, directChild},
				NativeCapabilities: []capability.CapabilityID{"isolation/worktree"},
			},
			golden: `{"profile":"portable-flat","qualified_capabilities":["delegation/direct-child"],"degradations":["isolation/worktree: no fresh proven capability fact"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := SelectWorkflowProfile(tt.input)
			got, err := json.Marshal(selection)
			if err != nil {
				t.Fatalf("marshal selection: %v", err)
			}
			if string(got) != tt.golden {
				t.Fatalf("selection golden mismatch\n got: %s\nwant: %s", got, tt.golden)
			}
		})
	}
}

func TestSelectWorkflowProfile_IsDeterministicAcrossInputOrder(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	directChild := qualifiedProfileFact("delegation/direct-child", now)
	isolation := qualifiedProfileFact("isolation/worktree", now)
	input := ProfileSelectionInput{
		Now:                now,
		Facts:              []capability.CapabilityFact{isolation, directChild},
		NativeCapabilities: []capability.CapabilityID{"isolation/worktree", "isolation/worktree"},
		ExperimentalOptIns: []capability.CapabilityID{"isolation/worktree", "isolation/worktree"},
	}

	first, err := json.Marshal(SelectWorkflowProfile(input))
	if err != nil {
		t.Fatal(err)
	}
	input.Facts[0], input.Facts[1] = input.Facts[1], input.Facts[0]
	second, err := json.Marshal(SelectWorkflowProfile(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("selection depends on input order: %s != %s", first, second)
	}
}

func TestSelectCompiledWorkflowProfileUsesOnlyNormalizedCompilerSnapshot(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	compiled := compiledInjectionFixture(now, ProfilePortableFlat, []capability.CapabilityFact{
		qualifiedProfileFact(directChildDelegation, now),
	})

	selection, err := SelectCompiledWorkflowProfile(compiled, nil, nil)
	if err != nil {
		t.Fatalf("SelectCompiledWorkflowProfile() error = %v", err)
	}
	if selection.Profile != ProfilePortableFlat || len(selection.Degradations) != 0 {
		t.Fatalf("selection = %+v", selection)
	}

	compiled.Normalized.Profile = string(ProfilePortableSequential)
	if _, err := SelectCompiledWorkflowProfile(compiled, nil, nil); err == nil {
		t.Fatal("SelectCompiledWorkflowProfile() accepted a profile inconsistent with its normalized capability snapshot")
	}
}

func qualifiedProfileFact(id capability.CapabilityID, now time.Time) capability.CapabilityFact {
	return capability.CapabilityFact{
		ID:              id,
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          "test-target",
		RuntimeID:       "test-runtime",
		AdapterID:       "test-adapter",
		RuntimeVersions: ir.VersionRange{Minimum: ir.MustParseVersion("1.0.0"), MaximumTested: ir.MustParseVersion("1.1.0")},
		EvidenceClass:   capability.EvidenceRuntimeObserved,
		EvidenceRef:     "cortex://evidence/profile-selection",
		ObservedAt:      now.Add(-time.Hour),
		FreshUntil:      now.Add(time.Hour),
		Confidence:      1,
		Current:         true,
		Enforcement:     capability.EnforcementRuntime,
	}
}

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
	if len(p.ConfiguredAssignments) != len(ProfilePhaseOrder()) {
		t.Errorf("expected %d phase assignments, got %d", len(ProfilePhaseOrder()), len(p.ConfiguredAssignments))
	}
	if p.ConfiguredAssignments["design"].FormatOpenCodeModel() != "openai/gpt-4o-mini" {
		t.Errorf("design = %#v", p.ConfiguredAssignments["design"])
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
	name, phase, pm, err := ParseProfilePhaseSpec("cheap:design:provider-test/model-test")
	if err != nil {
		t.Fatalf("ParseProfilePhaseSpec: %v", err)
	}
	if name != "cheap" || phase != "design" || pm != "provider-test/model-test" {
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
		{Name: "a", ModelAssignments: map[string]string{"sdd-init": "route/v1/one"}},
	}
	got := UpsertProfile(initial, model.Profile{
		Name: "a", ModelAssignments: map[string]string{"sdd-init": "route/v1/two"},
	})
	if len(got) != 1 {
		t.Errorf("expected 1 profile after upsert, got %d", len(got))
	}
	if got[0].ModelAssignments["sdd-init"] != "route/v1/two" {
		t.Errorf("upsert did not replace value")
	}
}

func TestSetProfilePhase_NewProfile(t *testing.T) {
	got := SetProfilePhase(nil, "fast", "sdd-apply", "provider-test/model-test")
	if len(got) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(got))
	}
	if got[0].ConfiguredAssignments["sdd-apply"].FormatOpenCodeModel() != "provider-test/model-test" {
		t.Errorf("phase value not set: %v", got[0].ConfiguredAssignments)
	}
	if len(got[0].ConfiguredAssignments) != 1 {
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

	_, removed = RemoveProfile(got, "missing")
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
	p := model.Profile{Name: "mix", ConfiguredAssignments: map[string]model.OpenCodeModelAssignment{
		"sdd-init":   {Provider: "openai", Model: "gpt-4o-mini"},
		"sdd-design": {Provider: "provider-test", Model: "model-test"},
	}}
	s := ProfileSummary(p)
	if !contains(s, "phase(s) configured") {
		t.Errorf("expected per-phase summary, got %q", s)
	}
}

func TestProfileToOpenCodeAssignments_FullyQualified(t *testing.T) {
	p, _ := ParseProfileSpec("cheap:openai/gpt-4o-mini")
	got := ProfileToOpenCodeAssignments(p)
	if len(got) != len(ProfilePhaseOrder()) {
		t.Errorf("expected one assignment per phase, got %d", len(got))
	}
	a := got["architect"] // sdd-design -> architect agent
	if a.Provider != "openai" || a.Model != "gpt-4o-mini" {
		t.Errorf("architect assignment = %+v", a)
	}
	if got["implement"].Model != "gpt-4o-mini" {
		t.Errorf("sdd-apply should map directly to implement, got %+v", got["implement"])
	}
	if _, has := got["team-lead"]; has {
		t.Error("portable profile mapping must not emit team-lead")
	}
	if _, has := got["design"]; has {
		t.Error("profile mapping leaked legacy design key instead of architect")
	}
	if _, has := got["apply"]; has {
		t.Error("profile mapping leaked legacy apply key instead of implement")
	}
}

func TestProfileToOpenCodeAssignments_RejectsLegacyAliases(t *testing.T) {
	p := model.Profile{Name: "x", ModelAssignments: map[string]string{
		"sdd-init": "legacy-tier",
	}}
	got := ProfileToOpenCodeAssignments(p)
	if len(got) != 0 {
		t.Errorf("legacy aliases must not emit assignments: %+v", got)
	}
}

func TestProfileToOpenCodeAssignments_DropsUnparseable(t *testing.T) {
	p := model.Profile{Name: "x", ConfiguredAssignments: map[string]model.OpenCodeModelAssignment{
		"sdd-init":  {Provider: "provider-test", Model: "model-test"},
		"sdd-bogus": {Provider: "provider-test"},
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
	p := model.Profile{Name: "direct", ConfiguredAssignments: map[string]model.OpenCodeModelAssignment{
		"architect": {Provider: "openai", Model: "gpt-5.4"},
		"implement": {Provider: "provider-test", Model: "model-test"},
	}}
	got := ProfileToOpenCodeAssignments(p)
	if got["architect"].FormatOpenCodeModel() != "openai/gpt-5.4" {
		t.Errorf("architect = %+v", got["architect"])
	}
	if got["implement"].FormatOpenCodeModel() != "provider-test/model-test" {
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
