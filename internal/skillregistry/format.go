package skillregistry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FormatMarkdown produces the .sdd/skill-registry.md content as a markdown
// table sorted by skill name. The output is deterministic — calling it twice
// with the same input always produces the same string.
func FormatMarkdown(out RegistryOutput) string {
	var sb strings.Builder

	sb.WriteString("# cortex-ia Skill Registry\n\n")
	sb.WriteString("## Skills\n\n")
	sb.WriteString("| Name | Source | Path | Trigger |\n")
	sb.WriteString("|---|---|---|---|\n")

	// Sort a copy for deterministic output regardless of input order.
	sorted := make([]SkillEntry, len(out.Skills))
	copy(sorted, out.Skills)
	sortByName(sorted)

	for _, s := range sorted {
		trigger := s.Trigger
		if trigger == "" {
			trigger = "N/A"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			s.Name, s.Category, s.Path, trigger))
	}

	return sb.String()
}

// FormatJSON produces deterministic JSON output sorted by skill name.
func FormatJSON(out RegistryOutput) (string, error) {
	// Sort a copy so the caller's slice is not mutated.
	sorted := make([]SkillEntry, len(out.Skills))
	copy(sorted, out.Skills)
	sortByName(sorted)

	clone := RegistryOutput{
		Skills:  sorted,
		Version: out.Version,
	}

	data, err := json.MarshalIndent(clone, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal registry JSON: %w", err)
	}
	return string(data), nil
}

// sortByName sorts skill entries alphabetically by Name in place.
func sortByName(skills []SkillEntry) {
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
}
