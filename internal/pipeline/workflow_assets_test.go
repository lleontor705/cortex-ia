package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestPrepareWorkflowUsesFreshQualifiedAdapterProfile(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir:        home,
		Adapters:       []agents.Adapter{codex.NewAdapter()},
		EvaluationTime: now,
		ModelRoutes:    testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plan.Profile != "portable-flat" {
		t.Fatalf("profile = %q, want strongest fresh adapter-qualified profile", prepared.Plan.Profile)
	}
}

func TestResolveWorkflowRoutesCompletesTransverseRolesFromQualifiedBootstrap(t *testing.T) {
	table, metadata, err := resolveWorkflowRoutes(context.Background(), WorkflowRequest{ModelRoutes: testModelRoutes()})
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []ir.SemanticID{"role/orchestrator", "role/debate", "role/parallel-dispatch"} {
		route, routeErr := table.ModelFor(role)
		if routeErr != nil {
			t.Fatalf("inherited route %q: %v", role, routeErr)
		}
		if route.Role != role || route.Primary.Model != "model-test" || len(route.Evidence) == 0 {
			t.Fatalf("inherited route %q is incomplete: %+v", role, route)
		}
		if metadata[string(role)].Role != role {
			t.Fatalf("metadata route %q was not attributed to the transverse role", role)
		}
	}
}

func TestPrepareWorkflowInstallsTwelveRoleCatalogForPrimaryTargets(t *testing.T) {
	for _, adapter := range []agents.Adapter{opencode.NewAdapter(), claude.NewAdapter(), codex.NewAdapter()} {
		prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
			HomeDir:          t.TempDir(),
			Adapters:         []agents.Adapter{adapter},
			RequestedProfile: sdd.ProfilePortableSequential,
			EvaluationTime:   time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC),
			ModelRoutes:      testModelRoutes(),
		})
		if err != nil {
			t.Fatal(err)
		}
		bundle := prepared.Bundles[0]
		inventory := make(map[ir.SemanticID]bool, len(bundle.Bundle.Assets))
		for _, asset := range bundle.Bundle.Assets {
			inventory[asset.SemanticID] = true
			if bundle.Target == "opencode" && asset.SemanticID == "asset/opencode/agent/orchestrator" {
				content := string(asset.Content)
				for _, marker := range []string{"mode: primary", "model: provider-test/model-test", "native `skill` tool", `"*": deny`, "orchestrator: allow", "task: deny"} {
					if !strings.Contains(content, marker) {
						t.Errorf("OpenCode orchestrator missing %q:\n%s", marker, content)
					}
				}
			}
		}
		for _, name := range []string{"orchestrator", "debate", "parallel-dispatch"} {
			for _, id := range []ir.SemanticID{
				ir.SemanticID("asset/skill/" + name),
				ir.SemanticID("asset/role/" + name + "/binding"),
			} {
				if !inventory[id] {
					t.Errorf("target %q missing installed transverse asset %q", bundle.Target, id)
				}
			}
		}
	}
}

func TestPrepareWorkflowInstallsTenNativeOpenCodeCommands(t *testing.T) {
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir:          t.TempDir(),
		Adapters:         []agents.Adapter{opencode.NewAdapter()},
		RequestedProfile: sdd.ProfilePortableSequential,
		EvaluationTime:   time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC),
		ModelRoutes:      testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	commands := map[string]string{}
	for _, asset := range prepared.Bundles[0].Bundle.Assets {
		if asset.Kind == "command" {
			commands[asset.Path] = string(asset.Content)
		}
	}
	for _, name := range []string{"bootstrap", "investigate", "new-change", "continue", "fast-forward", "implement", "validate", "finalize", "debate", "monitor"} {
		path := ".config/opencode/commands/" + name + ".md"
		content, ok := commands[path]
		if !ok {
			t.Errorf("missing OpenCode command %q", path)
			continue
		}
		if !strings.Contains(content, "$ARGUMENTS") {
			t.Errorf("OpenCode command %q does not preserve user arguments", path)
		}
	}
	if len(commands) != 10 {
		t.Fatalf("OpenCode commands = %d, want exactly 10: %v", len(commands), commands)
	}
	if _, exists := commands[".config/opencode/commands/run-workflow.md"]; exists {
		t.Fatal("deprecated run-workflow command is still installed")
	}
}

