package renderers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

type openCodePermissionRule struct {
	Pattern string
	Action  string
}

type openCodeToolPermission struct {
	Tool   string
	Action string
	Rules  []openCodePermissionRule
}

type openCodeAgentPolicy struct {
	RoleID      ir.SemanticID
	Name        string
	Description string
	Mode        string
	Model       string
	Temperature float64
	Steps       int
	Color       string
	Skill       ir.SemanticID
	Permissions []openCodeToolPermission
}

type openCodeCommandPolicy struct {
	ID          ir.SemanticID
	Name        string
	Description string
	Agent       string
	Subtask     bool
}

var openCodeCommandDefinitions = map[ir.SemanticID]openCodeCommandPolicy{
	"command/bootstrap":    {Name: "bootstrap", Description: "Activate SDD bootstrap for the current project", Subtask: true},
	"command/investigate":  {Name: "investigate", Description: "Activate investigation for a supplied topic", Subtask: true},
	"command/new-change":   {Name: "new-change", Description: "Activate a new SDD change request"},
	"command/continue":     {Name: "continue", Description: "Activate the next SDD phase"},
	"command/fast-forward": {Name: "fast-forward", Description: "Activate requested SDD planning dispatch"},
	"command/implement":    {Name: "implement", Description: "Activate implementation for ready SDD work", Subtask: true},
	"command/validate":     {Name: "validate", Description: "Activate validation for an SDD change", Subtask: true},
	"command/finalize":     {Name: "finalize", Description: "Activate finalization for a verified SDD change", Subtask: true},
	"command/debate":       {Name: "debate", Description: "Activate debate for an SDD topic"},
	"command/monitor":      {Name: "monitor", Description: "Activate monitoring for SDD state", Subtask: true},
	"command/tdd":          {Name: "tdd", Description: "Launch Fast-TDD micro loop for bounded feature or bugfix"},
	"command/hotfix":       {Name: "hotfix", Description: "Launch emergency hotfix triage and atomic patch"},
	"command/spike":        {Name: "spike", Description: "Launch exploratory spike and disposable proof-of-concept"},
	"command/review":       {Name: "review", Description: "Launch independent adversarial code review"},
}

func buildOpenCodeCommandPolicy(id ir.SemanticID, profile string) (openCodeCommandPolicy, error) {
	definition, ok := openCodeCommandDefinitions[id]
	if !ok {
		return openCodeCommandPolicy{}, fmt.Errorf("unknown OpenCode command %q", id)
	}
	if profile != "portable-sequential" && profile != "portable-flat" && profile != "native-advanced" {
		return openCodeCommandPolicy{}, fmt.Errorf("unsupported OpenCode command profile %q", profile)
	}
	definition.ID = id
	definition.Agent = "orchestrator"
	if profile == "portable-sequential" {
		definition.Subtask = false
	} else {
		definition.Agent = map[ir.SemanticID]string{
			"command/bootstrap": "bootstrap", "command/investigate": "investigate", "command/implement": "implement",
			"command/validate": "validate", "command/finalize": "finalize", "command/debate": "debate",
			"command/tdd": "implement", "command/hotfix": "implement", "command/spike": "investigate", "command/review": "reviewer",
		}[id]
		if definition.Agent == "" {
			definition.Agent = "orchestrator"
		}
	}
	if profile == "native-advanced" && id == "command/fast-forward" {
		definition.Agent = "parallel-dispatch"
	}
	if profile != "portable-sequential" && definition.Agent != "orchestrator" {
		definition.Subtask = true
	}
	return definition, nil
}

func buildOpenCodeAgentPolicy(role ir.Role, workflow ir.WorkflowIR, profile string, composition Composition) (openCodeAgentPolicy, error) {
	if profile != "portable-sequential" && profile != "portable-flat" && profile != "native-advanced" {
		return openCodeAgentPolicy{}, fmt.Errorf("unsupported OpenCode agent policy profile %q", profile)
	}
	policy := openCodeAgentPolicy{
		RoleID: role.ID, Name: openCodeSemanticName(role.ID), Description: role.Objective,
		Mode: "subagent", Temperature: openCodeRoleTemperature(role.ID), Steps: openCodeRoleSteps(role.ID),
		Color: openCodeRoleColor(role.ID),
	}
	if role.ID == "role/orchestrator" {
		policy.Mode = "primary"
	}
	if binding, ok := compositionSkillBinding(composition, role.ID); ok {
		policy.Skill = binding.Skill
	}
	if route, ok := openCodeModelRoute(composition, role.ID); ok {
		policy.Model = string(route.Primary.Provider) + "/" + string(route.Primary.Model)
	}
	policy.Permissions = openCodeRolePermissions(role, workflow, profile, policy.Skill)
	return policy, nil
}

func openCodeModelRoute(composition Composition, role ir.SemanticID) (ModelRoute, bool) {
	for _, route := range composition.ModelRoutes {
		if route.Role == role && route.Primary.Provider != "" && route.Primary.Model != "" {
			return route, true
		}
	}
	wantAsset := ir.SemanticID("asset/role/" + openCodeSemanticName(role) + "/binding")
	for _, asset := range composition.OperationalAssets {
		if asset.ID == wantAsset && asset.Route.Primary.Provider != "" && asset.Route.Primary.Model != "" {
			return asset.Route, true
		}
	}
	return ModelRoute{}, false
}

