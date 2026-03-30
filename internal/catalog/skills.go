package catalog

import "github.com/lleontor705/cortex-ia/internal/model"

// AllSDDSkillIDs returns the skill IDs managed by the SDD component.
func AllSDDSkillIDs() []model.SkillID {
	return []model.SkillID{
		model.SkillSDDInit, model.SkillSDDExplore, model.SkillSDDPropose,
		model.SkillSDDSpec, model.SkillSDDDesign, model.SkillSDDTasks,
		model.SkillSDDApply, model.SkillSDDVerify, model.SkillSDDArchive,
		model.SkillTeamLead, model.SkillDebug, model.SkillIdeate,
		model.SkillDebate, model.SkillMonitor, model.SkillExecutePlan,
		model.SkillOpenPR, model.SkillFileIssue, model.SkillScanRegistry,
	}
}
