package renderers

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestOpenCodeRendererEmitsTwelveAgentsForEveryProfile(t *testing.T) {
	for _, profile := range []string{"portable-sequential", "portable-flat", "native-advanced"} {
		t.Run(profile, func(t *testing.T) {
			resolved := ResolvedWorkflow{
				Target: "opencode", Profile: profile, Workflow: canonicalOpenCodeWorkflow(),
				GenerationFingerprint: strings.Repeat("a", 64),
				AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetCommand, AssetAgent},
				AllowedPermissions:    []string{"filesystem/read", "filesystem/write", "process/execute"},
			}
			if profile == "portable-flat" {
				resolved.Capabilities = qualifiedDirectChild()
			}
			if profile == "native-advanced" {
				resolved.Capabilities = qualifiedNative()
				resolved.Extensions = []ExtensionDeclaration{{ID: "opencode/native-advanced", Optional: true}}
			}
			bundle, err := Render(context.Background(), NewOpenCodeRenderer(), resolved)
			if err != nil {
				t.Fatal(err)
			}
			agents := 0
			for _, asset := range bundle.Assets {
				if asset.Kind != AssetAgent {
					continue
				}
				agents++
				if asset.Path == "agents/orchestrator.md" && !strings.Contains(string(asset.Content), "mode: primary") {
					t.Errorf("orchestrator is not primary:\n%s", asset.Content)
				}
				if asset.Path != "agents/orchestrator.md" && !strings.Contains(string(asset.Content), "mode: subagent") {
					t.Errorf("%s is not a subagent", asset.Path)
				}
			}
			if agents != 12 {
				t.Fatalf("agent assets = %d, want 12", agents)
			}
		})
	}
}

func TestOpenCodeAgentPolicyDerivesPermissionsAndQualifiedModel(t *testing.T) {
	workflow := canonicalOpenCodeWorkflow()
	composition := openCodePolicyComposition(workflow)
	implement := canonicalRole(t, workflow, "role/implement")
	policy, err := buildOpenCodeAgentPolicy(implement, workflow, "portable-flat", composition)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Mode != "subagent" || policy.Model != "provider-test/model-test" || policy.Steps != 60 || policy.Temperature != 0.2 {
		t.Fatalf("implement policy = %+v", policy)
	}
	assertFlatPermission(t, policy, "edit", "allow")
	assertRulePermission(t, policy, "bash", "*", "ask")
	assertFlatPermission(t, policy, "task", "deny")
	assertRulePermission(t, policy, "skill", "*", "deny")
	assertRulePermission(t, policy, "skill", "implement", "allow")
	assertRulePermission(t, policy, "read", ".env.*", "deny")
	assertRulePermission(t, policy, "read", ".env.example", "allow")
}

func TestOpenCodeAgentPolicyRestrictsTaskByProfile(t *testing.T) {
	workflow := canonicalOpenCodeWorkflow()
	composition := openCodePolicyComposition(workflow)
	for _, tc := range []struct {
		profile string
		role    ir.SemanticID
		allows  []string
	}{
		{profile: "portable-sequential", role: "role/orchestrator"},
		{profile: "portable-flat", role: "role/orchestrator", allows: elevenSubagentNames()},
		{profile: "portable-flat", role: "role/debate"},
		{profile: "native-advanced", role: "role/debate", allows: ninePhaseRoleNames(workflow)},
		{profile: "native-advanced", role: "role/parallel-dispatch", allows: ninePhaseRoleNames(workflow)},
	} {
		t.Run(tc.profile+"/"+string(tc.role), func(t *testing.T) {
			policy, err := buildOpenCodeAgentPolicy(canonicalRole(t, workflow, tc.role), workflow, tc.profile, composition)
			if err != nil {
				t.Fatal(err)
			}
			permission := openCodePermissionFor(t, policy, "task")
			if len(tc.allows) == 0 {
				if permission.Action != "deny" || len(permission.Rules) != 0 {
					t.Fatalf("task permission = %+v, want deny", permission)
				}
				return
			}
			if len(permission.Rules) != len(tc.allows)+1 || permission.Rules[0] != (openCodePermissionRule{Pattern: "*", Action: "deny"}) {
				t.Fatalf("task rules = %+v", permission.Rules)
			}
			got := make([]string, 0, len(permission.Rules)-1)
			for _, rule := range permission.Rules[1:] {
				if rule.Action != "allow" {
					t.Fatalf("task rule = %+v, want allow", rule)
				}
				got = append(got, rule.Pattern)
			}
			if !slices.Equal(got, tc.allows) {
				t.Fatalf("task allowlist = %v, want %v", got, tc.allows)
			}
		})
	}
}

