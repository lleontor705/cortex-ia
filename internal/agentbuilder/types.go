package agentbuilder

import "github.com/lleontor705/cortex-ia/internal/model"

// SDDIntegrationMode defines how the agent integrates with SDD.
type SDDIntegrationMode string

const (
	// Existing template-mode constants (kept for backward compatibility with
	// the original Generate() flow).
	SDDFull  SDDIntegrationMode = "full"
	SDDPhase SDDIntegrationMode = "phase"
	SDDNone  SDDIntegrationMode = "none"

	// New engine-driven constants. Match the gentle-ai naming so the parser /
	// prompt / SDD-injector code can be ported without renaming.
	SDDStandalone   SDDIntegrationMode = "standalone"
	SDDPhaseSupport SDDIntegrationMode = "phase-support"
	SDDNewPhase     SDDIntegrationMode = "new-phase"
)

// SDDIntegration carries the resolved SDD mode + (optional) phase pointer.
type SDDIntegration struct {
	Mode  SDDIntegrationMode
	Phase string // existing phase ID for SDDPhaseSupport, new phase name for SDDNewPhase
}

// AgentSpec describes a custom agent to generate (template/back-compat path).
type AgentSpec struct {
	Engine   model.AgentID
	Purpose  string
	SDDMode  SDDIntegrationMode
	SDDPhase string // only when SDDMode == SDDPhase
}

// GeneratedAgent is the result of agent generation.
//
// Two layers of fields:
//   - Legacy: Spec, SkillName, SkillContent, PromptContent — used by the
//     template-based Generate(spec) path.
//   - New: Name, Title, Description, Trigger, Content, SDDConfig — used by
//     the engine-driven GenerateWithEngine() path and the parser.
type GeneratedAgent struct {
	// Legacy fields (template path).
	Spec          AgentSpec
	SkillName     string
	SkillContent  string
	PromptContent string

	// New fields (engine path).
	Name        string          // kebab-case identifier
	Title       string          // human-friendly H1 title
	Description string          // one-line summary from `## Description`
	Trigger     string          // trigger text from `## Trigger`
	Content     string          // full SKILL.md body (post-fence-strip)
	SDDConfig   *SDDIntegration // pointer to keep the field optional/nullable
}

// InstallResult describes the outcome of installing a generated agent into a
// single adapter. Used both by the legacy Install(homeDir, agent) and the
// new InstallToAdapters multi-target installer.
type InstallResult struct {
	FilesWritten []string
	Err          error

	// Multi-installer additions:
	AgentID model.AgentID
	Path    string
	Success bool
}

// AdapterInfo bundles the bits the multi-installer needs about a target adapter.
type AdapterInfo struct {
	ID         model.AgentID
	SkillsDir  string // adapter.SkillsDir(homeDir)
	HasSkills  bool   // adapter.SupportsSkills()
	PromptFile string // adapter.SystemPromptFile(homeDir) — for SDD mode injection
}

// RegistryEntry is one row in the persistent agentbuilder registry.
type RegistryEntry struct {
	Name        string             `json:"name"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Engine      model.AgentID      `json:"engine"`
	SDDMode     SDDIntegrationMode `json:"sdd_mode,omitempty"`
	SDDPhase    string             `json:"sdd_phase,omitempty"`
	Targets     []model.AgentID    `json:"targets,omitempty"`
	Persona     model.PersonaID    `json:"persona,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
}

// Registry is the persisted JSON document at ~/.cortex-ia/agentbuilder/registry.json.
type Registry struct {
	Version int             `json:"version"`
	Agents  []RegistryEntry `json:"agents"`
}
