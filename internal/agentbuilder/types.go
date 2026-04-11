package agentbuilder

import "github.com/lleontor705/cortex-ia/internal/model"

// SDDIntegrationMode defines how the agent integrates with SDD.
type SDDIntegrationMode string

const (
	SDDFull  SDDIntegrationMode = "full"
	SDDPhase SDDIntegrationMode = "phase"
	SDDNone  SDDIntegrationMode = "none"
)

// AgentSpec describes a custom agent to generate.
type AgentSpec struct {
	Engine   model.AgentID
	Purpose  string
	SDDMode  SDDIntegrationMode
	SDDPhase string // only when SDDMode == SDDPhase
}

// GeneratedAgent is the result of agent generation.
type GeneratedAgent struct {
	Spec          AgentSpec
	SkillName     string // kebab-case skill name derived from purpose
	SkillContent  string // the SKILL.md content
	PromptContent string // system prompt content (if applicable)
}

// InstallResult describes the outcome of installing a generated agent.
type InstallResult struct {
	FilesWritten []string
	Err          error
}
