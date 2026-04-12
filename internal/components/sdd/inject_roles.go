package sdd

// sddSkillIDs are the core SDD skills written by this component.
var sddSkillIDs = []string{
	"bootstrap", "investigate", "draft-proposal", "write-specs",
	"architect", "decompose", "team-lead", "implement", "validate", "finalize",
	"debate", "debug", "execute-plan", "ideate", "monitor",
	"open-pr", "file-issue", "parallel-dispatch", "scan-registry",
}

// sddSkillDescriptions maps each SDD skill ID to a short description used in
// sub-agent frontmatter so that OpenCode recognises the agent type.
var sddSkillDescriptions = map[string]string{
	"bootstrap":         "Bootstrap SDD context — detects project stack, conventions, and persistence backend",
	"investigate":       "Investigate codebase, analyze bugs, assess migrations, compare approaches. Supports FOCUS modes: ARCHITECTURE, INVESTIGATION, MIGRATION, GENERAL.",
	"draft-proposal":    "Create a change proposal from exploration results",
	"write-specs":       "Write delta specifications from an approved proposal",
	"architect":         "Create technical design decisions from an approved proposal",
	"decompose":         "Break specs and design into ordered implementation tasks",
	"team-lead":         "Owns the entire apply phase — executes task board groups sequentially, launches @implement sub-agents in parallel within each group, manages file reservations and retries.",
	"implement":         "Implement code changes following SDD specs, design, and task definitions. Supports TASK-TYPE modes: IMPLEMENTATION, REFACTOR, DATABASE, INFRASTRUCTURE, DOCUMENTATION.",
	"validate":          "Validate implementation against specs — runs tests, generates compliance matrix, and applies quality/security/performance review lenses.",
	"finalize":          "Archive completed SDD change — merges delta specs and closes the cycle",
	"debate":            "Moderates adversarial debates between competing approaches",
	"debug":             "Systematic root-cause debugging before proposing fixes",
	"execute-plan":      "Executes written implementation plans with review checkpoints",
	"ideate":            "Collaborative brainstorming to explore intent and requirements",
	"monitor":           "Generates dashboard visualising SDD pipeline state and tasks",
	"open-pr":           "Creates pull requests following issue-first enforcement",
	"file-issue":        "Creates GitHub issues using required templates",
	"parallel-dispatch": "Dispatches independent tasks to parallel sub-agents",
	"scan-registry":     "Scans skill directories and builds unified skill registry",
}

// coordinatorSkills lists skills that are allowed to delegate to sub-agents
// via the task tool. All other skills are leaf agents with task disabled.
var coordinatorSkills = map[string]bool{
	"team-lead":         true,
	"debate":            true,
	"parallel-dispatch": true,
}

// isCoordinator reports whether a skill is allowed to delegate to sub-agents.
func isCoordinator(skillID string) bool {
	return coordinatorSkills[skillID]
}

// agentRole classifies each SDD skill for tool matrix generation.
type agentRole int

const (
	roleLeafReader  agentRole = iota // read-only exploration (investigate, ideate)
	roleLeafPlanner                  // read-only planning (draft-proposal, write-specs, architect, decompose)
	roleLeafWriter                   // code-writing leaf (implement, bootstrap, finalize, debug, execute-plan)
	roleLeafOps                      // git/CI operations (open-pr, file-issue, scan-registry, monitor)
	roleLeafVerify                   // verification with execution (validate)
	roleCoordinator                  // can delegate via task() (team-lead, debate, parallel-dispatch)
)

// agentRoles maps each SDD skill to its role.
var agentRoles = map[string]agentRole{
	"bootstrap":         roleLeafWriter,
	"investigate":       roleLeafReader,
	"draft-proposal":    roleLeafPlanner,
	"write-specs":       roleLeafPlanner,
	"architect":         roleLeafPlanner,
	"decompose":         roleLeafPlanner,
	"team-lead":         roleCoordinator,
	"implement":         roleLeafWriter,
	"validate":          roleLeafVerify,
	"finalize":          roleLeafWriter,
	"debate":            roleCoordinator,
	"debug":             roleLeafWriter,
	"execute-plan":      roleLeafWriter,
	"ideate":            roleLeafReader,
	"monitor":           roleLeafOps,
	"open-pr":           roleLeafOps,
	"file-issue":        roleLeafOps,
	"parallel-dispatch": roleCoordinator,
	"scan-registry":     roleLeafOps,
}

// agentColors assigns a hex color per skill for the OpenCode UI.
var agentColors = map[string]string{
	"bootstrap":         "#607D8B",
	"investigate":       "#78909C",
	"draft-proposal":    "#90A4AE",
	"write-specs":       "#B0BEC5",
	"architect":         "#546E7A",
	"decompose":         "#455A64",
	"team-lead":         "#1565C0",
	"implement":         "#2E7D32",
	"validate":          "#F57F17",
	"finalize":          "#37474F",
	"debate":            "#6A1B9A",
	"debug":             "#C62828",
	"execute-plan":      "#1565C0",
	"ideate":            "#00838F",
	"monitor":           "#4527A0",
	"open-pr":           "#2E7D32",
	"file-issue":        "#EF6C00",
	"parallel-dispatch": "#00695C",
	"scan-registry":     "#795548",
	"orchestrator":      "#4A90D9",
}

// agentSteps returns the max agentic iterations for a skill.
func agentSteps(skillID string) int {
	switch skillID {
	case "team-lead":
		return 80
	case "implement":
		return 60
	case "orchestrator":
		return 50
	case "investigate", "validate":
		return 40
	default:
		return 30
	}
}

// agentTemperature returns the LLM temperature for a skill.
func agentTemperature(skillID string) float64 {
	switch skillID {
	case "validate":
		return 0.1
	case "investigate", "draft-proposal", "ideate":
		return 0.3
	default:
		return 0.2
	}
}

// toolsForRole returns the full OpenCode tools matrix for a given role.
// Starts from a base set of tools shared by all sub-agents, then applies
// role-specific overrides for permissions that differ.
func toolsForRole(role agentRole) map[string]any {
	// Base: read-only leaf agent with no write/delegation capabilities.
	tools := map[string]any{
		"bash": true, "read": true, "glob": true, "grep": true, "list": true,
		"question": true, "engram_*": true, "sdd_*": true, "msg_*": true,
		"tb_*": true, "cli_*": true,
		"edit": false, "write": false, "patch": false, "task": false,
		"lsp": false, "webfetch": false, "websearch": false,
		"skill": false, "todoread": false, "todowrite": false, "playwright_*": false,
	}

	switch role {
	case roleLeafReader:
		tools["lsp"] = true
		tools["webfetch"] = true
		tools["websearch"] = true
	case roleLeafPlanner:
		// Base is already correct for planners.
	case roleLeafWriter:
		tools["edit"] = true
		tools["write"] = true
		tools["patch"] = true
		tools["lsp"] = true
		tools["webfetch"] = true
		tools["file_*"] = true
	case roleLeafOps:
		tools["edit"] = true
		tools["write"] = true
	case roleLeafVerify:
		tools["task"] = true
		tools["webfetch"] = true
		tools["websearch"] = true
	case roleCoordinator:
		tools["bash"] = false
		tools["read"] = false
		tools["glob"] = false
		tools["grep"] = false
		tools["list"] = false
		tools["task"] = true
		tools["file_*"] = true
	}

	return tools
}
