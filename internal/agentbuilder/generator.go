package agentbuilder

import (
	"fmt"
	"strings"
	"unicode"
)

// Generate creates a GeneratedAgent from a spec.
// It builds the SKILL.md content based on the purpose and SDD integration mode.
func Generate(spec AgentSpec) (*GeneratedAgent, error) {
	if strings.TrimSpace(spec.Purpose) == "" {
		return nil, fmt.Errorf("agentbuilder: purpose must not be empty")
	}

	name := toKebabCase(spec.Purpose)
	skill := buildSkillContent(name, spec)

	return &GeneratedAgent{
		Spec:         spec,
		SkillName:    name,
		SkillContent: skill,
	}, nil
}

// buildSkillContent constructs the full SKILL.md text from the spec.
func buildSkillContent(name string, spec AgentSpec) string {
	var sb strings.Builder

	// Title derived from purpose
	title := toTitle(spec.Purpose)

	// Frontmatter
	sb.WriteString(fmt.Sprintf("---\nname: %s\ndescription: %s\ntype: agent\n---\n\n", name, spec.Purpose))

	// Role section
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString("## Role\n\n")
	sb.WriteString(fmt.Sprintf("You are a specialized agent for: %s.\n", spec.Purpose))
	sb.WriteString(fmt.Sprintf("Powered by the %s engine.\n\n", string(spec.Engine)))

	// SDD integration section
	switch spec.SDDMode {
	case SDDFull:
		sb.WriteString("## SDD Integration (Full)\n\n")
		sb.WriteString("This agent participates in ALL SDD phases.\n\n")
		sb.WriteString("### Workflow\n\n")
		sb.WriteString("1. Respond to orchestrator delegations for any SDD phase.\n")
		sb.WriteString("2. Follow the SDD contract format: return `{ status, executive_summary, artifacts, next_recommended, risks }`.\n")
		sb.WriteString("3. Persist artifacts via `mem_save` with the appropriate `sdd/{change-name}/{phase}` topic key.\n")
		sb.WriteString("4. Use `mem_relate` to connect your output to upstream artifacts.\n\n")

	case SDDPhase:
		sb.WriteString(fmt.Sprintf("## SDD Integration (Phase: %s)\n\n", spec.SDDPhase))
		sb.WriteString(fmt.Sprintf("This agent specializes in the `%s` SDD phase.\n\n", spec.SDDPhase))
		sb.WriteString("### Workflow\n\n")
		sb.WriteString(fmt.Sprintf("1. Activate when the orchestrator delegates the `%s` phase.\n", spec.SDDPhase))
		sb.WriteString("2. Read upstream artifacts required by this phase.\n")
		sb.WriteString("3. Produce the phase artifact following the SDD contract format.\n")
		sb.WriteString(fmt.Sprintf("4. Persist output via `mem_save` with topic key `sdd/{change-name}/%s`.\n\n", spec.SDDPhase))

	case SDDNone:
		sb.WriteString("## Standalone Agent\n\n")
		sb.WriteString("This agent operates independently without SDD pipeline integration.\n\n")
		sb.WriteString("### Workflow\n\n")
		sb.WriteString("1. Receive user requests directly.\n")
		sb.WriteString("2. Perform the requested work.\n")
		sb.WriteString("3. Return results to the user.\n\n")
	}

	// Rules section
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- Follow all instructions precisely.\n")
	sb.WriteString("- Persist significant decisions via `mem_save`.\n")
	sb.WriteString("- Report blockers immediately rather than guessing.\n")

	return sb.String()
}

// toKebabCase converts a human-readable string to kebab-case.
// Example: "Fix Auth Bugs" -> "fix-auth-bugs"
func toKebabCase(s string) string {
	var parts []string
	var current strings.Builder

	for _, r := range strings.TrimSpace(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else if current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	result := strings.Join(parts, "-")

	// Enforce max length of 50 characters.
	if len(result) > 50 {
		result = result[:50]
		// Trim trailing hyphen if truncation landed on one.
		result = strings.TrimRight(result, "-")
	}

	return result
}

// toTitle converts a purpose string to a title by capitalizing the first letter
// of each word.
func toTitle(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
