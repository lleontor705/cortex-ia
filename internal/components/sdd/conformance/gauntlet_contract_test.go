package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type gauntletDecisionCase struct {
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Facts   []string `json:"facts"`
	Flow    string   `json:"flow"`
	Reasons []string `json:"reasons"`
}

const (
	gauntletRootIndex = "internal/assets/generic/sdd-orchestrator-root-index.md"
	gauntletRoot      = "internal/assets/generic/sdd-root"
	mutationSkill     = "internal/assets/skills/mutation-testing/SKILL.md"
)

func loadGauntletCases(t *testing.T) []gauntletDecisionCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), "internal/components/sdd/conformance/testdata/gauntlet/decision_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []gauntletDecisionCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("gauntlet fixture is malformed: %v", err)
	}
	if len(cases) < 10 {
		t.Fatalf("gauntlet fixture has %d cases, want broad contract coverage", len(cases))
	}
	return cases
}

func gauntletAssets(t *testing.T) string {
	t.Helper()
	parts := []string{readAsset(t, gauntletRootIndex)}
	for _, name := range []string{"routing-and-risk", "contracts-and-thresholds", "recovery-and-reflection", "parallel-apply", "memory-and-state"} {
		parts = append(parts, readAsset(t, filepath.ToSlash(filepath.Join(gauntletRoot, name+".md"))))
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

func TestGauntletDecisionFixtureCoversSelectorPrecedenceAndReasons(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range loadGauntletCases(t) {
		if tc.Kind != "selector" {
			continue
		}
		seen[tc.Flow] = true
		if len(tc.Facts) == 0 || len(tc.Reasons) == 0 {
			t.Errorf("%s must include facts and observable reason codes", tc.Name)
		}
		if tc.Flow != "A" && tc.Flow != "B" && tc.Flow != "C" {
			t.Errorf("%s has invalid selector flow %q", tc.Name, tc.Flow)
		}
	}
	for _, flow := range []string{"A", "B", "C"} {
		if !seen[flow] {
			t.Errorf("selector fixture missing flow %s", flow)
		}
	}
	assets := gauntletAssets(t)
	for _, marker := range []string{"c > b > a", "read_only", "no_side_effect", "evidence_missing", "conflicting_facts", "reason code"} {
		if !strings.Contains(assets, marker) {
			t.Errorf("gauntlet guidance missing selector marker %q", marker)
		}
	}
}

func TestGauntletFiveStageOverlayPreservesNinePhases(t *testing.T) {
	assets := gauntletAssets(t)
	for _, stage := range []string{"constitutional preflight", "requirement quality", "blueprint handoff", "deterministic gates", "bounded remediation"} {
		if !strings.Contains(assets, stage) {
			t.Errorf("overlay missing stage %q", stage)
		}
	}
	for _, phase := range []string{"init", "explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"} {
		if !strings.Contains(assets, phase) {
			t.Errorf("overlay does not preserve phase %q", phase)
		}
	}
	for _, forbidden := range []string{"tenth phase", "new workflowir phase", "ears parser"} {
		if !strings.Contains(assets, forbidden) {
			t.Errorf("overlay must explicitly reject %q", forbidden)
		}
	}
}

func TestGauntletAuthorityAliasAndSequentialDefault(t *testing.T) {
	assets := gauntletAssets(t)
	for _, marker := range []string{"minion", "alias", "orchestrator alone", "reference-only", "sequential by default", "disjoint read-only"} {
		if !strings.Contains(assets, marker) {
			t.Errorf("authority guidance missing %q", marker)
		}
	}
}

func TestGauntletMutationContractAndNormalization(t *testing.T) {
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mutationSkill)))
	if err != nil {
		t.Fatalf("mutation-testing utility asset is missing: %v", err)
	}
	content := strings.ToLower(string(data))
	for _, marker := range []string{"non-phase", "utility authority", "execution-time", "probe", "manifest", "gomutants", "pass", "fail", "inconclusive", "blocked", "degraded", "deterministic", "does not edit production"} {
		if !strings.Contains(content, marker) {
			t.Errorf("mutation contract missing marker %q", marker)
		}
	}
	if strings.Contains(content, "install mutation") || strings.Contains(content, "bundle mutation") {
		t.Error("mutation contract must not install or bundle a product")
	}
	for _, tc := range loadGauntletCases(t) {
		if tc.Kind == "mutation" && len(tc.Facts) == 0 {
			t.Errorf("mutation case %s has no probe/normalization facts", tc.Name)
		}
	}
}

func TestGauntletRemediationDeliveryAndRollbackContract(t *testing.T) {
	assets := gauntletAssets(t)
	for _, marker := range []string{"tests, fixtures, and assertions", "at most two", "no production", "skill", "prompt-equivalent", "documented degradation", "native", "sequential", "prepare", "apply", "rollback", "idempotent", "unmanaged"} {
		if !strings.Contains(assets, marker) {
			t.Errorf("gauntlet delivery/remediation guidance missing %q", marker)
		}
	}
}

func TestGauntletRejectsForbiddenArchitectureAndConditionalAttribution(t *testing.T) {
	assets := gauntletAssets(t)
	for _, marker := range []string{"no new role", "no adapter api", "no scheduler", "no permission expansion", "no runtime orchestration"} {
		if !strings.Contains(assets, marker) {
			t.Errorf("guidance missing forbidden-architecture marker %q", marker)
		}
	}
	if strings.Contains(assets, "superpowers") {
		if !strings.Contains(assets, "mit") {
			t.Error("adapted Superpowers material must retain MIT attribution")
		}
		if !regexp.MustCompile(`[0-9a-f]{40}`).MatchString(assets) {
			t.Error("adapted Superpowers material must cite an immutable revision")
		}
	}
	for _, tc := range loadGauntletCases(t) {
		if tc.Kind == "provenance" && len(tc.Reasons) == 0 {
			t.Errorf("provenance case %s must record conditional attribution reason", tc.Name)
		}
	}
}
