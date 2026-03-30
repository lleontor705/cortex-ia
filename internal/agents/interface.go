package agents

import "github.com/lleontor705/cortex-ia/internal/model"

// Adapter is the core abstraction for AI agent integration. Components use
// adapter methods instead of switch statements on AgentID, making it trivial
// to add new agents without modifying component code.
type Adapter interface {
	// Identity
	Agent() model.AgentID
	Tier() model.SupportTier

	// Detection — checks if the agent binary and config dir exist.
	Detect(homeDir string) (installed bool, binaryPath string, configPath string, configFound bool, err error)

	// Config paths — components use these instead of hardcoding paths per agent.
	GlobalConfigDir(homeDir string) string
	SystemPromptDir(homeDir string) string
	SystemPromptFile(homeDir string) string
	SkillsDir(homeDir string) string
	SettingsPath(homeDir string) string

	// Config strategies — HOW to inject content, not WHERE (that's paths above).
	SystemPromptStrategy() model.SystemPromptStrategy
	MCPStrategy() model.MCPStrategy

	// MCP path resolution — for agents using SeparateMCPFiles strategy.
	MCPConfigPath(homeDir string, serverName string) string

	// Capabilities — agents declare what they support.
	SupportsSkills() bool
	SupportsSystemPrompt() bool
	SupportsMCP() bool

	SupportsSlashCommands() bool
	CommandsDir(homeDir string) string

	// Sub-agent capabilities — determines multi-agent vs single-agent SDD.
	SupportsTaskDelegation() bool
	SupportsSubAgents() bool
	SubAgentsDir(homeDir string) string
}
