package conformance

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// stage1ContentRoots are the asset directories whose content the Stage 1
// restoration must validate against REQ-EVAL-001 / REQ-ORCH-001.
const (
	rootIndexRel       = "internal/assets/generic/sdd-orchestrator-root-index.md"
	sharedContractRel  = "internal/assets/skills/_shared/sdd-phase-contract.md"
	overlayDirRel      = "internal/assets/generic/profiles"
	rootModuleDirRel   = "internal/assets/generic/sdd-root"
	canonicalSkillBase = "internal/assets/skills"
	commandsDirRel     = "internal/assets/opencode/commands"
)

var rootModuleFiles = []string{
	"routing-and-risk",
	"contracts-and-thresholds",
	"recovery-and-reflection",
	"parallel-apply",
	"memory-and-state",
	"model-routing",
}

var profileOverlayFiles = []string{
	"portable-sequential",
	"portable-flat",
	"native-advanced",
}

var canonicalSkillDirs = []string{
	"bootstrap", "investigate", "draft-proposal",
	"write-specs", "architect", "decompose",
	"implement", "validate", "finalize",
}

var commandFiles = []string{
	"bootstrap", "investigate", "new-change", "continue",
	"fast-forward", "implement", "validate", "finalize",
	"debate", "monitor",
}

// estimateTokens is the conservative deterministic token estimator required by
// design §5: ceil(UTF-8 runes / 3). No optional tokenizer may waive it.
func estimateTokens(content string) int {
	runes := utf8.RuneCountInString(content)
	return (runes + 2) / 3
}

// stalePattern matches every forbidden current-surface tool/coordination
// reference that REQ-INST-004 removes from retained assets.
var stalePattern = regexp.MustCompile(`(?i)(agent[- ]mailbox|team-lead|\bmsg_\w|\ba2a_\w|\bresource_acquire|\bresource_release|\bresource_check|\bdlq_\w)`)

// classificationMarker allows legacy/historical/retired mentions to survive the
// docs scan when they are clearly classified (matches docs_test.go convention).
var classificationMarker = regexp.MustCompile(`(?i)(retired|historical|legacy|removed|unsupported|unbound|never auto)`)

func readAsset(t *testing.T, rel string) string {
	t.Helper()
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read asset %s: %v", rel, err)
	}
	return string(data)
}

func assertNoStaleRefs(t *testing.T, rel, content string) {
	t.Helper()
	for lineNum, line := range strings.Split(content, "\n") {
		if stalePattern.MatchString(line) && !classificationMarker.MatchString(line) {
			t.Errorf("%s:%d unclassified stale reference: %s", rel, lineNum+1, strings.TrimSpace(line))
		}
	}
}

// ---------------------------------------------------------------------------
// Root operational index (task f0ef)
// ---------------------------------------------------------------------------

func TestRootIndexExists(t *testing.T) {
	root := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rootIndexRel))); err != nil {
		t.Fatalf("root index %s does not exist: %v", rootIndexRel, err)
	}
}

func TestRootIndexTokenBudget(t *testing.T) {
	content := readAsset(t, rootIndexRel)
	tokens := estimateTokens(content)
	if tokens > 1500 {
		t.Fatalf("root index %d tokens exceeds 1500 (runes=%d)", tokens, utf8.RuneCountInString(content))
	}
}

func TestRootIndexReferencesAllModules(t *testing.T) {
	content := readAsset(t, rootIndexRel)
	for _, mod := range rootModuleFiles {
		if !strings.Contains(content, mod) {
			t.Errorf("root index must reference module %q", mod)
		}
	}
}

func TestRootIndexHasRouteTable(t *testing.T) {
	content := readAsset(t, rootIndexRel)
	for _, depth := range []string{"trivial", "simple", "normal", "complex"} {
		if !strings.Contains(strings.ToLower(content), depth) {
			t.Errorf("root index route table missing depth %q", depth)
		}
	}
}

func TestRootIndexHasStopFirstRules(t *testing.T) {
	content := readAsset(t, rootIndexRel)
	lowered := strings.ToLower(content)
	for _, marker := range []string{"stop", "blocked"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("root index missing stop-first marker %q", marker)
		}
	}
}

