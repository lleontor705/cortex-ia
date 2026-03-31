package model

// Selection captures the user's choices from the TUI or CLI flags.
type Selection struct {
	Agents           []AgentID
	Preset           PresetID
	Components       []ComponentID
	Skills           []SkillID
	DryRun           bool
	Persona          PersonaID        // "" = use professional default
	ModelAssignments ModelAssignments // nil = use balanced default
}
