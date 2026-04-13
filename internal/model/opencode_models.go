package model

// OpenCodeDefaultAssignments returns default model assignments for OpenCode sub-agents.
// Uses Anthropic models as defaults since they're the most commonly used.
func OpenCodeDefaultAssignments() OpenCodeModelAssignments {
	return OpenCodeModelAssignments{
		"orchestrator":   {Provider: "anthropic", Model: "claude-opus-4-20250514"},
		"bootstrap":      {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"investigate":    {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"draft-proposal": {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"write-specs":    {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"architect":      {Provider: "anthropic", Model: "claude-opus-4-20250514"},
		"decompose":      {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"team-lead":      {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"implement":      {Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		"validate":       {Provider: "anthropic", Model: "claude-opus-4-20250514"},
		"finalize":       {Provider: "anthropic", Model: "claude-haiku-4-20250506"},
	}
}

// OpenCodeSubAgentDescription returns a human-readable description for a sub-agent.
func OpenCodeSubAgentDescription(name string) string {
	switch name {
	case "orchestrator":
		return "Coordinates all SDD phases"
	case "bootstrap":
		return "Initializes project context"
	case "investigate":
		return "Explores and diagnoses issues"
	case "draft-proposal":
		return "Creates change proposals"
	case "write-specs":
		return "Writes specifications"
	case "architect":
		return "Designs architecture"
	case "decompose":
		return "Breaks work into tasks"
	case "team-lead":
		return "Manages implementation"
	case "implement":
		return "Writes production code"
	case "validate":
		return "Verifies correctness"
	case "finalize":
		return "Archives and documents"
	}
	return ""
}