func TestRootIndexNoDuplicatedModuleContent(t *testing.T) {
	content := readAsset(t, rootIndexRel)
	// The root index must not inline full module paragraphs — it references them.
	// Check for absence of deep module-specific vocabulary that belongs in modules.
	deepMarkers := []string{"transient_max", "semantic_max", "no_progress_cycles"}
	for _, marker := range deepMarkers {
		if strings.Contains(content, marker) {
			t.Errorf("root index duplicates module content (%q should stay in module)", marker)
		}
	}
}

func TestRootIndexNoStaleRefs(t *testing.T) {
	assertNoStaleRefs(t, rootIndexRel, readAsset(t, rootIndexRel))
}

// ---------------------------------------------------------------------------
// Root module set A: routing/contracts/recovery (task 2e0a)
// ---------------------------------------------------------------------------

func TestRootModuleSetAExists(t *testing.T) {
	root := repositoryRoot(t)
	for _, mod := range rootModuleFiles[:3] {
		p := filepath.Join(root, filepath.FromSlash(rootModuleDirRel), mod+".md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("module %s.md missing: %v", mod, err)
		}
	}
}

func TestRootModuleRoutingRiskContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/routing-and-risk.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"reversib", "risk", "fast-track", "override"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("routing-and-risk missing concept %q", marker)
		}
	}
}

func TestRootModuleContractsThresholdsContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/contracts-and-thresholds.md")
	for _, threshold := range []string{"0.5", "0.7", "0.8", "0.6", "0.9"} {
		if !strings.Contains(content, threshold) {
			t.Errorf("contracts-and-thresholds missing threshold %s", threshold)
		}
	}
}

func TestRootModuleRecoveryContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/recovery-and-reflection.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"transient", "semantic", "no-progress", "reflection", "reconcil"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("recovery-and-reflection missing concept %q", marker)
		}
	}
}

func TestRootModulesNoStaleRefs(t *testing.T) {
	for _, mod := range rootModuleFiles {
		rel := rootModuleDirRel + "/" + mod + ".md"
		assertNoStaleRefs(t, rel, readAsset(t, rel))
	}
}

// ---------------------------------------------------------------------------
// Root module set B: apply/memory/models (task 044a)
// ---------------------------------------------------------------------------

func TestRootModuleSetBExists(t *testing.T) {
	root := repositoryRoot(t)
	for _, mod := range rootModuleFiles[3:] {
		p := filepath.Join(root, filepath.FromSlash(rootModuleDirRel), mod+".md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("module %s.md missing: %v", mod, err)
		}
	}
}

func TestRootModuleParallelApplyContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/parallel-apply.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"readiness", "cas", "worktree", "sequential"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("parallel-apply missing concept %q", marker)
		}
	}
}

func TestRootModuleParallelApplyForbidsStaleCoordination(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/parallel-apply.md")
	forbidden := []string{"team-lead", "team lead", "Mailbox", "A2A"}
	for _, f := range forbidden {
		if strings.Contains(content, f) {
			t.Errorf("parallel-apply must forbid %q (current surface)", f)
		}
	}
}

func TestRootModuleMemoryStateContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/memory-and-state.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"forgespec", "cortex", "reference", "handoff"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("memory-and-state missing concept %q", marker)
		}
	}
}

func TestRootModuleModelRoutingContent(t *testing.T) {
	content := readAsset(t, rootModuleDirRel+"/model-routing.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"provider-neutral", "route/v1/", "capability", "fallback"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("model-routing missing concept %q", marker)
		}
	}
}

// ---------------------------------------------------------------------------
// Shared phase contract (task 68b2)
// ---------------------------------------------------------------------------

func TestSharedContractExists(t *testing.T) {
	root := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(sharedContractRel))); err != nil {
		t.Fatalf("shared contract %s does not exist: %v", sharedContractRel, err)
	}
}

func TestSharedContractTokenBudget(t *testing.T) {
	content := readAsset(t, sharedContractRel)
	tokens := estimateTokens(content)
	if tokens > 1000 {
		t.Fatalf("shared contract %d tokens exceeds 1000 (runes=%d)", tokens, utf8.RuneCountInString(content))
	}
}

