package model

import "github.com/lleontor705/cortex-ia/internal/modelroute"

// AgentID identifies a supported AI coding agent.
type AgentID string

const (
	AgentOpenCode AgentID = "opencode"
)

// SupportTier indicates how fully an agent supports the cortex-ia ecosystem.
type SupportTier string

const (
	TierFull SupportTier = "full"
)

// ComponentID identifies an installable ecosystem component.
type ComponentID string

const (
	ComponentCortex      ComponentID = "cortex"
	ComponentMailbox     ComponentID = "agent-mailbox"
	ComponentForgeSpec   ComponentID = "forgespec"
	ComponentSDD         ComponentID = "sdd"
	ComponentSkills      ComponentID = "skills"
	ComponentContext7    ComponentID = "context7"
	ComponentConventions ComponentID = "conventions"
	ComponentPersona     ComponentID = "persona"
	ComponentPermissions ComponentID = "permissions"
	ComponentTheme       ComponentID = "theme"
)

// SkillID identifies an SDD or utility skill.
type SkillID string

const (
	SkillSDDInit          SkillID = "sdd-init"
	SkillSDDExplore       SkillID = "sdd-explore"
	SkillSDDPropose       SkillID = "sdd-propose"
	SkillSDDSpec          SkillID = "sdd-spec"
	SkillSDDDesign        SkillID = "sdd-design"
	SkillSDDTasks         SkillID = "sdd-tasks"
	SkillSDDApply         SkillID = "sdd-apply"
	SkillSDDVerify        SkillID = "sdd-verify"
	SkillSDDArchive       SkillID = "sdd-archive"
	SkillDebug            SkillID = "debug"
	SkillIdeate           SkillID = "ideate"
	SkillDebate           SkillID = "debate"
	SkillMonitor          SkillID = "monitor"
	SkillExecutePlan      SkillID = "execute-plan"
	SkillOpenPR           SkillID = "open-pr"
	SkillFileIssue        SkillID = "file-issue"
	SkillScanRegistry     SkillID = "scan-registry"
	SkillJudgmentDay      SkillID = "judgment-day"
	SkillParallelDispatch SkillID = "parallel-dispatch"

	// Skills ported from gentle-ai in the port-gentle-ai-patterns change.
	SkillWorkUnitCommits SkillID = "work-unit-commits"
	SkillChainedPR       SkillID = "chained-pr"
	SkillCognitiveDoc    SkillID = "cognitive-doc-design"
	SkillCommentWriter   SkillID = "comment-writer"
	SkillGoTesting       SkillID = "go-testing"
	SkillSkillCreator    SkillID = "skill-creator"
	SkillSkillImprover   SkillID = "skill-improver"
	SkillOnboard         SkillID = "onboard"
)

// SystemPromptStrategy defines how an agent's system prompt file is managed.
type SystemPromptStrategy int

const (
	// StrategyMarkdownSections uses <!-- cortex-ia:ID --> markers to inject sections
	// into an existing file without clobbering user content.
	StrategyMarkdownSections SystemPromptStrategy = iota
	// StrategyFileReplace replaces the entire system prompt file.
	StrategyFileReplace
	// StrategyAppendToFile appends content to an existing system prompt file.
	StrategyAppendToFile
)

// MCPStrategy defines how MCP server configs are written for an agent.
type MCPStrategy int

const (
	// StrategySeparateMCPFiles writes one JSON file per server in a dedicated directory.
	StrategySeparateMCPFiles MCPStrategy = iota
	// StrategyMergeIntoSettings merges mcpServers into a settings file.
	StrategyMergeIntoSettings
	// StrategyMCPConfigFile writes to a dedicated mcp.json config file.
	StrategyMCPConfigFile
	// StrategyTOMLFile writes MCP config to a TOML file.
	StrategyTOMLFile
)

// PresetID identifies an installation preset.
type PresetID string

const (
	PresetFull    PresetID = "full"
	PresetMinimal PresetID = "minimal"
	PresetCustom  PresetID = "custom"
)

// PersonaID identifies a communication style persona.
type PersonaID string

const (
	PersonaProfessional PersonaID = "professional"
	PersonaMentor       PersonaID = "mentor"
	PersonaMinimal      PersonaID = "minimal"
)

// ModelPreset identifies a predefined model assignment strategy.
type ModelPreset string

const (
	ModelPresetBalanced    ModelPreset = "balanced"
	ModelPresetPerformance ModelPreset = "performance"
	ModelPresetEconomy     ModelPreset = "economy"
	ModelPresetFast        ModelPreset = "fast"
	ModelPresetCodex       ModelPreset = "codex"
)

// ModelAssignments maps SDD skill names to explicit configured provider/model
// values or opaque semantic route identifiers.
type ModelAssignments map[string]string

// RouteAssignments stores semantic phase routes. It deliberately has no
// provider/model values; those are carried separately as explicit config.
type RouteAssignments map[string]modelroute.RouteRequest

// Profile stores a named set of semantic routes and explicit configured
// provider/model values for reuse. ModelAssignments is retained only as an
// in-memory source-compatibility field and is never serialized.
type Profile struct {
	Name                  string                             `json:"name"`
	Routes                RouteAssignments                   `json:"routes,omitempty"`
	ConfiguredAssignments map[string]OpenCodeModelAssignment `json:"configured_assignments,omitempty"`
	ModelAssignments      map[string]string                  `json:"-"`
}

// --- OpenCode model types ---

// OpenCodeProvider represents a detected provider with its models.
type OpenCodeProvider struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Models []OpenCodeModel `json:"models"`
}

// OpenCodeModel represents a model available in OpenCode.
type OpenCodeModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToolCall bool   `json:"tool_call"`
}

// OpenCodeModelAssignment maps a sub-agent to a provider/model pair.
type OpenCodeModelAssignment struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// OpenCodeModelAssignments maps sub-agent names to provider/model pairs.
type OpenCodeModelAssignments map[string]OpenCodeModelAssignment

// FormatOpenCodeModel returns "provider/model" string used in OpenCode config.
func (a OpenCodeModelAssignment) FormatOpenCodeModel() string {
	if a.Provider == "" || a.Model == "" {
		return ""
	}
	return a.Provider + "/" + a.Model
}

// OpenCodeSubAgents returns the ordered list of SDD sub-agent names
// that are registered in opencode.json as agents.
func OpenCodeSubAgents() []string {
	return []string{
		"orchestrator",
		"bootstrap",
		"investigate",
		"draft-proposal",
		"write-specs",
		"architect",
		"decompose",
		"implement",
		"validate",
		"finalize",
		"debate",
		"parallel-dispatch",
	}
}
