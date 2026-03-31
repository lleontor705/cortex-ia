package model

import (
	"fmt"
	"sort"
	"strings"
)

// ModelsForPreset returns the model assignments for a named preset.
func ModelsForPreset(preset ModelPreset) ModelAssignments {
	switch preset {
	case ModelPresetPerformance:
		return ModelAssignments{
			"orchestrator":    ModelOpus,
			"investigate":     ModelSonnet,
			"draft-proposal":  ModelOpus,
			"write-specs":     ModelSonnet,
			"architect":       ModelOpus,
			"decompose":       ModelSonnet,
			"team-lead":       ModelSonnet,
			"implement":       ModelSonnet,
			"validate":        ModelOpus,
			"finalize":        ModelHaiku,
		}
	case ModelPresetEconomy:
		return ModelAssignments{
			"orchestrator":    ModelSonnet,
			"investigate":     ModelSonnet,
			"draft-proposal":  ModelSonnet,
			"write-specs":     ModelSonnet,
			"architect":       ModelSonnet,
			"decompose":       ModelHaiku,
			"team-lead":       ModelHaiku,
			"implement":       ModelSonnet,
			"validate":        ModelSonnet,
			"finalize":        ModelHaiku,
		}
	default: // balanced
		return ModelAssignments{
			"orchestrator":    ModelOpus,
			"investigate":     ModelSonnet,
			"draft-proposal":  ModelSonnet,
			"write-specs":     ModelSonnet,
			"architect":       ModelOpus,
			"decompose":       ModelHaiku,
			"team-lead":       ModelSonnet,
			"implement":       ModelSonnet,
			"validate":        ModelOpus,
			"finalize":        ModelHaiku,
		}
	}
}

// FormatModelAssignments returns a markdown table for prompt injection.
func FormatModelAssignments(m ModelAssignments) string {
	if len(m) == 0 {
		return "No model assignments configured — use default model for all phases."
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("| Phase | Model |\n|-------|-------|\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %s |\n", k, m[k])
	}
	return b.String()
}
