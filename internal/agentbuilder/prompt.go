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
// tone and route-resolution evidence so the generated skill stays coherent
// with the rest of the cortex-ia ecosystem. Legacy model assignments supplied
// to the compatibility-shaped API are intentionally ignored.
type RouteSelection struct {
	Route      string
	Provider   string
	Model      string
	Provenance string
	Resolved   bool
	Deferred   bool
}

// ComposePromptWithRoute composes a prompt with an explicit route decision.
// Concrete provider/model values require trusted configuration provenance.
func ComposePromptWithRoute(userInput string, sdd *SDDIntegration, installedTargets []model.AgentID, persona model.PersonaID, selection RouteSelection) (string, error) {
	if selection.Resolved {
		if selection.Route == "" || selection.Provider == "" || selection.Model == "" || !trustedRouteProvenance(selection.Provenance) {
			return "", fmt.Errorf("agentbuilder: resolved route requires trusted configuration provenance")
		}
	} else if selection.Route == "" && !selection.Deferred {
		return "", fmt.Errorf("agentbuilder: route decision is unresolved")
	}
	p := composePromptBody(userInput, sdd, installedTargets, persona, selection)
	return p, nil
}

func trustedRouteProvenance(source string) bool {
	return source == "user-config" || source == "provider-config"
}

func ComposePrompt(
	userInput string,
	sdd *SDDIntegration,
	installedTargets []model.AgentID,
	persona model.PersonaID,
	models any,
) string {
	return composePromptBody(userInput, sdd, installedTargets, persona, RouteSelection{Deferred: true})
}

/* prompt body continues below */
func composePromptBody(
	userInput string,
	sdd *SDDIntegration,
	installedTargets []model.AgentID,
	persona model.PersonaID,
	selection RouteSelection,
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
			sb.WriteString("call cortex memory tools (`cortex_save`, `cortex_search`, ")
			sb.WriteString("`cortex_get_observation`, `cortex_relate`, `cortex_graph`), forgespec ")
			sb.WriteString("tools (`sdd_validate`, `sdd_save`, `sdd_list`, `sdd_get`, ")
			sb.WriteString("`tb_create_board`, `tb_status`, `tb_claim`, `tb_update`, ")
			sb.WriteString("`tb_get`, `tb_unblocked`, `file_reserve`, `file_release`) ")
			sb.WriteString("when useful.\n\n")
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

	if selection.Route != "" {
		fmt.Fprintf(&sb, "## Route resolution\nRoute: %s\n", selection.Route)
		if selection.Resolved {
			fmt.Fprintf(&sb, "Configured provider/model: %s/%s\nConfiguration source: %s\n\n", selection.Provider, selection.Model, selection.Provenance)
		} else {
			sb.WriteString("Concrete provider/model resolution is deferred until execution.\n\n")
		}
	} else if selection.Deferred {
		sb.WriteString("## Route resolution\nConcrete provider/model resolution is unresolved and MUST fail closed before execution.\n\n")
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
	sb.WriteString("- Reference cortex MCP for persistence (`cortex_save`, `cortex_search`).\n")
	sb.WriteString("- Reference forgespec MCP for spec/proposal artifacts.\n")
	sb.WriteString("- License: Apache-2.0. Front-matter author: cortex-ia.\n")
	sb.WriteString("- Markers used by cortex-ia in injected files have prefix `<!-- cortex-ia:* -->`.\n")

	return sb.String()
}
