package registry

// WU-08 end-to-end Resolve oracles for the declarative-skill-registry-
// foundation slice. Every test is named after the spec oracle it implements
// (spec sdd-de4191a255e941d59ada39b6a7510011) and drives the single registry
// ingress orchestrator, Resolve, against the real embedded baseline catalog:
// verified local provenance, additive merge, protected selection,
// deterministic canonical evidence, and observable deterministic diagnostics.
//
// Filesystem fixtures live exclusively in deterministic temporary
// directories. Subtests that need symlinks skip only when symlink creation
// is unavailable (Windows without SeCreateSymbolicLinkPrivilege); every
// other containment case is exercised with real directories and lexical
// traversal spellings.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// --- fixtures -------------------------------------------------------------

// embeddedBaseline materializes the real embedded operational catalog. Every
// oracle resolves against it so override and baseline assertions observe the
// genuine canonical asset set, never a hand-built substitute.
func embeddedBaseline(t *testing.T) assets.MaterializedCatalog {
	t.Helper()
	embedded, err := assets.BuildOperationalCatalog()
	if err != nil {
		t.Fatalf("build embedded baseline catalog: %v", err)
	}
	return embedded
}

// testPolicy is a disable-policy fixture that classifies one component of
// every class so the oracles can observe each protection category and the
// optional selection surface. It mirrors design D4: authority, workflow, and
// retained-dependency components are protected; only explicit entries are
// optional; anything unclassified is protected fail-closed.
func testPolicy() Policy {
	return Policy{
		SchemaVersion: "1.0.0",
		PolicyVersion: "test-policy-1",
		ComponentClasses: map[model.ComponentID]DisableClass{
			model.ComponentCortex:    ProtectedAuthority,
			model.ComponentForgeSpec: ProtectedAuthority,
			model.ComponentSDD:       ProtectedWorkflow,
			"retained-dep":           ProtectedRequired,
			model.ComponentSkills:    Optional,
			model.ComponentContext7:  Optional,
		},
	}
}

// skillDoc renders a minimal valid custom SKILL.md whose frontmatter
// declares the given identity. Line endings are LF so the raw bytes equal
// the canonicalized bytes and digests are stable on every platform.
func skillDoc(name, body string) []byte {
	return []byte(fmt.Sprintf("---\nname: %s\n---\n\n# %s\n\n%s\n", name, name, body))
}

// writeOverlayConfig writes a regular configuration file that anchors local
// provenance at the temporary root. Resolve only verifies the file as a
// canonical local source; it never parses its content.
func writeOverlayConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "cortex-ia.yaml")
	if err := os.WriteFile(path, []byte("preset: full\n"), 0o644); err != nil {
		t.Fatalf("write overlay config file: %v", err)
	}
	return path
}

// writeRawSkill creates a custom skill directory carrying the exact bytes
// given and returns its absolute path for declaration.
func writeRawSkill(t *testing.T, root, dir string, doc []byte) string {
	t.Helper()
	skillDir := filepath.Join(root, "skills", dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create custom skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, SkillMDName), doc, 0o644); err != nil {
		t.Fatalf("write custom skill %s: %v", SkillMDName, err)
	}
	return skillDir
}

// writeSkillDir creates a custom skill directory whose SKILL.md frontmatter
// declares name; the directory name and the declared identity may differ so
// collision and override fixtures can be built explicitly.
func writeSkillDir(t *testing.T, root, dir, name, body string) string {
	t.Helper()
	return writeRawSkill(t, root, dir, skillDoc(name, body))
}

// newRequest assembles one registry request from the declaring config file,
// declared custom skill paths, and any disabled component selections.
func newRequest(configFile string, paths []string, disabled ...model.ComponentID) Request {
	return Request{Selection: model.RegistrySelection{
		ConfigFile:         configFile,
		CustomSkillPaths:   paths,
		DisabledComponents: disabled,
	}}
}

// newRetainedRequest assembles one registry request that also carries the
// pipeline handover of retained components (design D4): the resolved selected
// set with declared disables already removed. Oracles that observe receipt
// EffectiveComponents use this builder so the explicit selection, not the
// policy classification map, defines the expected effective evidence.
func newRetainedRequest(configFile string, retained []model.ComponentID, paths []string, disabled ...model.ComponentID) Request {
	req := newRequest(configFile, paths, disabled...)
	req.RetainedComponents = retained
	return req
}

// --- resolve helpers ------------------------------------------------------

// mustResolve resolves and fails the test on any diagnostic.
func mustResolve(t *testing.T, embedded assets.MaterializedCatalog, policy Policy, req Request) Resolved {
	t.Helper()
	resolved, diags := Resolve(context.Background(), req, embedded, policy)
	if len(diags) > 0 {
		t.Fatalf("Resolve reported %d diagnostics, want success; primary: %s", len(diags), diagnosticSummary(diags)[0])
	}
	return resolved
}

// requireRejection resolves, requires a pure pre-write rejection, and returns
// the report. A rejected resolve must be the zero value and canonically
// ordered so no partial result or unstable cause can leak to callers.
func requireRejection(t *testing.T, embedded assets.MaterializedCatalog, policy Policy, req Request) Diagnostics {
	t.Helper()
	resolved, diags := Resolve(context.Background(), req, embedded, policy)
	if len(diags) == 0 {
		t.Fatal("Resolve succeeded, want a pre-write rejection")
	}
	requireZeroResolved(t, resolved)
	if !slices.IsSortedFunc(diags, compareDiagnostics) {
		t.Fatalf("rejection report is not canonically ordered: %v", diagnosticSummary(diags))
	}
	return diags
}

// requireZeroResolved asserts a rejection carried no partial resolution.
func requireZeroResolved(t *testing.T, resolved Resolved) {
	t.Helper()
	if len(resolved.Catalog.Catalog.Assets) != 0 || len(resolved.EffectiveSkills) != 0 ||
		resolved.CanonicalReceipt.Fingerprint != "" || len(resolved.Provenance) != 0 {
		t.Fatal("rejected resolve returned a partial resolution; a failure must be the zero value")
	}
}

// --- assertion helpers ------------------------------------------------------

// snapshotTree records every regular file under root as relative path to
// content digest so oracles can prove a pure resolve performed zero writes.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		snapshot[filepath.ToSlash(rel)] = ir.FingerprintContent(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree under %s: %v", root, err)
	}
	return snapshot
}

// requireTreeUnchanged proves the registry boundary never mutated the tree.
func requireTreeUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("filesystem under %s changed during a pure resolve:\nbefore: %v\nafter:  %v", root, before, after)
	}
}