func openCodeRolePermissions(role ir.Role, workflow ir.WorkflowIR, profile string, skill ir.SemanticID) []openCodeToolPermission {
	permissions := make([]openCodeToolPermission, 0, 9)
	managedOrchestratorRead := role.ID == "role/orchestrator" && profile == "portable-flat"
	if managedOrchestratorRead {
		permissions = append(permissions,
			openCodeToolPermission{Tool: "read", Rules: openCodeManagedReadRules()},
			openCodeToolPermission{Tool: "glob", Action: "deny"},
			openCodeToolPermission{Tool: "grep", Action: "deny"},
			openCodeToolPermission{Tool: "list", Action: "deny"},
		)
	} else if slices.Contains(role.AllowedEffects, ir.Effect("filesystem/read")) {
		permissions = append(permissions,
			openCodeToolPermission{Tool: "read", Rules: openCodeReadRules()},
			openCodeToolPermission{Tool: "glob", Action: "allow"},
			openCodeToolPermission{Tool: "grep", Action: "allow"},
			openCodeToolPermission{Tool: "list", Action: "allow"},
		)
	} else {
		permissions = append(permissions,
			openCodeToolPermission{Tool: "read", Action: "deny"},
			openCodeToolPermission{Tool: "glob", Action: "deny"},
			openCodeToolPermission{Tool: "grep", Action: "deny"},
			openCodeToolPermission{Tool: "list", Action: "deny"},
		)
	}
	if slices.Contains(role.AllowedEffects, ir.Effect("filesystem/write")) {
		permissions = append(permissions, openCodeToolPermission{Tool: "edit", Action: "allow"})
	} else {
		permissions = append(permissions, openCodeToolPermission{Tool: "edit", Action: "deny"})
	}
	if slices.Contains(role.AllowedEffects, ir.Effect("process/execute")) {
		permissions = append(permissions, openCodeToolPermission{Tool: "bash", Rules: openCodeBashRules()})
	} else {
		permissions = append(permissions, openCodeToolPermission{Tool: "bash", Action: "deny"})
	}
	if managedOrchestratorRead {
		permissions = append(permissions, openCodeToolPermission{Tool: "external_directory", Rules: openCodeManagedReadRules()})
	}
	if skill != "" {
		permissions = append(permissions, openCodeToolPermission{Tool: "skill", Rules: []openCodePermissionRule{
			{Pattern: "*", Action: "deny"},
			{Pattern: strings.TrimPrefix(string(skill), "skill/"), Action: "allow"},
		}})
	}
	permissions = append(permissions, openCodeTaskPermission(role.ID, workflow, profile))
	return permissions
}

func openCodeTaskPermission(role ir.SemanticID, workflow ir.WorkflowIR, profile string) openCodeToolPermission {
	if profile == "portable-sequential" || role != "role/orchestrator" {
		return openCodeToolPermission{Tool: "task", Action: "deny"}
	}
	names := make([]string, 0, len(workflow.Roles))
	for _, candidate := range workflow.Roles {
		if candidate.ID != "role/orchestrator" {
			names = append(names, openCodeSemanticName(candidate.ID))
		}
	}
	rules := []openCodePermissionRule{{Pattern: "*", Action: "deny"}}
	for _, name := range names {
		rules = append(rules, openCodePermissionRule{Pattern: name, Action: "allow"})
	}
	return openCodeToolPermission{Tool: "task", Rules: rules}
}

func openCodeManagedReadRules() []openCodePermissionRule {
	return []openCodePermissionRule{
		{Pattern: "*", Action: "deny"},
		{Pattern: "~/.cortex-ia/opencode/root/**", Action: "allow"},
		{Pattern: "~/.cortex-ia/opencode/contracts/**", Action: "allow"},
	}
}

func openCodeReadRules() []openCodePermissionRule {
	return []openCodePermissionRule{
		{Pattern: "*", Action: "allow"},
		{Pattern: ".env", Action: "deny"},
		{Pattern: ".env.*", Action: "deny"},
		{Pattern: "*.pem", Action: "deny"},
		{Pattern: "*.key", Action: "deny"},
		{Pattern: "*.p12", Action: "deny"},
		{Pattern: "*.pfx", Action: "deny"},
		{Pattern: "credentials.json", Action: "deny"},
		{Pattern: "service-account.json", Action: "deny"},
		{Pattern: "**/secrets/**", Action: "deny"},
		{Pattern: "**/.secrets/**", Action: "deny"},
		{Pattern: ".env.example", Action: "allow"},
	}
}

func openCodeBashRules() []openCodePermissionRule {
	return []openCodePermissionRule{
		{Pattern: "*", Action: "ask"},
		{Pattern: "rm -rf /", Action: "deny"},
		{Pattern: "rm -rf /*", Action: "deny"},
		{Pattern: "rm -rf ~", Action: "deny"},
		{Pattern: "sudo rm -rf *", Action: "deny"},
		{Pattern: ":(){ :|:& };:", Action: "deny"},
	}
}

func openCodeRoleSteps(role ir.SemanticID) int {
	switch role {
	case "role/implement":
		return 60
	case "role/orchestrator":
		return 50
	case "role/investigate", "role/validate":
		return 40
	default:
		return 30
	}
}

func openCodeRoleTemperature(role ir.SemanticID) float64 {
	switch role {
	case "role/validate":
		return 0.1
	case "role/investigate", "role/draft-proposal":
		return 0.3
	default:
		return 0.2
	}
}

func openCodeRoleColor(role ir.SemanticID) string {
	return map[ir.SemanticID]string{
		"role/orchestrator": "#4A90D9", "role/bootstrap": "#607D8B", "role/investigate": "#78909C",
		"role/draft-proposal": "#90A4AE", "role/write-specs": "#B0BEC5", "role/architect": "#546E7A",
		"role/decompose": "#455A64", "role/implement": "#2E7D32", "role/validate": "#F57F17",
		"role/finalize": "#37474F", "role/debate": "#6A1B9A", "role/parallel-dispatch": "#00695C",
	}[role]
}