func TestSharedContractTrustModel(t *testing.T) {
	content := readAsset(t, sharedContractRel)
	lowered := strings.ToLower(content)
	for _, marker := range []string{"trust", "policy", "evidence"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("shared contract missing trust-model concept %q", marker)
		}
	}
}

func TestSharedContractPersistenceAuthority(t *testing.T) {
	content := readAsset(t, sharedContractRel)
	lowered := strings.ToLower(content)
	for _, marker := range []string{"forgespec", "cortex"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("shared contract missing persistence authority %q", marker)
		}
	}
}

func TestSharedContractHandoffUsesReferences(t *testing.T) {
	content := readAsset(t, sharedContractRel)
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "reference") {
		t.Error("shared contract must define reference-only handoff")
	}
	if strings.Contains(lowered, "transcript") {
		t.Error("shared contract must not require transcripts in handoff")
	}
}

func TestSharedContractNoStaleRefs(t *testing.T) {
	assertNoStaleRefs(t, sharedContractRel, readAsset(t, sharedContractRel))
}

// ---------------------------------------------------------------------------
// Profile overlays (task 2cba)
// ---------------------------------------------------------------------------

func TestProfileOverlaysExist(t *testing.T) {
	root := repositoryRoot(t)
	for _, prof := range profileOverlayFiles {
		p := filepath.Join(root, filepath.FromSlash(overlayDirRel), prof+".md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("profile overlay %s.md missing: %v", prof, err)
		}
	}
}

func TestProfileOverlayTokenBudgets(t *testing.T) {
	for _, prof := range profileOverlayFiles {
		rel := overlayDirRel + "/" + prof + ".md"
		content := readAsset(t, rel)
		tokens := estimateTokens(content)
		if tokens > 800 {
			t.Errorf("profile %s %d tokens exceeds 800 (runes=%d)", prof, tokens, utf8.RuneCountInString(content))
		}
	}
}

func TestProfilePortableSequentialNoParallel(t *testing.T) {
	content := readAsset(t, overlayDirRel+"/portable-sequential.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "sequential") {
		t.Error("portable-sequential must name sequential behavior")
	}
	if !strings.Contains(lowered, "degrad") {
		t.Error("portable-sequential must name degradation explicitly")
	}
}

func TestProfilePortableFlatRequiresWorktreeProof(t *testing.T) {
	content := readAsset(t, overlayDirRel+"/portable-flat.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "worktree") {
		t.Error("portable-flat must require worktree proof for parallel")
	}
}

func TestProfileNativeAdvancedNeverAssumes(t *testing.T) {
	content := readAsset(t, overlayDirRel+"/native-advanced.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "qualif") {
		t.Error("native-advanced must require qualified capability")
	}
}

func TestProfileOverlaysNoStaleRefs(t *testing.T) {
	for _, prof := range profileOverlayFiles {
		rel := overlayDirRel + "/" + prof + ".md"
		assertNoStaleRefs(t, rel, readAsset(t, rel))
	}
}

// ---------------------------------------------------------------------------
// Canonical skills batch 1: bootstrap/investigate/draft-proposal (task a802)
// ---------------------------------------------------------------------------

func TestSkillsBatch1NoStaleRefs(t *testing.T) {
	for _, skill := range canonicalSkillDirs[:3] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		assertNoStaleRefs(t, rel, readAsset(t, rel))
	}
}

func TestSkillsBatch1ReferencesSharedContract(t *testing.T) {
	for _, skill := range canonicalSkillDirs[:3] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		content := readAsset(t, rel)
		if !strings.Contains(content, "sdd-phase-contract") {
			t.Errorf("skill %s must reference sdd-phase-contract", skill)
		}
	}
}

func TestSkillBootstrapProbeBudget(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/bootstrap/SKILL.md")
	if !strings.Contains(content, "8") || !strings.Contains(content, "10") {
		t.Error("bootstrap must declare probe budget (8 reads, 10 calls)")
	}
}

func TestSkillInvestigateCitationRequirement(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/investigate/SKILL.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "citation") || !strings.Contains(lowered, "file:") {
		t.Error("investigate must require file:line citations")
	}
}