// embeddedSkillIDs lists the skill-class asset IDs of a materialized catalog.
func embeddedSkillIDs(catalog assets.MaterializedCatalog) []string {
	ids := make([]string, 0, len(catalog.Catalog.Assets))
	for _, spec := range catalog.Catalog.Assets {
		if spec.Class != ir.AssetSkill || !strings.HasPrefix(string(spec.ID), skillAssetIDPrefix) {
			continue
		}
		if id := strings.TrimPrefix(string(spec.ID), skillAssetIDPrefix); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// findSkillByID returns the effective skill record for id.
func findSkillByID(skills []Skill, id model.SkillID) (Skill, bool) {
	for _, skill := range skills {
		if skill.ID == id {
			return skill, true
		}
	}
	return Skill{}, false
}

// skillIDSequence projects effective skills onto their ordered ID sequence.
func skillIDSequence(skills []Skill) []string {
	ids := make([]string, len(skills))
	for i, skill := range skills {
		ids[i] = string(skill.ID)
	}
	return ids
}

// sortedProvenance orders evidence records by source path so order-dependent
// declaration sequences can be compared by content.
func sortedProvenance(resolved Resolved) []Evidence {
	ordered := slices.Clone(resolved.Provenance)
	slices.SortFunc(ordered, func(a, b Evidence) int {
		return strings.Compare(a.ConfigRelativePath, b.ConfigRelativePath)
	})
	return ordered
}

// diagnosticSummary renders a report as stable one-line facts so repeated
// validations can be compared for determinism.
func diagnosticSummary(diags Diagnostics) []string {
	summary := make([]string, 0, len(diags))
	for _, diag := range diags {
		id := "<nil>"
		if diag.ID != nil {
			id = string(*diag.ID)
		}
		summary = append(summary, fmt.Sprintf("%s/%s/%s/id=%s/decl=%d", diag.Stage, diag.Class, diag.Rule, id, diag.DeclarationIndex))
	}
	return summary
}

// assertSingleDiagnostic requires exactly one diagnostic with the expected
// class, stage, and rule, carrying a cause.
func assertSingleDiagnostic(t *testing.T, diags Diagnostics, class ErrorClass, stage Stage, rule string) Diagnostic {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want exactly 1: %v", len(diags), diagnosticSummary(diags))
	}
	diag := diags[0]
	if diag.Class != class || diag.Stage != stage || diag.Rule != rule {
		t.Fatalf("diagnostic = %s/%s/%s, want %s/%s/%s", diag.Stage, diag.Class, diag.Rule, stage, class, rule)
	}
	if diag.Cause == nil {
		t.Fatalf("diagnostic %s carries no cause", diag.Rule)
	}
	return diag
}

// sortedMapKeys returns the sorted keys of a decoded JSON object.
func sortedMapKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// --- single-defect request builders shared by the diagnostics oracles -----

func untrustedRequest(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	return newRequest(configFile, []string{filepath.Join(root, "skills", "ghost")})
}

func invalidRequest(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	broken := writeRawSkill(t, root, "broken", []byte("# no frontmatter\n\njust markdown\n"))
	return newRequest(configFile, []string{broken})
}

func overrideRequest(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	evil := writeSkillDir(t, root, "evil", "bootstrap", "Hostile replacement body.")
	return newRequest(configFile, []string{evil})
}

func collisionRequest(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	one := writeSkillDir(t, root, "one", "clash", "First declaration body.")
	two := writeSkillDir(t, root, "two", "clash", "Second declaration body.")
	return newRequest(configFile, []string{one, two})
}

func protectedDisableRequest(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	return newRequest(configFile, nil, model.ComponentSDD)
}

// --- REQ-TRUST-001 ---------------------------------------------------------

// TestSpec_REQ_TRUST_001_LocalVerifiedAccepted implements SC-TRUST-H
// (AC-TRUST-1): a locally declared skill source with verifiable provenance is
// accepted as an additive custom skill, recorded as verified evidence, and
// grants no new permissions — the canonical evidence carries identity and
// content digests only.
func TestSpec_REQ_TRUST_001_LocalVerifiedAccepted(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	doc := skillDoc("alpha", "Verified local skill body.")
	skillDir := writeSkillDir(t, root, "alpha", "alpha", "Verified local skill body.")
	before := snapshotTree(t, root)

	embedded := embeddedBaseline(t)
	// The pipeline handover carries the retained selection (design D4) so
	// the receipt observes the explicit selected set, not the policy map.
	retained := []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec, model.ComponentSDD, model.ComponentSkills}
	resolved := mustResolve(t, embedded, testPolicy(), newRetainedRequest(configFile, retained, []string{skillDir}))

	// Provenance evidence covers the declaring config and the skill source.
	if len(resolved.Provenance) != 2 {
		t.Fatalf("provenance records %d entries, want 2 (config + skill source)", len(resolved.Provenance))
	}
	configEvidence, skillEvidence := resolved.Provenance[0], resolved.Provenance[1]
	if configEvidence.Kind != EvidenceConfigSource || !configEvidence.Verified {
		t.Fatalf("config evidence = %+v, want a verified config source", configEvidence)
	}
	if configEvidence.ConfigRelativePath != "cortex-ia.yaml" {
		t.Errorf("config evidence path = %q, want %q", configEvidence.ConfigRelativePath, "cortex-ia.yaml")
	}
	if got, want := configEvidence.ContentSHA256, ir.FingerprintContent([]byte("preset: full\n")); got != want {
		t.Errorf("config evidence digest = %s, want %s", got, want)
	}
	if skillEvidence.Kind != EvidenceSkillSource || !skillEvidence.Verified {
		t.Fatalf("skill evidence = %+v, want a verified skill source", skillEvidence)
	}
	if skillEvidence.ConfigRelativePath != "skills/alpha" {
		t.Errorf("skill evidence path = %q, want %q", skillEvidence.ConfigRelativePath, "skills/alpha")
	}
	if got, want := skillEvidence.ContentSHA256, ir.FingerprintContent(doc); got != want {
		t.Errorf("skill evidence digest = %s, want %s", got, want)
	}

	// The verified source is eligible as an additive custom skill carrying
	// identity and content only — never agents, tools, or permissions.
	skill, ok := findSkillByID(resolved.EffectiveSkills, "alpha")
	if !ok {
		t.Fatal("verified custom skill alpha missing from effective skills")
	}
	if skill.Origin != OriginCustom {
		t.Errorf("alpha origin = %v, want OriginCustom", skill.Origin)
	}
	if !bytes.Equal(skill.Content, doc) {
		t.Errorf("alpha content = %q, want the canonicalized declared bytes %q", skill.Content, doc)
	}
	if skill.ContentSHA256 != ir.FingerprintContent(doc) {
		t.Errorf("alpha digest = %s, want %s", skill.ContentSHA256, ir.FingerprintContent(doc))
	}

	// No new permissions: the sealed canonical receipt projects only stable
	// semantic evidence, with no tool, permission, agent, or binding keys.
	if err := ValidateReceipt(resolved.CanonicalReceipt); err != nil {
		t.Fatalf("canonical receipt is not sealed correctly: %v", err)
	}
	var projection map[string]any
	if err := json.Unmarshal(CanonicalReceiptJSON(resolved.CanonicalReceipt), &projection); err != nil {
		t.Fatalf("decode canonical receipt projection: %v", err)
	}
	wantKeys := []string{"baseline_digest", "effective_components", "effective_skills", "host_outputs", "policy_digest", "schema_version"}
	if got := sortedMapKeys(projection); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("receipt projection keys = %v, want exactly %v", got, wantKeys)
	}
	projectedSkills, _ := projection["effective_skills"].([]any)
	if len(projectedSkills) == 0 {
		t.Fatal("receipt projects no effective skills")
	}
	for _, entry := range projectedSkills {
		skillProjection, _ := entry.(map[string]any)
		if got := sortedMapKeys(skillProjection); !reflect.DeepEqual(got, []string{"content_sha256", "id", "origin"}) {
			t.Fatalf("effective skill projection keys = %v, want exactly [content_sha256 id origin]", got)
		}
	}

	// A verified local source never disables or selects anything implicitly.
	if len(resolved.Disabled) != 0 {
		t.Errorf("verified provenance implicitly disabled %v, want none", resolved.Disabled)
	}

	// The receipt lists exactly the retained selection handed over by
	// the pipeline: unselected policy-classified IDs such as context7 and
	// retained-dep never leak into the effective evidence (REQ-REM-B3).
	wantComponents := []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec, model.ComponentSDD, model.ComponentSkills}
	if got := resolved.CanonicalReceipt.EffectiveComponents; !reflect.DeepEqual(got, wantComponents) {
		t.Errorf("effective components = %v, want the retained selection %v", got, wantComponents)
	}

	requireTreeUnchanged(t, root, before)
}

