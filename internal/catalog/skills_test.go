package catalog

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// TestAllSkills_ReturnsNonEmpty verifies AllSkills returns a populated slice.
func TestAllSkills_ReturnsNonEmpty(t *testing.T) {
	skills := AllSkills()
	if len(skills) == 0 {
		t.Fatal("AllSkills() returned empty slice")
	}
}

// TestAllSkills_EveryEntryHasCategoryAndPriority verifies every Skill entry
// has a non-empty Category and a non-zero Priority.
func TestAllSkills_EveryEntryHasCategoryAndPriority(t *testing.T) {
	skills := AllSkills()
	for _, s := range skills {
		if s.Category == "" {
			t.Errorf("skill %q has empty Category", s.ID)
		}
		if s.Priority == 0 {
			t.Errorf("skill %q has zero Priority", s.ID)
		}
		if s.Name == "" {
			t.Errorf("skill %q has empty Name", s.ID)
		}
	}
}

// TestAllSkills_ContainsExistingSkillIDs verifies all skills that were
// previously in AllSDDSkillIDs are present in AllSkills.
func TestAllSkills_ContainsExistingSkillIDs(t *testing.T) {
	existing := []model.SkillID{
		model.SkillSDDInit, model.SkillSDDExplore, model.SkillSDDPropose,
		model.SkillSDDSpec, model.SkillSDDDesign, model.SkillSDDTasks,
		model.SkillSDDApply, model.SkillSDDVerify, model.SkillSDDArchive,
		model.SkillTeamLead, model.SkillDebug, model.SkillIdeate,
		model.SkillDebate, model.SkillMonitor, model.SkillExecutePlan,
		model.SkillOpenPR, model.SkillFileIssue, model.SkillScanRegistry,
	}

	skills := AllSkills()
	idSet := make(map[model.SkillID]bool, len(skills))
	for _, s := range skills {
		idSet[s.ID] = true
	}

	for _, id := range existing {
		if !idSet[id] {
			t.Errorf("existing skill %q missing from AllSkills()", id)
		}
	}
}

// TestAllSkills_ContainsNewSkillIDs verifies the 8 new skill IDs are present.
func TestAllSkills_ContainsNewSkillIDs(t *testing.T) {
	newSkills := []model.SkillID{
		model.SkillWorkUnitCommits,
		model.SkillChainedPR,
		model.SkillCognitiveDoc,
		model.SkillCommentWriter,
		model.SkillGoTesting,
		model.SkillSkillCreator,
		model.SkillSkillImprover,
		model.SkillOnboard,
	}

	skills := AllSkills()
	idSet := make(map[model.SkillID]bool, len(skills))
	for _, s := range skills {
		idSet[s.ID] = true
	}

	for _, id := range newSkills {
		if !idSet[id] {
			t.Errorf("new skill %q missing from AllSkills()", id)
		}
	}
}

// TestAllSkills_ContainsJudgmentDay verifies the judgment-day skill is in the
// comprehensive catalog.
func TestAllSkills_ContainsJudgmentDay(t *testing.T) {
	skills := AllSkills()
	for _, s := range skills {
		if s.ID == model.SkillJudgmentDay {
			return
		}
	}
	t.Errorf("judgment-day missing from AllSkills()")
}

// TestAllSkills_NoDuplicateIDs verifies there are no duplicate skill IDs.
func TestAllSkills_NoDuplicateIDs(t *testing.T) {
	skills := AllSkills()
	seen := make(map[model.SkillID]bool, len(skills))
	for _, s := range skills {
		if seen[s.ID] {
			t.Errorf("duplicate skill ID: %q", s.ID)
		}
		seen[s.ID] = true
	}
}

// TestAllSkills_CategoryValues verifies categories use the expected set of
// values (sdd, utility, meta, review).
func TestAllSkills_CategoryValues(t *testing.T) {
	validCategories := map[string]bool{
		"sdd":     true,
		"utility": true,
		"meta":    true,
		"review":  true,
		"ops":     true,
	}

	skills := AllSkills()
	for _, s := range skills {
		if !validCategories[s.Category] {
			t.Errorf("skill %q has unexpected category %q", s.ID, s.Category)
		}
	}
}

// TestAllSDDSkillIDs_BackwardCompat verifies AllSDDSkillIDs returns
// []model.SkillID extracted from AllSkills, with matching length and IDs.
func TestAllSDDSkillIDs_BackwardCompat(t *testing.T) {
	skills := AllSkills()
	ids := AllSDDSkillIDs()

	if len(ids) != len(skills) {
		t.Fatalf("AllSDDSkillIDs() length = %d, want %d (matching AllSkills)", len(ids), len(skills))
	}

	// Build a set of IDs from AllSkills for membership verification.
	idSet := make(map[model.SkillID]bool, len(skills))
	for _, s := range skills {
		idSet[s.ID] = true
	}

	for _, id := range ids {
		if !idSet[id] {
			t.Errorf("AllSDDSkillIDs() returned ID %q not present in AllSkills()", id)
		}
	}

	// Verify the return type is []model.SkillID (compile-time check via
	// variable assignment).
	var _ []model.SkillID = ids
}

// TestAllSDDSkillIDs_ReturnTypeIsModelSkillID ensures the return type is
// exactly []model.SkillID so existing callers compile without modification.
func TestAllSDDSkillIDs_ReturnTypeIsModelSkillID(t *testing.T) {
	ids := AllSDDSkillIDs()
	if ids == nil {
		t.Fatal("AllSDDSkillIDs() returned nil")
	}
	// The assignment above to []model.SkillID would fail at compile time if
	// the return type changed. This test exists as an explicit guard.
}