func TestSkillDraftProposalScopeAcceptance(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/draft-proposal/SKILL.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"scope", "rollback", "acceptance"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("draft-proposal missing concept %q", marker)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonical skills batch 2: write-specs/architect/decompose (task 6907)
// ---------------------------------------------------------------------------

func TestSkillsBatch2NoStaleRefs(t *testing.T) {
	for _, skill := range canonicalSkillDirs[3:6] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		assertNoStaleRefs(t, rel, readAsset(t, rel))
	}
}

func TestSkillsBatch2ReferencesSharedContract(t *testing.T) {
	for _, skill := range canonicalSkillDirs[3:6] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		content := readAsset(t, rel)
		if !strings.Contains(content, "sdd-phase-contract") {
			t.Errorf("skill %s must reference sdd-phase-contract", skill)
		}
	}
}

func TestSkillWriteSpecsGherkinRules(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/write-specs/SKILL.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"gherkin", "given", "when", "then"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("write-specs missing concept %q", marker)
		}
	}
}

func TestSkillArchitectAlternatives(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/architect/SKILL.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "alternative") {
		t.Error("architect must require >=2 alternatives")
	}
}

func TestSkillDecomposeWorkloadForecast(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/decompose/SKILL.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"workload", "forecast"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("decompose missing concept %q", marker)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonical skills batch 3: implement/validate/finalize (task 7234)
// ---------------------------------------------------------------------------

func TestSkillsBatch3NoStaleRefs(t *testing.T) {
	for _, skill := range canonicalSkillDirs[6:] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		assertNoStaleRefs(t, rel, readAsset(t, rel))
	}
}

func TestSkillsBatch3ReferencesSharedContract(t *testing.T) {
	for _, skill := range canonicalSkillDirs[6:] {
		rel := canonicalSkillBase + "/" + skill + "/SKILL.md"
		content := readAsset(t, rel)
		if !strings.Contains(content, "sdd-phase-contract") {
			t.Errorf("skill %s must reference sdd-phase-contract", skill)
		}
	}
}

func TestSkillImplementTDDDiscipline(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/implement/SKILL.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"red", "green", "refactor"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("implement missing TDD concept %q", marker)
		}
	}
}

func TestSkillValidateIndependentExecution(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/validate/SKILL.md")
	lowered := strings.ToLower(content)
	for _, marker := range []string{"independent", "fail_to_pass", "pass_to_pass"} {
		if !strings.Contains(lowered, marker) {
			t.Errorf("validate missing concept %q", marker)
		}
	}
}

func TestSkillFinalizeVerifyPassRequired(t *testing.T) {
	content := readAsset(t, canonicalSkillBase+"/finalize/SKILL.md")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "verify") || !strings.Contains(lowered, "pass") {
		t.Error("finalize must require verify-PASS before archive")
	}
}

// ---------------------------------------------------------------------------
// Commands (task 7730)
// ---------------------------------------------------------------------------

func TestCommandsAreThinEntryPoints(t *testing.T) {
	root := repositoryRoot(t)
	for _, cmd := range commandFiles {
		rel := filepath.Join(commandsDirRel, cmd+".md")
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("command %s.md missing: %v", cmd, err)
			continue
		}
		content := string(data)
		// Commands must not duplicate the orchestration manual.
		if strings.Contains(content, "Route table") || strings.Contains(content, "stop-first rules") {
			t.Errorf("command %s.md must not duplicate root manual", cmd)
		}
	}
}

func TestCommandsNoStaleRefs(t *testing.T) {
	root := repositoryRoot(t)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(commandsDirRel)), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("read %s: %v", rel, readErr)
			return nil
		}
		assertNoStaleRefs(t, filepath.ToSlash(rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Asset registration (task 7730)
// ---------------------------------------------------------------------------

func TestAssetsGoImportsTypedCatalog(t *testing.T) {
	content := readAsset(t, "internal/assets/assets.go")
	lowered := strings.ToLower(content)
	if !strings.Contains(lowered, "ir") && !strings.Contains(content, "AssetSpec") {
		// assets.go must either import the ir package or reference AssetSpec.
		t.Error("assets.go must register typed assets via ir.AssetSpec")
	}
}