// TestSpec_REQ_TRUST_001_UnverifiableLocalRejected implements SC-TRUST-E
// (AC-TRUST-2): a declared local path that cannot be verified from canonical
// filesystem facts is rejected untrusted before any write.
func TestSpec_REQ_TRUST_001_UnverifiableLocalRejected(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	t.Run("missing path", func(t *testing.T) {
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		before := snapshotTree(t, root)
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{filepath.Join(root, "skills", "ghost")}))
		diag := assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillProvenance)
		if diag.ID != nil {
			t.Errorf("unverifiable source invented skill ID %q", *diag.ID)
		}
		if diag.SafeRemediation == "" {
			t.Error("untrusted diagnostic omits its known-safe remediation")
		}
		requireTreeUnchanged(t, root, before)
	})

	t.Run("file declared instead of directory", func(t *testing.T) {
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		before := snapshotTree(t, root)
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{configFile}))
		assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillProvenance)
		requireTreeUnchanged(t, root, before)
	})
}

// TestSpec_REQ_TRUST_001_NonLocalRejected implements SC-TRUST-F (AC-TRUST-3):
// a source whose canonical origin is not local to the configuration root is
// rejected as untrusted even though it is syntactically valid: real outside
// directories, lexical traversal spellings, symlink escapes, and declarations
// without any verified local config anchor.
func TestSpec_REQ_TRUST_001_NonLocalRejected(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	t.Run("real directory outside the config root", func(t *testing.T) {
		outside := t.TempDir()
		outsideSkill := writeSkillDir(t, outside, "outsider", "outsider", "Syntactically valid but non-local.")
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		before := snapshotTree(t, root)
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{outsideSkill}))
		diag := assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillContainment)
		if diag.ID != nil {
			t.Errorf("containment rejection invented skill ID %q", *diag.ID)
		}
		requireTreeUnchanged(t, root, before)
	})

	t.Run("lexical traversal spelling", func(t *testing.T) {
		outside := t.TempDir()
		writeSkillDir(t, outside, "outsider", "outsider", "Syntactically valid but non-local.")
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		sep := string(filepath.Separator)
		declared := root + sep + "skills" + sep + ".." + sep + ".." + sep +
			filepath.Base(outside) + sep + "skills" + sep + "outsider"
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{declared}))
		assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillContainment)
	})

	t.Run("symlink escape", func(t *testing.T) {
		outside := t.TempDir()
		outsideSkill := writeSkillDir(t, outside, "outsider", "outsider", "Syntactically valid but non-local.")
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
			t.Fatalf("create skills directory: %v", err)
		}
		link := filepath.Join(root, "skills", "escape")
		if err := os.Symlink(outsideSkill, link); err != nil {
			t.Skipf("symlink creation unavailable (SeCreateSymbolicLinkPrivilege not held): %v", err)
		}
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{link}))
		assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillContainment)
	})

	t.Run("no verified config anchor", func(t *testing.T) {
		root := t.TempDir()
		skillDir := writeSkillDir(t, root, "orphan", "orphan", "Local directory but nothing anchors provenance.")
		diags := requireRejection(t, embedded, policy, newRequest("", []string{skillDir}))
		assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillProvenance)
	})
}

// --- REQ-REG-001 -----------------------------------------------------------

// TestSpec_REQ_REG_001_AddValidCustomSkill implements SC-REG1-H (AC-REG-1):
// a valid local custom skill with a new ID appears exactly once alongside the
// complete embedded baseline, with identity and content preserved.
func TestSpec_REQ_REG_001_AddValidCustomSkill(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	doc := skillDoc("alpha", "Domain-specific review guidance.")
	skillDir := writeSkillDir(t, root, "alpha", "alpha", "Domain-specific review guidance.")

	embedded := embeddedBaseline(t)
	policy := testPolicy()
	resolved := mustResolve(t, embedded, policy, newRequest(configFile, []string{skillDir}))

	if got := resolved.Catalog.Count("asset/skill/alpha"); got != 1 {
		t.Errorf("custom skill appears %d times, want exactly 1", got)
	}
	if len(resolved.Catalog.Catalog.Assets) != len(embedded.Catalog.Assets)+1 {
		t.Errorf("effective catalog has %d assets, want baseline %d plus the one addition",
			len(resolved.Catalog.Catalog.Assets), len(embedded.Catalog.Assets))
	}
	for _, spec := range embedded.Catalog.Assets {
		if got := resolved.Catalog.Count(string(spec.ID)); got != 1 {
			t.Errorf("baseline asset %q appears %d times, want 1", spec.ID, got)
		}
	}

	// The addition mirrors the embedded skill layout exactly.
	var alphaSpec *ir.AssetSpec
	for i := range resolved.Catalog.Catalog.Assets {
		if resolved.Catalog.Catalog.Assets[i].ID == ir.SemanticID("asset/skill/alpha") {
			alphaSpec = &resolved.Catalog.Catalog.Assets[i]
		}
	}
	if alphaSpec == nil {
		t.Fatal("custom skill asset spec missing from the effective catalog")
	}
	if alphaSpec.Class != ir.AssetSkill {
		t.Errorf("custom skill class = %q, want %q", alphaSpec.Class, ir.AssetSkill)
	}
	if !alphaSpec.Required {
		t.Error("custom skill must be a required asset so it is installed")
	}
	if alphaSpec.SourcePath != "skills/alpha/SKILL.md" {
		t.Errorf("custom skill source path = %q, want %q", alphaSpec.SourcePath, "skills/alpha/SKILL.md")
	}
	if alphaSpec.SHA256 != ir.FingerprintContent(doc) {
		t.Errorf("custom skill SHA256 = %q, want %q", alphaSpec.SHA256, ir.FingerprintContent(doc))
	}
	if alphaSpec.MaxTokens != customSkillMaxTokens {
		t.Errorf("custom skill MaxTokens = %d, want the embedded skill budget %d", alphaSpec.MaxTokens, customSkillMaxTokens)
	}

	// Effective skills keep the whole baseline and gain the addition once.
	embeddedIDs := embeddedSkillIDs(embedded)
	if len(resolved.EffectiveSkills) != len(embeddedIDs)+1 {
		t.Fatalf("effective skills = %d, want %d embedded plus alpha", len(resolved.EffectiveSkills), len(embeddedIDs))
	}
	for _, id := range embeddedIDs {
		if _, ok := findSkillByID(resolved.EffectiveSkills, model.SkillID(id)); !ok {
			t.Errorf("embedded skill %q missing from the effective set", id)
		}
	}
	alpha, _ := findSkillByID(resolved.EffectiveSkills, "alpha")
	if alpha.Origin != OriginCustom || !bytes.Equal(alpha.Content, doc) {
		t.Errorf("alpha = %+v, want a custom-origin skill with the declared bytes", alpha)
	}

	// The effective evidence differs from the no-overlay baseline.
	baselineOnly := mustResolve(t, embedded, policy, newRequest(configFile, nil))
	if resolved.CanonicalReceipt.Fingerprint == baselineOnly.CanonicalReceipt.Fingerprint {
		t.Error("adding a custom skill must change the canonical receipt fingerprint")
	}
	if resolved.Catalog.Fingerprint() == embedded.Fingerprint() {
		t.Error("adding a custom skill must change the effective catalog fingerprint")
	}
	if err := ValidateReceipt(resolved.CanonicalReceipt); err != nil {
		t.Fatalf("canonical receipt is not sealed correctly: %v", err)
	}
}