func TestRenderOpenCodeAgentUsesNativeOnDemandSkill(t *testing.T) {
	workflow := canonicalOpenCodeWorkflow()
	composition := openCodePolicyComposition(workflow)
	content := string(renderOpenCodeAgent(canonicalRole(t, workflow, "role/implement"), workflow, "portable-flat", composition))
	for _, marker := range []string{"mode: subagent", "model: provider-test/model-test", "permission:", `"*": deny`, "implement: allow", "Invoke the canonical skill `implement` with the native `skill` tool"} {
		if !strings.Contains(content, marker) {
			t.Errorf("agent missing %q:\n%s", marker, content)
		}
	}
	for _, deprecated := range []string{"tools:", "maxSteps", "native-preload"} {
		if strings.Contains(content, deprecated) {
			t.Errorf("agent contains deprecated %q:\n%s", deprecated, content)
		}
	}
}

func TestOpenCodeCommandPolicyRoutesByProfile(t *testing.T) {
	if len(openCodeCommandDefinitions) != 10 {
		t.Fatalf("OpenCode command definitions = %d, want 10", len(openCodeCommandDefinitions))
	}
	for id := range openCodeCommandDefinitions {
		policy, err := buildOpenCodeCommandPolicy(id, "portable-sequential")
		if err != nil {
			t.Fatal(err)
		}
		if policy.Agent != "orchestrator" || policy.Subtask {
			t.Errorf("sequential command %q = %+v, want non-subtask orchestrator", id, policy)
		}
	}
	for _, tc := range []struct {
		profile string
		command ir.SemanticID
		agent   string
		subtask bool
	}{
		{profile: "portable-sequential", command: "command/implement", agent: "orchestrator", subtask: false},
		{profile: "portable-sequential", command: "command/debate", agent: "orchestrator", subtask: false},
		{profile: "portable-flat", command: "command/implement", agent: "implement", subtask: true},
		{profile: "portable-flat", command: "command/debate", agent: "debate", subtask: true},
		{profile: "portable-flat", command: "command/fast-forward", agent: "orchestrator", subtask: false},
		{profile: "native-advanced", command: "command/fast-forward", agent: "parallel-dispatch", subtask: true},
		{profile: "native-advanced", command: "command/monitor", agent: "orchestrator", subtask: true},
	} {
		t.Run(tc.profile+"/"+string(tc.command), func(t *testing.T) {
			policy, err := buildOpenCodeCommandPolicy(tc.command, tc.profile)
			if err != nil {
				t.Fatal(err)
			}
			if policy.Agent != tc.agent || policy.Subtask != tc.subtask {
				t.Fatalf("command policy = %+v, want agent %q subtask %t", policy, tc.agent, tc.subtask)
			}
		})
	}
}

