package model

import "fmt"

// Selection captures supported installer choices from the TUI or CLI flags.
type Selection struct {
	Agents     []AgentID
	Preset     PresetID
	Components []ComponentID
	DryRun     bool
	Persona    PersonaID // "" = use professional default
	// ModelAssignments and ProfileName are retained only to reject stale
	// programmatic selections. Supported installer routing must not set them.
	ModelAssignments ModelAssignments
	ProfileName      string
	StrictTDD        bool      // when true, enforce test-first development in SDD
	CommunitySkills  []SkillID // community skills selected for installation
}

// ValidateCurrent rejects compatibility-only identifiers before a selection
// reaches dependency resolution or any mutation boundary.
func (s Selection) ValidateCurrent() error {
	if s.ProfileName != "" {
		return fmt.Errorf("retired selection field %q is not supported", "profile")
	}
	if len(s.ModelAssignments) > 0 {
		return fmt.Errorf("retired selection field %q is not supported", "model-assignment")
	}
	return ValidateCurrentComponents(s.Components)
}

// ValidateCurrentComponents rejects unknown or removed component identifiers
// before dependency resolution or mutation.
func ValidateCurrentComponents(components []ComponentID) error {
	for _, component := range components {
		switch component {
		case ComponentCortex, ComponentMailbox, ComponentForgeSpec, ComponentSDD,
			ComponentSkills, ComponentContext7, ComponentConventions:
			continue
		default:
			return fmt.Errorf("component %q is not supported", component)
		}
	}
	return nil
}