// TestSpec_REQ_REG_001_OrderIndependentNewSkills implements SC-REG1-E
// (AC-REG-2): unique custom skills declared in different orders produce the
// identical selection and evidence.
func TestSpec_REQ_REG_001_OrderIndependentNewSkills(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Alpha guidance.")
	betaDir := writeSkillDir(t, root, "beta", "beta", "Beta guidance.")

	embedded := embeddedBaseline(t)
	policy := testPolicy()
	forward := mustResolve(t, embedded, policy, newRequest(configFile, []string{alphaDir, betaDir}))
	reverse := mustResolve(t, embedded, policy, newRequest(configFile, []string{betaDir, alphaDir}))

	if !reflect.DeepEqual(skillIDSequence(forward.EffectiveSkills), skillIDSequence(reverse.EffectiveSkills)) {
		t.Fatalf("effective skill order depends on declaration order:\nforward: %v\nreverse: %v",
			skillIDSequence(forward.EffectiveSkills), skillIDSequence(reverse.EffectiveSkills))
	}
	if forward.CanonicalReceipt.EffectiveSkills.Fingerprint != reverse.CanonicalReceipt.EffectiveSkills.Fingerprint {
		t.Error("skill-set fingerprint depends on declaration order")
	}
	if forward.CanonicalReceipt.Fingerprint != reverse.CanonicalReceipt.Fingerprint {
		t.Error("receipt fingerprint depends on declaration order")
	}
	if !bytes.Equal(CanonicalReceiptJSON(forward.CanonicalReceipt), CanonicalReceiptJSON(reverse.CanonicalReceipt)) {
		t.Error("canonical receipt encoding depends on declaration order")
	}
	if forward.Catalog.Fingerprint() != reverse.Catalog.Fingerprint() {
		t.Error("effective catalog fingerprint depends on declaration order")
	}
	if !reflect.DeepEqual(sortedProvenance(forward), sortedProvenance(reverse)) {
		t.Errorf("per-source provenance evidence depends on declaration order:\nforward: %v\nreverse: %v",
			sortedProvenance(forward), sortedProvenance(reverse))
	}
}

// TestSpec_REQ_REG_001_InvalidCustomSkillRejected implements SC-REG1-F
// (AC-REG-3): an invalid custom skill declaration is rejected before any
// write with the violated rule and a cause.
func TestSpec_REQ_REG_001_InvalidCustomSkillRejected(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	cases := []struct {
		name string
		doc  []byte
		rule string
	}{
		{name: "missing frontmatter", doc: []byte("# no frontmatter\n\njust markdown\n"), rule: RuleFrontmatterName},
		{name: "unterminated frontmatter", doc: []byte("---\nname: alpha\n"), rule: RuleFrontmatterName},
		{name: "missing name field", doc: []byte("---\ndescription: no identity\n---\n\nbody\n"), rule: RuleSkillIDGrammar},
		{name: "empty name", doc: []byte("---\nname:\n---\n\nbody\n"), rule: RuleSkillIDGrammar},
		{name: "uppercase id", doc: skillDoc("Alpha", "Uppercase identity."), rule: RuleSkillIDGrammar},
		{name: "invalid utf-8", doc: append([]byte("---\nname: alpha\n---\n\n"), 0xff, 0xfe, '\n'), rule: RuleContentEncoding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			configFile := writeOverlayConfig(t, root)
			broken := writeRawSkill(t, root, "broken", tc.doc)
			before := snapshotTree(t, root)
			diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{broken}))
			diag := assertSingleDiagnostic(t, diags, ErrorInvalid, StageNormalize, tc.rule)
			if diag.ID != nil {
				t.Errorf("invalid declaration invented skill ID %q", *diag.ID)
			}
			if diag.DeclarationIndex != 0 {
				t.Errorf("declaration index = %d, want 0", diag.DeclarationIndex)
			}
			requireTreeUnchanged(t, root, before)
		})
	}

	t.Run("directory without SKILL.md", func(t *testing.T) {
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		skillDir := filepath.Join(root, "skills", "not-a-skill")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("create skill directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "README.txt"), []byte("not a skill\n"), 0o644); err != nil {
			t.Fatalf("write distractor file: %v", err)
		}
		before := snapshotTree(t, root)
		diags := requireRejection(t, embedded, policy, newRequest(configFile, []string{skillDir}))
		diag := assertSingleDiagnostic(t, diags, ErrorInvalid, StageLoad, RuleSkillSingleSkillMD)
		if diag.ID != nil {
			t.Errorf("structural rejection invented skill ID %q", *diag.ID)
		}
		requireTreeUnchanged(t, root, before)
	})
}

// --- REQ-REG-002 -----------------------------------------------------------

// TestSpec_REQ_REG_002_UniqueAdditionsAccepted implements SC-REG2-H
// (AC-COLL-1): custom IDs that are unique and not canonical compose
// additively onto the baseline without replacing anything.
func TestSpec_REQ_REG_002_UniqueAdditionsAccepted(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Alpha guidance.")
	betaDir := writeSkillDir(t, root, "beta", "beta", "Beta guidance.")
	gammaDir := writeSkillDir(t, root, "gamma", "gamma", "Gamma guidance.")

	embedded := embeddedBaseline(t)
	policy := testPolicy()
	resolved := mustResolve(t, embedded, policy, newRequest(configFile, []string{alphaDir, betaDir, gammaDir}))

	for _, id := range []string{"alpha", "beta", "gamma"} {
		if got := resolved.Catalog.Count("asset/skill/" + id); got != 1 {
			t.Errorf("custom skill %q appears %d times, want exactly 1", id, got)
		}
	}
	if len(resolved.Catalog.Catalog.Assets) != len(embedded.Catalog.Assets)+3 {
		t.Errorf("effective catalog has %d assets, want baseline %d plus the three additions",
			len(resolved.Catalog.Catalog.Assets), len(embedded.Catalog.Assets))
	}
	for _, spec := range embedded.Catalog.Assets {
		if got := resolved.Catalog.Count(string(spec.ID)); got != 1 {
			t.Errorf("baseline asset %q appears %d times, want 1 (additions must never replace)", spec.ID, got)
		}
	}

	// Additive composition is order-free: the same set in any declaration
	// order yields the identical effective catalog and evidence.
	reordered := mustResolve(t, embedded, policy, newRequest(configFile, []string{gammaDir, alphaDir, betaDir}))
	if resolved.Catalog.Fingerprint() != reordered.Catalog.Fingerprint() {
		t.Error("effective catalog depends on declaration order")
	}
	if resolved.CanonicalReceipt.Fingerprint != reordered.CanonicalReceipt.Fingerprint {
		t.Error("canonical receipt depends on declaration order")
	}
}

// TestSpec_REQ_REG_002_CustomCollisionRejected implements SC-REG2-E
// (AC-COLL-2): two effective custom declarations sharing one ID are rejected
// even when their content is identical.
func TestSpec_REQ_REG_002_CustomCollisionRejected(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	body := "Byte-for-byte identical declarations."
	one := writeSkillDir(t, root, "one", "clash", body)
	two := writeSkillDir(t, root, "two", "clash", body)
	before := snapshotTree(t, root)

	diags := requireRejection(t, embeddedBaseline(t), testPolicy(), newRequest(configFile, []string{one, two}))
	diag := assertSingleDiagnostic(t, diags, ErrorCollision, StageMerge, RuleCustomCollision)
	if diag.ID == nil || *diag.ID != model.SkillID("clash") {
		t.Errorf("collision diagnostic ID = %v, want clash", diag.ID)
	}
	if diag.DeclarationIndex != 1 {
		t.Errorf("collision declaration index = %d, want 1 (the duplicate declaration)", diag.DeclarationIndex)
	}
	if diag.SafeRemediation == "" {
		t.Error("collision diagnostic omits its known-safe remediation")
	}
	requireTreeUnchanged(t, root, before)
}

