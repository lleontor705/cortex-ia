package model

// AgentID identifies a supported AI coding agent.
type AgentID string

const (
	AgentClaudeCode    AgentID = "claude-code"
	AgentOpenCode      AgentID = "opencode"
	AgentGeminiCLI     AgentID = "gemini-cli"
	AgentCursor        AgentID = "cursor"
	AgentVSCodeCopilot AgentID = "vscode-copilot"
	AgentCodex         AgentID = "codex"
	AgentAntigravity   AgentID = "antigravity"
	AgentWindsurf      AgentID = "windsurf"
)

// SupportTier indicates how fully an agent supports the cortex-ia ecosystem.
type SupportTier string

const (
	TierFull SupportTier = "full"
)

// ComponentID identifies an installable ecosystem component.
type ComponentID string

const (
	ComponentCortex        ComponentID = "cortex"
	ComponentMailbox       ComponentID = "agent-mailbox"
	ComponentForgeSpec     ComponentID = "forgespec"
	ComponentSDD           ComponentID = "sdd"
	ComponentSkills        ComponentID = "skills"
	ComponentContext7       ComponentID = "context7"
	ComponentConventions   ComponentID = "conventions"
	ComponentGGA           ComponentID = "gga"
)

// SkillID identifies an SDD or utility skill.
type SkillID string

const (
	SkillSDDInit      SkillID = "sdd-init"
	SkillSDDExplore   SkillID = "sdd-explore"
	SkillSDDPropose   SkillID = "sdd-propose"
	SkillSDDSpec      SkillID = "sdd-spec"
	SkillSDDDesign    SkillID = "sdd-design"
	SkillSDDTasks     SkillID = "sdd-tasks"
	SkillSDDApply     SkillID = "sdd-apply"
	SkillSDDVerify    SkillID = "sdd-verify"
	SkillSDDArchive   SkillID = "sdd-archive"
	SkillTeamLead     SkillID = "team-lead"
	SkillDebug        SkillID = "debug"
	SkillIdeate       SkillID = "ideate"
	SkillDebate       SkillID = "debate"
	SkillMonitor      SkillID = "monitor"
	SkillExecutePlan  SkillID = "execute-plan"
	SkillOpenPR       SkillID = "open-pr"
	SkillFileIssue    SkillID = "file-issue"
	SkillScanRegistry SkillID = "scan-registry"
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

// ClaudeModelAlias identifies a Claude model tier for per-phase routing.
type ClaudeModelAlias string

const (
	ModelOpus   ClaudeModelAlias = "opus"
	ModelSonnet ClaudeModelAlias = "sonnet"
	ModelHaiku  ClaudeModelAlias = "haiku"
)

// ModelPreset identifies a predefined model assignment strategy.
type ModelPreset string

const (
	ModelPresetBalanced    ModelPreset = "balanced"
	ModelPresetPerformance ModelPreset = "performance"
	ModelPresetEconomy     ModelPreset = "economy"
)

// ModelAssignments maps SDD skill names to Claude model aliases.
type ModelAssignments map[string]ClaudeModelAlias

// Profile stores a named set of model assignments for reuse.
type Profile struct {
	Name             string                      `json:"name"`
	ModelAssignments map[string]ClaudeModelAlias `json:"model_assignments,omitempty"`
}

// --- OpenCode model types ---

// OpenCodeProvider represents a detected provider with its models.
type OpenCodeProvider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Models []OpenCodeModel  `json:"models"`
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
		"team-lead",
		"implement",
		"validate",
		"finalize",
		"parallel-dispatch",
	}
}