func TestRenderOpenCodeCommandPreservesBodyAndArguments(t *testing.T) {
	policy, err := buildOpenCodeCommandPolicy("command/validate", "portable-flat")
	if err != nil {
		t.Fatal(err)
	}
	content, err := renderOpenCodeCommand(policy, []byte("---\ndescription: stale\nagent: orchestrator\nsubtask: true\n---\n\nCanonical command body.\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"description: Activate validation for an SDD change", "agent: validate", "subtask: true", "Canonical command body.", "$ARGUMENTS"} {
		if !strings.Contains(string(content), marker) {
			t.Errorf("rendered command missing %q:\n%s", marker, content)
		}
	}
	if strings.Contains(string(content), "description: stale") {
		t.Fatalf("rendered command retained stale frontmatter:\n%s", content)
	}
}

func openCodePolicyComposition(workflow ir.WorkflowIR) Composition {
	bindings := make([]SkillBinding, 0, len(workflow.Roles))
	routes := make([]ModelRoute, 0, len(workflow.Roles))
	for _, role := range workflow.Roles {
		skill := canonicalCompositionSkills[role.ID]
		bindings = append(bindings, SkillBinding{Role: role.ID, Skill: skill, Mode: SkillModeNativeOnDemand, Path: "skills/" + strings.TrimPrefix(string(skill), "skill/") + "/SKILL.md", Hash: "hash-" + string(role.ID)})
		routes = append(routes, ModelRoute{Role: role.ID, PrimaryID: "route/v1/test", Primary: modelroute.RouteRef{Provider: "provider-test", Model: "model-test"}})
	}
	return Composition{SkillBindings: bindings, ModelRoutes: routes}
}

func canonicalOpenCodeWorkflow() ir.WorkflowIR {
	roles := []ir.Role{
		{ID: "role/orchestrator"},
		{ID: "role/bootstrap", AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write"}},
		{ID: "role/investigate", AllowedEffects: []ir.Effect{"filesystem/read"}},
		{ID: "role/draft-proposal"},
		{ID: "role/write-specs"},
		{ID: "role/architect"},
		{ID: "role/decompose"},
		{ID: "role/implement", AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write", "process/execute"}},
		{ID: "role/validate", AllowedEffects: []ir.Effect{"filesystem/read", "process/execute"}},
		{ID: "role/finalize", AllowedEffects: []ir.Effect{"filesystem/write"}},
		{ID: "role/debate"},
		{ID: "role/parallel-dispatch"},
	}
	for index := range roles {
		roles[index].Objective = "Execute " + string(roles[index].ID)
	}
	phases := []ir.Phase{
		{ID: "phase/bootstrap", Role: "role/bootstrap"},
		{ID: "phase/investigate", Role: "role/investigate"},
		{ID: "phase/propose", Role: "role/draft-proposal"},
		{ID: "phase/spec", Role: "role/write-specs"},
		{ID: "phase/design", Role: "role/architect"},
		{ID: "phase/tasks", Role: "role/decompose"},
		{ID: "phase/apply", Role: "role/implement"},
		{ID: "phase/verify", Role: "role/validate"},
		{ID: "phase/archive", Role: "role/finalize"},
	}
	return ir.WorkflowIR{SchemaVersion: ir.WorkflowSchema.Current, ID: "workflow/sdd", Version: ir.MustParseVersion("1.0.0"), Roles: roles, Phases: phases}
}

func canonicalRole(t *testing.T, workflow ir.WorkflowIR, id ir.SemanticID) ir.Role {
	t.Helper()
	for _, role := range workflow.Roles {
		if role.ID == id {
			return role
		}
	}
	t.Fatalf("role %q not found", id)
	return ir.Role{}
}

func elevenSubagentNames() []string {
	return []string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize", "debate", "parallel-dispatch"}
}

func ninePhaseRoleNames(workflow ir.WorkflowIR) []string {
	result := make([]string, 0, len(workflow.Phases))
	for _, phase := range workflow.Phases {
		name := openCodeSemanticName(phase.Role)
		if !slices.Contains(result, name) {
			result = append(result, name)
		}
	}
	return result
}

func assertFlatPermission(t *testing.T, policy openCodeAgentPolicy, tool, action string) {
	t.Helper()
	permission := openCodePermissionFor(t, policy, tool)
	if permission.Action != action || len(permission.Rules) != 0 {
		t.Errorf("%s permission = %+v, want %s", tool, permission, action)
	}
}

func assertRulePermission(t *testing.T, policy openCodeAgentPolicy, tool, pattern, action string) {
	t.Helper()
	permission := openCodePermissionFor(t, policy, tool)
	if !slices.Contains(permission.Rules, openCodePermissionRule{Pattern: pattern, Action: action}) {
		t.Errorf("%s rules = %+v, missing %q -> %q", tool, permission.Rules, pattern, action)
	}
}

func openCodePermissionFor(t *testing.T, policy openCodeAgentPolicy, tool string) openCodeToolPermission {
	t.Helper()
	for _, permission := range policy.Permissions {
		if permission.Tool == tool {
			return permission
		}
	}
	t.Fatalf("permission %q missing", tool)
	return openCodeToolPermission{}
}