// TestSpec_REQ_REG_002_CanonicalOverrideRejected implements SC-REG2-F
// (AC-COLL-3): a custom ID matching an embedded canonical skill ID is
// rejected before any write and the canonical asset is preserved.
func TestSpec_REQ_REG_002_CanonicalOverrideRejected(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	evil := writeSkillDir(t, root, "evil", "bootstrap", "Hostile replacement body.")
	before := snapshotTree(t, root)

	embedded := embeddedBaseline(t)
	fingerprintBefore := embedded.Fingerprint()

	diags := requireRejection(t, embedded, testPolicy(), newRequest(configFile, []string{evil}))
	diag := assertSingleDiagnostic(t, diags, ErrorOverride, StageMerge, RuleCanonicalOverride)
	if diag.ID == nil || *diag.ID != model.SkillID("bootstrap") {
		t.Errorf("override diagnostic ID = %v, want bootstrap", diag.ID)
	}
	if diag.DeclarationIndex != 0 {
		t.Errorf("override declaration index = %d, want 0", diag.DeclarationIndex)
	}

	// The embedded canonical baseline is preserved byte-for-byte.
	if embedded.Fingerprint() != fingerprintBefore {
		t.Error("the rejected override mutated the embedded baseline catalog")
	}
	if got := embedded.Count("asset/skill/bootstrap"); got != 1 {
		t.Errorf("embedded bootstrap appears %d times after rejection, want 1", got)
	}
	// Removing the overlay restores a clean baseline resolve with the
	// canonical skill still present as an embedded fact.
	clean := mustResolve(t, embedded, testPolicy(), newRequest(configFile, nil))
	bootstrap, ok := findSkillByID(clean.EffectiveSkills, "bootstrap")
	if !ok {
		t.Fatal("canonical bootstrap skill missing after the overlay rejection")
	}
	if bootstrap.Origin != OriginEmbedded {
		t.Errorf("bootstrap origin = %v, want OriginEmbedded", bootstrap.Origin)
	}
	requireTreeUnchanged(t, root, before)
}

// --- REQ-SEL-001 -----------------------------------------------------------

// TestSpec_REQ_SEL_001_ProtectedDisableRejected implements SC-SEL-F
// (AC-SEL-3): disabling a protected component class — authority, workflow,
// retained dependency, or anything unclassified — is rejected before any
// write with the protection category identified.
func TestSpec_REQ_SEL_001_ProtectedDisableRejected(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	cases := []struct {
		name     string
		id       model.ComponentID
		category string
	}{
		{name: "authority component", id: model.ComponentCortex, category: "protected_authority"},
		{name: "workflow component", id: model.ComponentSDD, category: "protected_workflow"},
		{name: "retained dependency", id: "retained-dep", category: "protected_required"},
		{name: "unclassified component", id: "mystery-component", category: "protected-unclassified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			configFile := writeOverlayConfig(t, root)
			before := snapshotTree(t, root)
			diags := requireRejection(t, embedded, policy, newRequest(configFile, nil, tc.id))
			diag := assertSingleDiagnostic(t, diags, ErrorProtectedDisable, StageMerge, RuleProtectedDisable)
			if diag.Cause == nil || !strings.Contains(diag.Cause.Error(), tc.category) {
				t.Fatalf("cause %v does not identify the protection category %q", diag.Cause, tc.category)
			}
			if !strings.Contains(diag.Cause.Error(), string(tc.id)) {
				t.Errorf("cause %v does not name the protected component %q", diag.Cause, tc.id)
			}
			if diag.ID != nil {
				t.Errorf("component disable invented skill ID %q", *diag.ID)
			}
			requireTreeUnchanged(t, root, before)
		})
	}
}

// --- REQ-DET-001 -----------------------------------------------------------

// TestSpec_REQ_DET_001_IdenticalInputsStableEvidence implements SC-DET-H
// (AC-DET-1): identical effective inputs from independent clean runs produce
// identical canonical evidence regardless of the absolute location of the
// declared sources.
func TestSpec_REQ_DET_001_IdenticalInputsStableEvidence(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	build := func(t *testing.T) Resolved {
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Same effective bytes.")
		betaDir := writeSkillDir(t, root, "beta", "beta", "Same effective bytes.")
		return mustResolve(t, embedded, policy, newRequest(configFile, []string{alphaDir, betaDir}))
	}

	first := build(t)
	second := build(t)

	for _, receipt := range []Receipt{first.CanonicalReceipt, second.CanonicalReceipt} {
		if err := ValidateReceipt(receipt); err != nil {
			t.Fatalf("canonical receipt is not sealed correctly: %v", err)
		}
	}
	if first.CanonicalReceipt.Fingerprint != second.CanonicalReceipt.Fingerprint {
		t.Error("receipt fingerprint depends on the absolute location of identical inputs")
	}
	if !bytes.Equal(CanonicalReceiptJSON(first.CanonicalReceipt), CanonicalReceiptJSON(second.CanonicalReceipt)) {
		t.Error("canonical receipt encoding depends on the absolute location of identical inputs")
	}
	if first.CanonicalReceipt.EffectiveSkills.Fingerprint != second.CanonicalReceipt.EffectiveSkills.Fingerprint {
		t.Error("skill-set fingerprint depends on the absolute location of identical inputs")
	}
	if first.Catalog.Fingerprint() != second.Catalog.Fingerprint() {
		t.Error("effective catalog fingerprint depends on the absolute location of identical inputs")
	}
	if !reflect.DeepEqual(skillIDSequence(first.EffectiveSkills), skillIDSequence(second.EffectiveSkills)) {
		t.Error("effective skill selection depends on the absolute location of identical inputs")
	}
	if !reflect.DeepEqual(sortedProvenance(first), sortedProvenance(second)) {
		t.Error("provenance evidence depends on the absolute location of identical inputs")
	}
}

// TestSpec_REQ_DET_001_SecondRunNoRewrite implements SC-DET-E (AC-DET-2):
// once the declared state has converged, repeating the resolution produces
// identical evidence and rewrites nothing — the registry boundary is a pure
// observation of the effective input.
func TestSpec_REQ_DET_001_SecondRunNoRewrite(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Converged skill body.")
	before := snapshotTree(t, root)

	embedded := embeddedBaseline(t)
	policy := testPolicy()

	first := mustResolve(t, embedded, policy, newRequest(configFile, []string{alphaDir}))
	requireTreeUnchanged(t, root, before)

	second := mustResolve(t, embedded, policy, newRequest(configFile, []string{alphaDir}))
	requireTreeUnchanged(t, root, before)

	if second.CanonicalReceipt.Fingerprint != first.CanonicalReceipt.Fingerprint {
		t.Error("second run over converged state drifted from the first receipt")
	}
	if !bytes.Equal(CanonicalReceiptJSON(first.CanonicalReceipt), CanonicalReceiptJSON(second.CanonicalReceipt)) {
		t.Error("second run over converged state changed the canonical receipt encoding")
	}
	if second.Catalog.Fingerprint() != first.Catalog.Fingerprint() {
		t.Error("second run over converged state changed the effective catalog")
	}
	if !reflect.DeepEqual(sortedProvenance(second), sortedProvenance(first)) {
		t.Error("second run over converged state changed the provenance evidence")
	}
}

