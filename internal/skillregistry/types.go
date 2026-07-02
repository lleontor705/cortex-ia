// Package skillregistry provides deterministic scanning of skill directories
// across three tiers: embedded assets, project-level skills, and community skills.
package skillregistry

// SkillEntry represents a single skill discovered by the scanner.
type SkillEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Trigger     string `json:"trigger"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// RegistryOutput is the result of a skill registry scan.
type RegistryOutput struct {
	Skills  []SkillEntry `json:"skills"`
	Version string       `json:"version"`
}
