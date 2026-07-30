package model

// OpenCodeDefaultAssignments is retained as a compatibility boundary. It never
// invents assignments; callers must provide explicit configuration.
func OpenCodeDefaultAssignments() OpenCodeModelAssignments {
	return OpenCodeModelAssignments{}
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
	case "implement":
		return "Writes production code"
	case "validate":
		return "Verifies correctness"
	case "finalize":
		return "Archives and documents"
	case "parallel-dispatch":
		return "Runs independent tasks concurrently"
	}
	return ""
}
