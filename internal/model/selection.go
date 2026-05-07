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

// ExportConfig represents a shareable installation configuration.
type ExportConfig struct {
	Agents          []AgentID    `json:"agents"`
	Preset          PresetID     `json:"preset"`
	Persona         PersonaID    `json:"persona"`
	ModelPreset     ModelPreset  `json:"model_preset"`
	SDDEnabled      bool         `json:"sdd_enabled"`
	StrictTDD       bool         `json:"strict_tdd"`
	CommunitySkills []SkillID    `json:"community_skills,omitempty"`
}
