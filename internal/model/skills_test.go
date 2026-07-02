package model

import "testing"

// TestNewSkillIDConstants verifies the 8 new SkillID constants added in
// the port-gentle-ai-patterns change have the expected string values.
func TestNewSkillIDConstants(t *testing.T) {
	tests := []struct {
		name     string
		id       SkillID
		expected string
	}{
		{"SkillWorkUnitCommits", SkillWorkUnitCommits, "work-unit-commits"},
		{"SkillChainedPR", SkillChainedPR, "chained-pr"},
		{"SkillCognitiveDoc", SkillCognitiveDoc, "cognitive-doc-design"},
		{"SkillCommentWriter", SkillCommentWriter, "comment-writer"},
		{"SkillGoTesting", SkillGoTesting, "go-testing"},
		{"SkillSkillCreator", SkillSkillCreator, "skill-creator"},
		{"SkillSkillImprover", SkillSkillImprover, "skill-improver"},
		{"SkillOnboard", SkillOnboard, "onboard"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.id) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.id, tc.expected)
			}
		})
	}
}

// TestSkillIDConstants_NoDuplicates verifies no two SkillID constants share
// the same string value.
func TestSkillIDConstants_NoDuplicates(t *testing.T) {
	all := []SkillID{
		SkillSDDInit, SkillSDDExplore, SkillSDDPropose,
		SkillSDDSpec, SkillSDDDesign, SkillSDDTasks,
		SkillSDDApply, SkillSDDVerify, SkillSDDArchive,
		SkillTeamLead, SkillDebug, SkillIdeate,
		SkillDebate, SkillMonitor, SkillExecutePlan,
		SkillOpenPR, SkillFileIssue, SkillScanRegistry,
		SkillJudgmentDay,
		// New constants:
		SkillWorkUnitCommits, SkillChainedPR, SkillCognitiveDoc,
		SkillCommentWriter, SkillGoTesting, SkillSkillCreator,
		SkillSkillImprover, SkillOnboard,
	}

	seen := make(map[string]bool, len(all))
	for _, id := range all {
		val := string(id)
		if val == "" {
			t.Error("found empty SkillID constant")
		}
		if seen[val] {
			t.Errorf("duplicate SkillID value: %q", val)
		}
		seen[val] = true
	}
}