// TestSpec_REQ_DET_001_EffectiveChangeUpdatesEvidence implements SC-DET-F
// (AC-DET-3): any effective change — added skill, changed selection, revised
// policy, or changed baseline — updates the canonical fingerprint, and each
// digest field changes only for the input it fingerprints.
func TestSpec_REQ_DET_001_EffectiveChangeUpdatesEvidence(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Effective change probe.")

	embedded := embeddedBaseline(t)
	policy := testPolicy()

	// Every variation carries the same retained selection (design D4) so
	// each fingerprint delta isolates exactly one effective input change.
	retained := []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec, model.ComponentSDD, model.ComponentSkills, model.ComponentContext7}

	r0 := mustResolve(t, embedded, policy, newRetainedRequest(configFile, retained, nil))
	r1 := mustResolve(t, embedded, policy, newRetainedRequest(configFile, retained, []string{alphaDir}))

	// Effective skill-set change.
	if _, ok := findSkillByID(r0.EffectiveSkills, "alpha"); ok {
		t.Fatal("baseline resolve must not contain the custom skill")
	}
	if _, ok := findSkillByID(r1.EffectiveSkills, "alpha"); !ok {
		t.Fatal("overlay resolve lost the custom skill")
	}
	if r1.CanonicalReceipt.EffectiveSkills.Fingerprint == r0.CanonicalReceipt.EffectiveSkills.Fingerprint {
		t.Error("effective skill-set change must update the skill-set fingerprint")
	}
	if r1.CanonicalReceipt.Fingerprint == r0.CanonicalReceipt.Fingerprint {
		t.Error("effective skill-set change must update the receipt fingerprint")
	}

	// Selection change: disabling an optional component changes the effective
	// components and the receipt fingerprint but not the policy digest.
	r2 := mustResolve(t, embedded, policy, newRetainedRequest(configFile, retained, []string{alphaDir}, model.ComponentSkills))
	if r2.CanonicalReceipt.PolicyDigest != r1.CanonicalReceipt.PolicyDigest {
		t.Error("a selection change must not alter the policy digest")
	}
	if r2.CanonicalReceipt.Fingerprint == r1.CanonicalReceipt.Fingerprint {
		t.Error("effective selection change must update the receipt fingerprint")
	}
	if !slices.Contains(r1.CanonicalReceipt.EffectiveComponents, model.ComponentSkills) {
		t.Fatal("optional component skills missing from the undisabled selection")
	}
	if slices.Contains(r2.CanonicalReceipt.EffectiveComponents, model.ComponentSkills) {
		t.Error("disabled optional component still present in effective components")
	}
	if !slices.Contains(r2.Disabled, model.ComponentSkills) {
		t.Error("disabled optional component missing from Resolved.Disabled")
	}

	// Policy change: a revised policy changes the policy digest and the
	// receipt fingerprint with the selection unchanged.
	revised := testPolicy()
	revised.PolicyVersion = "test-policy-2"
	revised.ComponentClasses[model.ComponentContext7] = ProtectedRequired
	r3 := mustResolve(t, embedded, revised, newRetainedRequest(configFile, retained, []string{alphaDir}))
	if r3.CanonicalReceipt.PolicyDigest == r1.CanonicalReceipt.PolicyDigest {
		t.Error("policy revision must update the policy digest")
	}
	if r3.CanonicalReceipt.BaselineDigest != r1.CanonicalReceipt.BaselineDigest {
		t.Error("policy revision must not alter the baseline digest")
	}
	if r3.CanonicalReceipt.Fingerprint == r1.CanonicalReceipt.Fingerprint {
		t.Error("policy revision must update the receipt fingerprint")
	}

	// Baseline change: a different valid embedded baseline changes the
	// baseline digest and the receipt fingerprint with selection unchanged.
	alternate, err := assets.BuildEffectiveCatalog([]assets.CustomSkill{
		{ID: model.SkillID("baseline-extra"), Content: []byte("# baseline-extra\n\ncarried baseline addition.\n")},
	})
	if err != nil {
		t.Fatalf("build alternate baseline catalog: %v", err)
	}
	r4 := mustResolve(t, alternate, policy, newRetainedRequest(configFile, retained, []string{alphaDir}))
	if r4.CanonicalReceipt.BaselineDigest == r1.CanonicalReceipt.BaselineDigest {
		t.Error("baseline change must update the baseline digest")
	}
	if r4.CanonicalReceipt.PolicyDigest != r1.CanonicalReceipt.PolicyDigest {
		t.Error("baseline change must not alter the policy digest")
	}
	if r4.CanonicalReceipt.Fingerprint == r1.CanonicalReceipt.Fingerprint {
		t.Error("baseline change must update the receipt fingerprint")
	}

	// Every effective variation yields a pairwise distinct fingerprint.
	fingerprints := []string{
		r0.CanonicalReceipt.Fingerprint,
		r1.CanonicalReceipt.Fingerprint,
		r2.CanonicalReceipt.Fingerprint,
		r3.CanonicalReceipt.Fingerprint,
		r4.CanonicalReceipt.Fingerprint,
	}
	seen := make(map[string]struct{}, len(fingerprints))
	for _, fingerprint := range fingerprints {
		if _, duplicate := seen[fingerprint]; duplicate {
			t.Errorf("receipt fingerprints are not pairwise distinct: %s", fingerprint)
		}
		seen[fingerprint] = struct{}{}
	}
}

// --- REQ-BASE-001 ----------------------------------------------------------

// TestSpec_REQ_BASE_001_InvalidBaselineFailsClosed implements SC-BASE-F
// (AC-BASE-3): an invalid embedded baseline catalog fails closed with a cause
// before any merge, load, or write work happens.
func TestSpec_REQ_BASE_001_InvalidBaselineFailsClosed(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	valid := writeSkillDir(t, root, "alpha", "alpha", "Would be merged if the baseline were usable.")
	before := snapshotTree(t, root)

	embedded := embeddedBaseline(t)
	invalid := embedded
	invalid.Catalog.Assets = append(slices.Clone(embedded.Catalog.Assets), embedded.Catalog.Assets[0])
	if err := invalid.Catalog.Validate(); err == nil {
		t.Fatal("fixture sanity: duplicated asset must make the baseline catalog invalid")
	}

	resolved, diags := Resolve(context.Background(), newRequest(configFile, []string{valid}), invalid, testPolicy())
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want exactly the baseline failure: %v", len(diags), diagnosticSummary(diags))
	}
	diag := diags[0]
	if diag.Class != ErrorInvalid || diag.Stage != StageMerge || diag.Rule != RuleBaselineCatalogValidity {
		t.Fatalf("diagnostic = %s/%s/%s, want %s/%s/%s",
			diag.Stage, diag.Class, diag.Rule, StageMerge, ErrorInvalid, RuleBaselineCatalogValidity)
	}
	if diag.Cause == nil {
		t.Fatal("baseline failure carries no cause")
	}
	if diag.SafeRemediation == "" {
		t.Error("baseline failure omits its known-safe remediation")
	}
	requireZeroResolved(t, resolved)
	requireTreeUnchanged(t, root, before)
}

// --- REQ-DIAG-001 ----------------------------------------------------------

// TestSpec_REQ_DIAG_001_ErrorClassesObservable implements SC-DIAG1-H
// (AC-DIAG-1): every failure class the registry can produce is observable
// with its correct class, stage, and rule, and no success is claimed.
func TestSpec_REQ_DIAG_001_ErrorClassesObservable(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	cases := []struct {
		name    string
		request func(t *testing.T) Request
		class   ErrorClass
		stage   Stage
		rule    string
	}{
		{name: "untrusted", request: untrustedRequest, class: ErrorUntrusted, stage: StageLoad, rule: RuleSkillProvenance},
		{name: "invalid", request: invalidRequest, class: ErrorInvalid, stage: StageNormalize, rule: RuleFrontmatterName},
		{name: "override", request: overrideRequest, class: ErrorOverride, stage: StageMerge, rule: RuleCanonicalOverride},
		{name: "collision", request: collisionRequest, class: ErrorCollision, stage: StageMerge, rule: RuleCustomCollision},
		{name: "protected disable", request: protectedDisableRequest, class: ErrorProtectedDisable, stage: StageMerge, rule: RuleProtectedDisable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := requireRejection(t, embedded, policy, tc.request(t))
			assertSingleDiagnostic(t, diags, tc.class, tc.stage, tc.rule)
		})
	}
}

