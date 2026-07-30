package model

// Selection captures the user's choices from the TUI or CLI flags.
type Selection struct {
	Agents           []AgentID
	Preset           PresetID
	Components       []ComponentID
	DryRun           bool
	Persona          PersonaID        // "" = use professional default
	ModelAssignments ModelAssignments // nil = use balanced default
	ProfileName      string           // if set, load ModelAssignments from this profile
	StrictTDD        bool             // when true, enforce test-first development in SDD
	CommunitySkills  []SkillID        // community skills selected for installation
}

// ValidateCurrent rejects compatibility-only identifiers before a selection
// reaches dependency resolution or any mutation boundary.
func (s Selection) ValidateCurrent() error {
	return ValidateCurrentComponents(s.Components)
}
