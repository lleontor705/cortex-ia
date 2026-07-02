package catalog

import "github.com/lleontor705/cortex-ia/internal/model"

// Skill describes a single skill managed by the cortex-ia ecosystem.
type Skill struct {
	ID       model.SkillID
	Name     string
	Category string // "sdd", "utility", "meta", "review", "ops"
	Priority int    // lower = higher priority
}

// allSkills is the single source of truth for the full skill catalog.
// Every entry MUST have a non-empty Name, Category, and Priority.
var allSkills = []Skill{
	// --- SDD phase skills ---
	{ID: model.SkillSDDInit, Name: "sdd-init", Category: "sdd", Priority: 1},
	{ID: model.SkillSDDExplore, Name: "sdd-explore", Category: "sdd", Priority: 2},
	{ID: model.SkillSDDPropose, Name: "sdd-propose", Category: "sdd", Priority: 3},
	{ID: model.SkillSDDSpec, Name: "sdd-spec", Category: "sdd", Priority: 3},
	{ID: model.SkillSDDDesign, Name: "sdd-design", Category: "sdd", Priority: 3},
	{ID: model.SkillSDDTasks, Name: "sdd-tasks", Category: "sdd", Priority: 4},
	{ID: model.SkillSDDApply, Name: "sdd-apply", Category: "sdd", Priority: 4},
	{ID: model.SkillSDDVerify, Name: "sdd-verify", Category: "sdd", Priority: 5},
	{ID: model.SkillSDDArchive, Name: "sdd-archive", Category: "sdd", Priority: 5},
	{ID: model.SkillTeamLead, Name: "team-lead", Category: "sdd", Priority: 4},
	{ID: model.SkillOnboard, Name: "onboard", Category: "sdd", Priority: 5},

	// --- Review skill ---
	{ID: model.SkillJudgmentDay, Name: "judgment-day", Category: "review", Priority: 3},

	// --- Utility skills ---
	{ID: model.SkillDebug, Name: "debug", Category: "utility", Priority: 2},
	{ID: model.SkillMonitor, Name: "monitor", Category: "utility", Priority: 3},
	{ID: model.SkillIdeate, Name: "ideate", Category: "utility", Priority: 3},
	{ID: model.SkillExecutePlan, Name: "execute-plan", Category: "utility", Priority: 3},
	{ID: model.SkillOpenPR, Name: "open-pr", Category: "utility", Priority: 3},
	{ID: model.SkillFileIssue, Name: "file-issue", Category: "utility", Priority: 3},
	{ID: model.SkillScanRegistry, Name: "scan-registry", Category: "utility", Priority: 4},
	{ID: model.SkillDebate, Name: "debate", Category: "utility", Priority: 4},
	{ID: model.SkillWorkUnitCommits, Name: "work-unit-commits", Category: "utility", Priority: 3},
	{ID: model.SkillChainedPR, Name: "chained-pr", Category: "utility", Priority: 3},
	{ID: model.SkillCommentWriter, Name: "comment-writer", Category: "utility", Priority: 3},
	{ID: model.SkillGoTesting, Name: "go-testing", Category: "utility", Priority: 3},
	{ID: model.SkillCognitiveDoc, Name: "cognitive-doc-design", Category: "utility", Priority: 4},

	// --- Meta skills ---
	{ID: model.SkillSkillCreator, Name: "skill-creator", Category: "meta", Priority: 4},
	{ID: model.SkillSkillImprover, Name: "skill-improver", Category: "meta", Priority: 4},
}

// AllSkills returns the full skill catalog as []Skill, each entry carrying
// ID, Name, Category, and Priority for grouping and ordering.
func AllSkills() []Skill {
	// Return a defensive copy so callers cannot mutate the package-level slice.
	out := make([]Skill, len(allSkills))
	copy(out, allSkills)
	return out
}

// AllSDDSkillIDs returns the skill IDs managed by the cortex-ia ecosystem.
// This is a backward-compatibility wrapper that extracts []model.SkillID
// from the richer []Skill slice returned by AllSkills(). Existing callers
// (e.g. agentbuilder/registry.go) continue to work without modification.
func AllSDDSkillIDs() []model.SkillID {
	skills := AllSkills()
	ids := make([]model.SkillID, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	return ids
}