// TestSpec_REQ_DIAG_001_MultipleDefectsDeterministic implements SC-DIAG1-E
// (AC-DIAG-2): with multiple defects present, repeated validation yields the
// identical deterministic cause and report with zero writes.
func TestSpec_REQ_DIAG_001_MultipleDefectsDeterministic(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	ghost := filepath.Join(root, "skills", "ghost")                          // index 0: unverifiable
	broken := writeRawSkill(t, root, "broken", []byte("# no frontmatter\n")) // index 1: invalid
	one := writeSkillDir(t, root, "one", "clash", "Identical bytes.")        // index 2: first clash
	two := writeSkillDir(t, root, "two", "clash", "Identical bytes.")        // index 3: duplicate clash
	evil := writeSkillDir(t, root, "evil", "bootstrap", "Replacement body.") // index 4: override
	before := snapshotTree(t, root)

	embedded := embeddedBaseline(t)
	policy := testPolicy()
	req := newRequest(configFile, []string{ghost, broken, one, two, evil}, model.ComponentSDD)

	first, firstDiags := Resolve(context.Background(), req, embedded, policy)
	if len(firstDiags) == 0 {
		t.Fatal("aggregate defect input resolved successfully, want rejection")
	}
	requireZeroResolved(t, first)

	second, secondDiags := Resolve(context.Background(), req, embedded, policy)
	requireZeroResolved(t, second)

	if !reflect.DeepEqual(diagnosticSummary(firstDiags), diagnosticSummary(secondDiags)) {
		t.Fatalf("repeated validation is not deterministic:\nfirst:  %v\nsecond: %v",
			diagnosticSummary(firstDiags), diagnosticSummary(secondDiags))
	}

	// The canonical report ordering is stage, then class, then ID, then rule,
	// then declaration index, with the load-stage untrusted defect primary.
	want := []string{
		"load/untrusted/skill-source-local-provenance/id=<nil>/decl=0",
		"merge/collision/custom-collision/id=clash/decl=3",
		"merge/override/canonical-override/id=bootstrap/decl=4",
		"merge/protected_disable/protected-disable/id=<nil>/decl=0",
		"normalize/invalid/frontmatter-name/id=<nil>/decl=1",
	}
	if got := diagnosticSummary(firstDiags); !reflect.DeepEqual(got, want) {
		t.Fatalf("aggregated report ordering:\ngot:  %v\nwant: %v", got, want)
	}
	for _, diag := range firstDiags {
		if diag.Cause == nil {
			t.Fatalf("diagnostic %s/%s dropped its cause", diag.Class, diag.Rule)
		}
	}
	requireTreeUnchanged(t, root, before)
}

// --- REQ-DIAG-002 ----------------------------------------------------------

// TestSpec_REQ_DIAG_002_ActionableCollision implements SC-DIAG2-H (AC-DIAG-4):
// a collision diagnostic identifies the involved ID, the violated rule, and a
// known-safe remediation, and locates both declarations.
func TestSpec_REQ_DIAG_002_ActionableCollision(t *testing.T) {
	diags := requireRejection(t, embeddedBaseline(t), testPolicy(), collisionRequest(t))
	diag := assertSingleDiagnostic(t, diags, ErrorCollision, StageMerge, RuleCustomCollision)

	if diag.ID == nil || *diag.ID != model.SkillID("clash") {
		t.Errorf("collision diagnostic ID = %v, want the known ID clash", diag.ID)
	}
	if diag.Rule != RuleCustomCollision {
		t.Errorf("collision rule = %q, want %q", diag.Rule, RuleCustomCollision)
	}
	if diag.SafeRemediation == "" || !strings.Contains(diag.SafeRemediation, "duplicate") {
		t.Errorf("collision remediation %q is not actionable for a duplicate declaration", diag.SafeRemediation)
	}
	if diag.Cause == nil ||
		!strings.Contains(diag.Cause.Error(), "index 0") ||
		!strings.Contains(diag.Cause.Error(), "index 1") {
		t.Errorf("collision cause %v does not locate both declarations", diag.Cause)
	}
	if diag.DeclarationIndex != 1 {
		t.Errorf("collision declaration index = %d, want 1", diag.DeclarationIndex)
	}
}

// TestSpec_REQ_DIAG_002_UnknownIDNotInvented implements SC-DIAG2-E (AC-DIAG-5):
// when a failing declaration has no parseable skill ID, diagnostics never
// invent one; the declaration index carries the location instead.
func TestSpec_REQ_DIAG_002_UnknownIDNotInvented(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	t.Run("provenance failure has no parseable ID", func(t *testing.T) {
		diags := requireRejection(t, embedded, policy, untrustedRequest(t))
		for _, diag := range diags {
			if diag.ID != nil {
				t.Fatalf("provenance diagnostic invented skill ID %q", *diag.ID)
			}
		}
		if diags[0].DeclarationIndex != 0 {
			t.Errorf("declaration index = %d, want 0 to locate the failing declaration", diags[0].DeclarationIndex)
		}
	})

	t.Run("unparsable frontmatter has no parseable ID", func(t *testing.T) {
		diags := requireRejection(t, embedded, policy, invalidRequest(t))
		for _, diag := range diags {
			if diag.ID != nil {
				t.Fatalf("normalize diagnostic invented skill ID %q", *diag.ID)
			}
		}
	})

	t.Run("component disable has no skill ID", func(t *testing.T) {
		diags := requireRejection(t, embedded, policy, protectedDisableRequest(t))
		for _, diag := range diags {
			if diag.ID != nil {
				t.Fatalf("component disable diagnostic invented skill ID %q", *diag.ID)
			}
		}
	})
}

// TestSpec_REQ_DIAG_002_UnsafeRemediationOmitted implements SC-DIAG2-F
// (AC-DIAG-6): diagnostics keep the deterministic cause, and remediation
// advice is omitted rather than guessed — what is emitted never weakens a
// protection, touches the embedded canonical baseline, or claims privileges.
func TestSpec_REQ_DIAG_002_UnsafeRemediationOmitted(t *testing.T) {
	embedded := embeddedBaseline(t)
	policy := testPolicy()

	// unsafeMarkers are advice shapes that would weaken protection or expand
	// authority; an empty remediation (advice omitted) is always acceptable.
	unsafeMarkers := []string{
		"force", "ignore", "sudo", "elevate", "chmod", "grant",
		"reclassif", "mark as optional", "delete the embedded",
		"overwrite the embedded", "replace the embedded",
		"disable protection", "trust ",
	}
	requireSafeAdvice := func(t *testing.T, diags Diagnostics) {
		t.Helper()
		for _, diag := range diags {
			if diag.Cause == nil {
				t.Fatalf("diagnostic %s/%s dropped its cause", diag.Class, diag.Rule)
			}
			lowered := strings.ToLower(diag.SafeRemediation)
			for _, marker := range unsafeMarkers {
				if strings.Contains(lowered, marker) {
					t.Errorf("diagnostic %s/%s suggests unsafe remediation %q (matched %q)",
						diag.Class, diag.Rule, diag.SafeRemediation, marker)
				}
			}
		}
	}

	t.Run("unclassifiable disable keeps cause and never advises weakening protection", func(t *testing.T) {
		// No safe remediation can make an unclassifiable component disableable;
		// the only safe action is removing the disable entry.
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		diags := requireRejection(t, embedded, policy, newRequest(configFile, nil, "mystery-component"))
		diag := assertSingleDiagnostic(t, diags, ErrorProtectedDisable, StageMerge, RuleProtectedDisable)
		if diag.Cause == nil || !strings.Contains(diag.Cause.Error(), "protected-unclassified") {
			t.Fatalf("cause %v does not maintain the protected-unclassified fact", diag.Cause)
		}
		if !strings.HasPrefix(diag.SafeRemediation, "remove the disabled-components entry") {
			t.Errorf("remediation %q must only name the known-safe local action", diag.SafeRemediation)
		}
		requireSafeAdvice(t, diags)
	})

	t.Run("invalid baseline keeps cause", func(t *testing.T) {
		invalid := embedded
		invalid.Catalog.Assets = append(slices.Clone(embedded.Catalog.Assets), embedded.Catalog.Assets[0])
		root := t.TempDir()
		configFile := writeOverlayConfig(t, root)
		_, diags := Resolve(context.Background(), newRequest(configFile, nil), invalid, policy)
		if len(diags) == 0 {
			t.Fatal("invalid baseline was accepted")
		}
		requireSafeAdvice(t, diags)
	})

	t.Run("every observable failure class keeps its cause and emits only known-safe advice", func(t *testing.T) {
		requests := []struct {
			name    string
			request func(t *testing.T) Request
		}{
			{name: "untrusted", request: untrustedRequest},
			{name: "invalid", request: invalidRequest},
			{name: "override", request: overrideRequest},
			{name: "collision", request: collisionRequest},
			{name: "protected disable", request: protectedDisableRequest},
		}
		for _, tc := range requests {
			t.Run(tc.name, func(t *testing.T) {
				diags := requireRejection(t, embedded, policy, tc.request(t))
				requireSafeAdvice(t, diags)
			})
		}
	})
}

