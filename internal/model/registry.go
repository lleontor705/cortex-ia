package model

// RegistrySelection carries declarative registry intent from configuration.
// It is transport-only: populated by config and never an authority source.
// It deliberately holds no agents, tools, permissions, or bindings fields.
type RegistrySelection struct {
	// ConfigFile is the path of the configuration file this selection was
	// loaded from.
	ConfigFile string
	// CustomSkillPaths lists local directories containing custom skills.
	CustomSkillPaths []string
	// DisabledComponents lists optional components explicitly disabled.
	DisabledComponents []ComponentID
}