func TestOpenCodeGeneratedAgentsPassRuntimeConfigQualification(t *testing.T) {
	binary := os.Getenv("CORTEX_IA_OPENCODE_QUALIFICATION_BIN")
	if binary == "" {
		t.Skip("set CORTEX_IA_OPENCODE_QUALIFICATION_BIN to run the isolated OpenCode qualification")
	}
	home := t.TempDir()
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir: home, Adapters: []agents.Adapter{opencode.NewAdapter()},
		RequestedProfile: sdd.ProfilePortableFlat,
		EvaluationTime:   time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC),
		ModelRoutes:      testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Apply(); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(home, "AppData", "Local")
	roaming := filepath.Join(home, "AppData", "Roaming")
	config := filepath.Join(home, ".config", "opencode")
	for _, directory := range []string{local, roaming, config} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(binary, "debug", "config")
	command.Dir = home
	command.Env = append(os.Environ(),
		"HOME="+home, "USERPROFILE="+home, "LOCALAPPDATA="+local, "APPDATA="+roaming,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "OPENCODE_CONFIG_DIR="+config,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("opencode debug config: %v\n%s", err, output)
	}
	var resolved map[string]any
	if err := json.Unmarshal(output, &resolved); err != nil {
		t.Fatalf("decode opencode debug config: %v\n%s", err, output)
	}
	agentMap, ok := resolved["agent"].(map[string]any)
	if !ok || len(agentMap) != 12 {
		t.Fatalf("qualified OpenCode agent catalog = %#v, want 12 agents", resolved["agent"])
	}
	orchestrator, ok := agentMap["orchestrator"].(map[string]any)
	if !ok || orchestrator["mode"] != "primary" {
		t.Fatalf("qualified orchestrator = %#v, want primary", agentMap["orchestrator"])
	}
	permissions, ok := orchestrator["permission"].(map[string]any)
	taskRules, rulesOK := permissions["task"].(map[string]any)
	if !ok || !rulesOK || len(taskRules) != 12 || taskRules["*"] != "deny" || taskRules["implement"] != "allow" {
		t.Fatalf("qualified orchestrator task allowlist = %#v", permissions["task"])
	}
	commandMap, ok := resolved["command"].(map[string]any)
	if !ok || len(commandMap) != 10 {
		t.Fatalf("qualified OpenCode command catalog = %#v, want 10 commands", resolved["command"])
	}
	implementCommand, ok := commandMap["implement"].(map[string]any)
	template, templateOK := implementCommand["template"].(string)
	if !ok || !templateOK || implementCommand["agent"] != "implement" || implementCommand["subtask"] != true || !strings.Contains(template, "$ARGUMENTS") {
		t.Fatalf("qualified implement command = %#v", commandMap["implement"])
	}
}

func TestPrepareWorkflowDegradesWhenAdapterProfileRouteIsNotQualified(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	provider := claude.NewAdapter()
	decision := ResolveProfileDecision(ProfileResolutionInput{
		Requested: sdd.ProfileNativeAdvanced, Facts: provider.CapabilityFacts(), Now: now,
	})
	if decision.Disposition != ProfileDispositionDegraded {
		t.Fatalf("unqualified adapter route must degrade explicitly: %+v", decision)
	}
}

func TestWorkflowMetadataRoundTripsSentinels(t *testing.T) {
	metadata := WorkflowMetadata{
		ContractFingerprint: "contract-sentinel",
		PrimaryModel:        "primary-sentinel",
		FallbackModel:       "fallback-sentinel",
		QualityPlanID:       "quality-sentinel",
		ProfileReasonID:     "profile-qualified",
		TrustEvidence:       []string{"evidence-sentinel"},
		Permissions:         []string{"permission/read"},
		HumanGate:           "gate-required",
		Observability:       "trace-sentinel",
	}
	if got := metadata.Clone(); got.ContractFingerprint != metadata.ContractFingerprint || got.PrimaryModel != metadata.PrimaryModel || got.FallbackModel != metadata.FallbackModel || got.QualityPlanID != metadata.QualityPlanID || got.ProfileReasonID != metadata.ProfileReasonID || got.HumanGate != metadata.HumanGate || got.Observability != metadata.Observability {
		t.Fatalf("metadata clone lost sentinel fields: %+v", got)
	}
}

func TestPreparedWorkflowMetadataSurvivesPlanAndReceipt(t *testing.T) {
	home := t.TempDir()
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir: home, Adapters: []agents.Adapter{codex.NewAdapter()},
		GeneratorVersion: "test-v1", EvaluationTime: time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC),
		ModelRoutes: testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var planned WorkflowMetadata
	if err := json.Unmarshal(prepared.Plan.Metadata, &planned); err != nil {
		t.Fatal(err)
	}
	if planned.ProfileReasonID == "" || len(planned.TrustEvidence) == 0 {
		t.Fatalf("plan metadata incomplete: %+v", planned)
	}
	if planned.PrimaryModel != "" || planned.FallbackModel != "" {
		t.Fatalf("profile metadata must not imply model routing: %+v", planned)
	}
	receipt, err := prepared.Apply()
	if err != nil {
		t.Fatal(err)
	}
	var applied WorkflowMetadata
	if err := json.Unmarshal(receipt.Metadata, &applied); err != nil {
		t.Fatal(err)
	}
	if applied.ContractFingerprint != planned.ContractFingerprint || applied.ProfileReasonID != planned.ProfileReasonID || applied.Permissions[0] != planned.Permissions[0] {
		t.Fatalf("metadata changed across apply: plan=%+v receipt=%+v", planned, applied)
	}
}

func testModelRoutes() prompt.ModelTable {
	roles := []ir.SemanticID{"role/bootstrap", "role/investigate", "role/draft-proposal", "role/write-specs", "role/architect", "role/decompose", "role/implement", "role/validate", "role/finalize"}
	routes := make([]prompt.ModelRoute, 0, len(roles))
	for _, role := range roles {
		route, _ := modelroute.NewRouteID("route/v1/test")
		routes = append(routes, modelroute.ResolvedRoute{Role: role, Requested: modelroute.RouteRequest{RouteID: route}, PrimaryID: route, Primary: modelroute.RouteRef{Provider: "provider-test", Model: "model-test"}, Evidence: []modelroute.ResolutionEvidence{{ID: "evidence-" + string(role), Source: modelroute.SourceProviderConfig, Provider: "provider-test", Route: route, ObservedAt: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC), FreshUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Digest: "digest-test", Qualified: true}}})
	}
	return prompt.ModelTable{Routes: routes}
}
