package hostbridge

import "strings"

// Roles are the installed role names, identical on every host.
var Roles = []string{"orchestrator", "discovery", "planner", "investigate", "implement", "reviewer"}

// toolRoles mirrors the role map of the OpenCode bridge
// (internal/assets/plugins/cortex-work.ts): tools absent from it are read-only
// and open to every role.
var toolRoles = map[string][]string{
	"cortex_ia_discovery_write":     {"discovery"},
	"cortex_ia_board_create":        {"planner", "orchestrator"},
	"cortex_ia_work_create":         {"planner", "orchestrator"},
	"cortex_ia_work_recover":        {"orchestrator"},
	"cortex_ia_work_recover_own":    {"implement"},
	"cortex_ia_work_retry":          {"orchestrator"},
	"cortex_ia_work_review_refresh": {"orchestrator"},
	"cortex_ia_work_decompose":      {"planner"},
	"cortex_ia_work_reconcile":      {"orchestrator"},
	"cortex_ia_work_claim":          {"implement"},
	"cortex_ia_work_renew":          {"implement"},
	"cortex_ia_file_reserve":        {"implement"},
	"cortex_ia_work_lease_renew":    {"implement"},
	"cortex_ia_work_release_all":    {"implement"},
	"cortex_ia_file_release":        {"implement"},
	"cortex_ia_work_transition":     {"implement"},
	"cortex_ia_work_approve":        {"reviewer", "orchestrator"},
	"cortex_ia_diagram_render":      {"implement"},
}

// RoleMayCall reports whether role may call the bridge tool. An empty role
// means the host could not identify it; such callers get only read-only tools.
func RoleMayCall(role, tool string) bool {
	allowed, restricted := toolRoles[tool]
	if !restricted {
		return true
	}
	role = strings.ToLower(strings.TrimSpace(role))
	for _, candidate := range allowed {
		if candidate == role {
			return true
		}
	}
	return false
}

// AllowedRoles lists the roles permitted to call a restricted tool, or nil
// when the tool is open to every role.
func AllowedRoles(tool string) []string { return toolRoles[tool] }

// BridgeTool extracts the cortex_ia_* tool name from a host tool name such as
// mcp__cortex_ia__cortex_ia_work_claim. ok is false for any other tool.
func BridgeTool(hostToolName string) (string, bool) {
	const prefix = "mcp__" + MCPServerName + "__"
	if !strings.HasPrefix(hostToolName, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(hostToolName, prefix)
	if !strings.HasPrefix(name, "cortex_ia_") {
		return "", false
	}
	return name, true
}
