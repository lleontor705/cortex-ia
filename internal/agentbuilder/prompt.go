package agentbuilder

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// ComposePrompt builds the system prompt the GenerationEngine receives.
//
// The prompt is cortex-ia-aware: it instructs the engine to write a SKILL.md
// that references the cortex MCP (memory) and forgespec (artifact contracts)
// rather than gentle-ai's Engram. It also injects the user's selected persona
// tone and an optional model-preset hint so the generated skill stays coherent
// with the rest of the cortex-ia ecosystem.
func ComposePrompt(
	userInput string,
	sdd *SDDIntegration,
	installedTargets []model.AgentID,
	persona model.PersonaID,
	models model.ModelAssignments,
) string {
	var sb strings.Builder

	sb.WriteString("You are cortex-ia's skill generator. Produce a single ")
	sb.WriteString("SKILL.md document that follows the schema below. Output ")
	sb.WriteString("ONLY the SKILL.md content — no commentary, no code fences ")
	sb.WriteString("around the whole document.\n\n")

	sb.WriteString("## Required Schema\n")
	sb.WriteString("```\n")
	sb.WriteString("# <Title>\n\n")
	sb.WriteString("## Description\n<one-line summary>\n\n")
	sb.WriteString("## Trigger\n<when to load this skill>\n\n")
	sb.WriteString("## Instructions\n<the prompt body — multi-paragraph allowed>\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## User intent\n")
	sb.WriteString(strings.TrimSpace(userInput))
	sb.WriteString("\n\n")

	if sdd != nil && sdd.Mode != "" && sdd.Mode != SDDNone {
		sb.WriteString("## SDD integration\n")
		switch sdd.Mode {
		case SDDStandalone, SDDFull:
			sb.WriteString("Standalone — the skill is loaded on its own trigger; it can ")
			sb.WriteString("call cortex (`mem_save`, `mem_search`) and forgespec tools ")
			sb.WriteString("(`forgespec_proposal_*`, `forgespec_spec_*`) when useful.\n\n")
		case SDDPhaseSupport:
			fmt.Fprintf(&sb,
				"Phase-support — augments the existing SDD phase %q. "+
					"It MUST coexist with the canonical %s skill (do not contradict it).\n\n",
				sdd.Phase, sdd.Phase)
		case SDDNewPhase:
			fmt.Fprintf(&sb,
				"New phase named %q — slot it after the closest existing SDD phase "+
					"and write/read forgespec artifacts the next phase downstream will need.\n\n",
				sdd.Phase)
		case SDDPhase:
			fmt.Fprintf(&sb, "Bound to SDD phase %q (legacy mode).\n\n", sdd.Phase)
		}
	}

	if persona != "" {
		sb.WriteString("## Tone (persona)\n")
		switch persona {
		case model.PersonaProfessional:
			sb.WriteString("Professional — concise, no hedging, no emojis.\n\n")
		case model.PersonaMentor:
			sb.WriteString("Mentor — explain trade-offs, anticipate beginner questions, ")
			sb.WriteString("show one canonical example.\n\n")
		case model.PersonaMinimal:
			sb.WriteString("Minimal — terse imperative bullets only.\n\n")
		default:
			fmt.Fprintf(&sb, "%s\n\n", persona)
		}
	}

	if len(models) > 0 {
		sb.WriteString("## Model hints (optional)\n")
		sb.WriteString("If the skill needs to recommend a model for a phase, prefer:\n")
		for phase, alias := range models {
			fmt.Fprintf(&sb, "  - %s → %s\n", phase, alias)
		}
		sb.WriteString("\n")
	}

	if len(installedTargets) > 0 {
		sb.WriteString("## Target agents\n")
		sb.WriteString("This skill will be installed into: ")
		ids := make([]string, len(installedTargets))
		for i, id := range installedTargets {
			ids[i] = string(id)
		}
		sb.WriteString(strings.Join(ids, ", "))
		sb.WriteString(".\nKeep instructions tool-agnostic so it loads correctly in all of them.\n\n")
	}

	sb.WriteString("## Conventions\n")
	sb.WriteString("- Reference cortex MCP for persistence (`mem_save`, `mem_search`).\n")
	sb.WriteString("- Reference forgespec MCP for spec/proposal artifacts.\n")
	sb.WriteString("- License: Apache-2.0. Front-matter author: cortex-ia.\n")
	sb.WriteString("- Markers used by cortex-ia in injected files have prefix `<!-- cortex-ia:* -->`.\n")

	return sb.String()
}