// --- REQ-REM-B4 -------------------------------------------------------------

// TestSpec_REM_B4_RelativeDeclarationsAnchorToConfigRoot implements the
// REQ-REM-B4 oracle (spec sdd-c561715317924d9a91bf52aabbf8ebfe@2): a
// relative custom skill declaration resolves against the verified canonical
// configuration directory, never against the process working directory,
// while absolute declarations keep their explicit trust behavior and a
// missing verified configuration root stays fail-closed.
func TestSpec_REM_B4_RelativeDeclarationsAnchorToConfigRoot(t *testing.T) {
	// The verified configuration root carries the real skill...
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	doc := skillDoc("anchored", "Body from the verified configuration root.")
	skillDir := writeSkillDir(t, root, "anchored", "anchored", "Body from the verified configuration root.")

	// ...and a decoy directory reuses the same relative spelling beneath
	// the process working directory, so a CWD-anchored resolution would
	// read the decoy instead. The decoy body differs so the trusted bytes
	// identify which directory was actually read.
	decoy := t.TempDir()
	decoyDoc := skillDoc("anchored", "CWD decoy body that must never be read.")
	writeRawSkill(t, decoy, "anchored", decoyDoc)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("record working directory: %v", err)
	}
	if err := os.Chdir(decoy); err != nil {
		t.Fatalf("pin working directory to the decoy: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Errorf("restore working directory %s: %v", originalWD, err)
		}
	})

	embedded := embeddedBaseline(t)
	policy := testPolicy()
	before := snapshotTree(t, root)

	t.Run("relative declaration reads the config root, not the CWD decoy", func(t *testing.T) {
		resolved := mustResolve(t, embedded, policy, newRequest(configFile, []string{filepath.Join("skills", "anchored")}))
		skill, ok := findSkillByID(resolved.EffectiveSkills, "anchored")
		if !ok {
			t.Fatal("custom skill anchored at the config root missing from effective skills")
		}
		if bytes.Equal(skill.Content, decoyDoc) {
			t.Error("anchored content matches the CWD decoy bytes; relative declaration was resolved from the process working directory")
		}
		if !bytes.Equal(skill.Content, doc) {
			t.Errorf("anchored content = %q, want the config-root bytes %q", skill.Content, doc)
		}
		if got, want := skill.ContentSHA256, ir.FingerprintContent(doc); got != want {
			t.Errorf("anchored digest = %s, want %s", got, want)
		}
		if len(resolved.Provenance) != 2 {
			t.Fatalf("provenance records %d entries, want 2 (config + skill source)", len(resolved.Provenance))
		}
		skillEvidence := resolved.Provenance[1]
		if skillEvidence.Kind != EvidenceSkillSource || !skillEvidence.Verified {
			t.Fatalf("skill evidence = %+v, want a verified skill source", skillEvidence)
		}
		if skillEvidence.ConfigRelativePath != "skills/anchored" {
			t.Errorf("skill evidence path = %q, want %q", skillEvidence.ConfigRelativePath, "skills/anchored")
		}
	})

	t.Run("absolute declaration trust is unchanged while CWD points at the decoy", func(t *testing.T) {
		resolved := mustResolve(t, embedded, policy, newRequest(configFile, []string{skillDir}))
		skill, ok := findSkillByID(resolved.EffectiveSkills, "anchored")
		if !ok {
			t.Fatal("absolute declaration no longer verifies")
		}
		if !bytes.Equal(skill.Content, doc) {
			t.Errorf("absolute declaration content = %q, want the config-root bytes %q", skill.Content, doc)
		}
	})

	t.Run("relative declaration without a verified config root stays fail-closed", func(t *testing.T) {
		diags := requireRejection(t, embedded, policy, newRequest("", []string{filepath.Join("skills", "anchored")}))
		assertSingleDiagnostic(t, diags, ErrorUntrusted, StageLoad, RuleSkillProvenance)
	})

	requireTreeUnchanged(t, root, before)
}

// --- REQ-REM-B3 -------------------------------------------------------------

// TestSpec_REM_B3_ReceiptContainsOnlyRetainedSelectedComponents implements
// SC-B3-TRUTH (spec sdd-c561715317924d9a91bf52aabbf8ebfe@2, design D4): the
// retained component selection handed over by the pipeline is a strict
// subset of the policy classification map, the map still classifies
// unselected IDs (retained-dep, context7), and one retained optional
// component is accepted as disabled. The sealed receipt must therefore list
// exactly the sorted retained selection minus the accepted disable and must
// exclude every unselected policy-only ID: the authorization map never
// defines the effective selection.
func TestSpec_REM_B3_ReceiptContainsOnlyRetainedSelectedComponents(t *testing.T) {
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Retained-only receipt probe.")

	embedded := embeddedBaseline(t)
	policy := testPolicy()

	// Retained selection in non-sorted declaration order so the oracle also
	// proves canonical ordering. retained-dep is deliberately absent: policy
	// classifies it as a protected dependency, but nothing selected retains
	// it, so it must not appear as effective. context7 is retained and then
	// disabled (Optional), so it must disappear from the effective set.
	retained := []model.ComponentID{model.ComponentSDD, model.ComponentCortex, model.ComponentContext7, model.ComponentForgeSpec, model.ComponentSkills}
	req := newRetainedRequest(configFile, retained, []string{alphaDir}, model.ComponentContext7)

	resolved := mustResolve(t, embedded, policy, req)

	// Fixture sanity: the disable was accepted, not rejected.
	if !slices.Contains(resolved.Disabled, model.ComponentContext7) {
		t.Fatalf("fixture sanity: context7 disable was not accepted: disabled = %v", resolved.Disabled)
	}

	want := []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec, model.ComponentSDD, model.ComponentSkills}
	got := resolved.CanonicalReceipt.EffectiveComponents
	if !slices.Equal(got, want) {
		t.Fatalf("effective components = %v, want exactly the sorted retained selection minus accepted disables %v: unselected policy-only IDs such as retained-dep must never leak into the receipt", got, want)
	}
	if !slices.IsSorted(got) {
		t.Errorf("effective components %v are not sorted", got)
	}
	if err := ValidateReceipt(resolved.CanonicalReceipt); err != nil {
		t.Fatalf("retained-only receipt is not sealed correctly: %v", err)
	}
}
