package hostbridge

import "strings"

// Roles are the installed role names, identical on every host.
var Roles = []string{"orchestrator", "discovery", "planner", "investigate", "implement", "reviewer"}

// toolRoles mirrors the role map of the OpenCode bridge
// (internal/assets/plugins/cortex-work.ts): tools absent from it are read-only
// and open to every role.
var toolRoles = map[string][]string{
	"cortex_ia_discovery_write":     {"discovery"},
	"cortex_ia_board_create":        {"planner"},
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
	"cortex_ia_work_release_all":    {"implement"},
	"cortex_ia_file_release":        {"implement"},
	"cortex_ia_work_transition":     {"implement"},
	"cortex_ia_work_approve":        {"reviewer", "orchestrator"},
	"cortex_ia_diagram_render":      {"implement"},
	"cortex_ia_workload_audit":      {"implement", "reviewer", "orchestrator"},
}

// unidentifiedRoleTools are the restricted tools a caller without a known role
// may still use. Hosts such as agy do not name the calling agent, so they can
// create and implement work, but approval and recovery of other agents' work
// need a known role; on such hosts the user approves from a terminal.
var unidentifiedRoleTools = map[string]bool{
	"cortex_ia_discovery_write":  true,
	"cortex_ia_board_create":     true,
	"cortex_ia_work_create":      true,
	"cortex_ia_work_recover_own": true,
	"cortex_ia_work_claim":       true,
	"cortex_ia_work_renew":       true,
	"cortex_ia_file_reserve":     true,
	"cortex_ia_work_release_all": true,
	"cortex_ia_file_release":     true,
	"cortex_ia_work_transition":  true,
	"cortex_ia_diagram_render":   true,
	"cortex_ia_workload_audit":   true,
}

// RoleMayCall reports whether role may call the bridge tool. An empty role
// means the host could not identify it; such callers get read-only tools and
// unidentifiedRoleTools.
func RoleMayCall(role, tool string) bool {
	allowed, restricted := toolRoles[tool]
	if !restricted {
		return true
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return unidentifiedRoleTools[tool]
	}
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
