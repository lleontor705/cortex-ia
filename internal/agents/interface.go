package agents

import (
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// CapabilityProvider is the optional capability qualification surface exposed
// by adapters. It deliberately contains no scheduler, launcher, or runtime-
// state API; external runtimes remain responsible for execution.
type CapabilityProvider interface {
	CapabilityFacts() []capability.CapabilityFact
	CapabilityProber() capability.Prober
}

// SkillLayoutProvider is the optional declarative skill-layout surface
// exposed by adapters that represent custom skills on their host. Host
// representation stays adapter-owned: declaring a layout grants no registry,
// command, subagent, config, tool, permission, or binding authority, and
// custom skills always lower to plain SKILL.md data assets.
type SkillLayoutProvider interface {
	// SkillDestinations returns the relative host destinations for one
	// verified custom skill. Destinations are slash-separated paths
	// relative to the home directory, deterministic and stably ordered for
	// a given skill, and always located beneath the adapter's SkillsDir.
	//
	// The method receives the typed skillcore Skill only — never YAML
	// bytes, file paths, or provenance evidence — and is a pure
	// declaration: it must not touch the filesystem or mutate state, so
	// planning can call it freely before any write is planned or made.
	SkillDestinations(skill skillcore.Skill) []string
}

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

	// SystemPromptStrategy returns how the agent's system prompt file should be modified.
	// Currently, all strategies use InjectMarkdownSection (marker-based injection) which
	// works universally with any Markdown file. The strategy field is metadata for future
	// use if an agent requires a different injection mechanism.
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

	// Auto-install — agents that can be installed via package managers.
	SupportsAutoInstall() bool
	InstallCommands(profile system.PlatformProfile) [][]string
}
