package renderers

import (
	"bytes"
	"errors"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// stubSkillLayout is a generic in-memory agents.SkillLayoutProvider double: it
// declares exactly the destinations a test wants, records every typed skill it
// receives, and performs no filesystem access, mirroring the purity the
// adapter contract requires.
type stubSkillLayout struct {
	destinations map[string][]string
	observed     []skillcore.Skill
}

func (s *stubSkillLayout) SkillDestinations(skill skillcore.Skill) []string {
	s.observed = append(s.observed, skill)
	if s.destinations == nil {
		return nil
	}
	return slices.Clone(s.destinations[string(skill.ID)])
}

var (
	_ agents.SkillLayoutProvider = (*stubSkillLayout)(nil)
	_ agents.SkillLayoutProvider = opencode.NewAdapter()
)

// customSkillFixture builds one custom skill record shaped exactly as the
// registry merge delivers it: a grammar-valid ID and canonical LF content with
// the normalization-stage digest and custom origin. The in-package test cannot
// import the registry package (renderers <- registry <- assets <- canonical <-
// renderers would cycle in the test binary), so the fixture derives the same
// canonical values directly from the shared leaf contracts.
func customSkillFixture(t *testing.T, id, document string) skillcore.Skill {
	t.Helper()
	if _, err := registryNormalizeSkillIDForTest(id); err != nil {
		t.Fatalf("normalize fixture skill ID %q: %v", id, err)
	}
	return skillcore.Skill{ID: model.SkillID(id), Content: []byte(document), ContentSHA256: ir.FingerprintContent([]byte(document)), Origin: skillcore.OriginCustom}
}

// registryNormalizeSkillIDForTest validates fixture IDs against the strict
// lowercase ASCII grammar without importing the registry package: the grammar
// is byte-level (one or more [a-z0-9] segments joined by single hyphens), so a
// lexical check is sufficient for fixture construction.
func registryNormalizeSkillIDForTest(id string) (string, error) {
	if id == "" || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") || strings.Contains(id, "--") {
		return "", errors.New("invalid skill ID grammar")
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return "", errors.New("invalid skill ID grammar")
		}
	}
	return id, nil
}

// requireRendererError asserts err is a renderers ValidationError carrying the
// expected semantic ID.
func requireRendererError(t *testing.T, err error, id ir.SemanticID) *ValidationError {
	t.Helper()
	if err == nil {
		t.Fatalf("lowering was accepted, want rejection with ID %q", id)
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error is %T (%v), want *renderers.ValidationError", err, err)
	}
	if validationErr.ID != id {
		t.Fatalf("error ID = %q, want %q (error: %v)", validationErr.ID, id, err)
	}
	return validationErr
}

// TestRenderers_LowerToAdapterDestination covers REQ-ADAPT-001 host-specific
// lowering: a typed custom Skill record must land byte-for-byte at exactly the
// relative destination its adapter declared, as SKILL.md data, with the
// destination authority coming from the layout provider and never from the
// renderer.
func TestRenderers_LowerToAdapterDestination(t *testing.T) {
	document := "---\nname: review-checklist\ndescription: Checklist discipline\n---\n\nFollow the checklist.\n"
	skill := customSkillFixture(t, "review-checklist", document)

	t.Run("lowers to the adapter-declared destination as SKILL.md data", func(t *testing.T) {
		layout := &stubSkillLayout{destinations: map[string][]string{
			"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
		}}
		assets, err := LowerCustomSkills(layout, []skillcore.Skill{skill})
		if err != nil {
			t.Fatalf("lower custom skill: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("lowered %d assets, want exactly 1", len(assets))
		}
		asset := assets[0]
		if asset.Path != ".config/opencode/skills/review-checklist/SKILL.md" {
			t.Errorf("path = %q, want the adapter-declared destination", asset.Path)
		}
		if asset.Kind != AssetSkill {
			t.Errorf("kind = %q, want %q", asset.Kind, AssetSkill)
		}
		if asset.SemanticID != ir.SemanticID("asset/skill/review-checklist") {
			t.Errorf("semantic ID = %q, want asset/skill/review-checklist", asset.SemanticID)
		}
		if !bytes.Equal(asset.Content, []byte(document)) {
			t.Errorf("content was rewritten:\n got %q\nwant %q", asset.Content, document)
		}
		if asset.Mode != 0o644 {
			t.Errorf("mode = %#o, want 0644", asset.Mode)
		}
	})

	t.Run("destination follows the declaring adapter, not the target", func(t *testing.T) {
		layout := &stubSkillLayout{destinations: map[string][]string{
			"review-checklist": {".claude/skills/review-checklist/SKILL.md"},
		}}
		assets, err := LowerCustomSkills(layout, []skillcore.Skill{skill})
		if err != nil {
			t.Fatalf("lower custom skill through a second adapter layout: %v", err)
		}
		if len(assets) != 1 || assets[0].Path != ".claude/skills/review-checklist/SKILL.md" {
			t.Fatalf("assets = %+v, want the identical skill at the second adapter's declared destination", assets)
		}
	})

	t.Run("adapter receives the typed skill record", func(t *testing.T) {
		layout := &stubSkillLayout{destinations: map[string][]string{
			"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
		}}
		if _, err := LowerCustomSkills(layout, []skillcore.Skill{skill}); err != nil {
			t.Fatalf("lower custom skill: %v", err)
		}
		if len(layout.observed) != 1 {
			t.Fatalf("layout observed %d skills, want 1", len(layout.observed))
		}
		received := layout.observed[0]
		if received.ID != skill.ID || !bytes.Equal(received.Content, skill.Content) || received.ContentSHA256 != skill.ContentSHA256 {
			t.Errorf("layout received %+v, want the typed skill record unchanged", received)
		}
	})

	t.Run("lowers through the opencode adapter layout", func(t *testing.T) {
		assets, err := LowerCustomSkills(opencode.NewAdapter(), []skillcore.Skill{skill})
		if err != nil {
			t.Fatalf("lower custom skill through the opencode layout: %v", err)
		}
		if len(assets) != 1 || assets[0].Path != ".config/opencode/skills/review-checklist/SKILL.md" {
			t.Fatalf("assets = %+v, want the opencode-declared .config/opencode/skills/<id>/SKILL.md destination", assets)
		}
	})

	t.Run("output is ordered by destination regardless of declaration order", func(t *testing.T) {
		alpha := customSkillFixture(t, "alpha-notes", "# Alpha\n\nFirst.\n")
		beta := customSkillFixture(t, "beta-notes", "# Beta\n\nSecond.\n")
		layout := &stubSkillLayout{destinations: map[string][]string{
			"alpha-notes": {".config/opencode/skills/alpha-notes/SKILL.md"},
			"beta-notes":  {".config/opencode/skills/beta-notes/SKILL.md"},
		}}
		forward, err := LowerCustomSkills(layout, []skillcore.Skill{alpha, beta})
		if err != nil {
			t.Fatalf("lower ordered overlay: %v", err)
		}
		reversed, err := LowerCustomSkills(layout, []skillcore.Skill{beta, alpha})
		if err != nil {
			t.Fatalf("lower reversed overlay: %v", err)
		}
		if !slices.EqualFunc(forward, reversed, func(left, right Asset) bool {
			return left.Path == right.Path && left.SemanticID == right.SemanticID && bytes.Equal(left.Content, right.Content)
		}) {
			t.Errorf("lowering is declaration-order dependent:\n forward = %+v\n reversed = %+v", forward, reversed)
		}
		if !slices.IsSortedFunc(forward, func(left, right Asset) int { return strings.Compare(left.Path, right.Path) }) {
			t.Errorf("lowered assets are not sorted by destination: %+v", forward)
		}
	})

	t.Run("empty overlay lowers nothing", func(t *testing.T) {
		for _, layout := range []agents.SkillLayoutProvider{nil, &stubSkillLayout{}} {
			assets, err := LowerCustomSkills(layout, nil)
			if err != nil {
				t.Fatalf("lower empty overlay: %v", err)
			}
			if len(assets) != 0 {
				t.Errorf("empty overlay lowered %d assets, want none", len(assets))
			}
		}
	})

	errorCases := []struct {
		name       string
		layout     agents.SkillLayoutProvider
		skills     func(t *testing.T) []skillcore.Skill
		wantError  ir.SemanticID
		wantSubstr string
	}{
		{
			name:      "overlay without an adapter layout fails closed",
			layout:    nil,
			skills:    func(t *testing.T) []skillcore.Skill { return []skillcore.Skill{skill} },
			wantError: ErrorUndeclaredSkillLayout,
		},
		{
			name: "adapter declaring no destination is unrepresentable",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": nil,
			}},
			skills:    func(t *testing.T) []skillcore.Skill { return []skillcore.Skill{skill} },
			wantError: ErrorUnrepresentableSkill,
		},
		{
			name: "adapter declaring several destinations is unrepresentable",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md", ".config/opencode/skills/review-checklist-copy/SKILL.md"},
			}},
			skills:    func(t *testing.T) []skillcore.Skill { return []skillcore.Skill{skill} },
			wantError: ErrorUnrepresentableSkill,
		},
		{
			name: "embedded-origin records are never lowered",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
			}},
			skills: func(t *testing.T) []skillcore.Skill {
				embedded := customSkillFixture(t, "review-checklist", document)
				embedded.Origin = skillcore.OriginEmbedded
				return []skillcore.Skill{embedded}
			},
			wantError: ErrorUnrepresentableSkill,
		},
		{
			name: "empty content is unrepresentable",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
			}},
			skills: func(t *testing.T) []skillcore.Skill {
				return []skillcore.Skill{{ID: skill.ID, Content: nil, ContentSHA256: ir.FingerprintContent(nil), Origin: skillcore.OriginCustom}}
			},
			wantError: ErrorUnrepresentableSkill,
		},
		{
			name: "digest disagreeing with content is rejected",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
			}},
			skills: func(t *testing.T) []skillcore.Skill {
				mismatched := customSkillFixture(t, "review-checklist", document)
				mismatched.ContentSHA256 = ir.FingerprintContent([]byte("tampered"))
				return []skillcore.Skill{mismatched}
			},
			wantError: ErrorInvalidAsset,
		},
		{
			name: "duplicate skill identity is rejected",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {".config/opencode/skills/review-checklist/SKILL.md"},
			}},
			skills: func(t *testing.T) []skillcore.Skill {
				return []skillcore.Skill{customSkillFixture(t, "review-checklist", document), customSkillFixture(t, "review-checklist", "# Other\n\nBody.\n")}
			},
			wantError: ErrorDuplicateAsset,
		},
		{
			name: "duplicate destination across skills is rejected",
			layout: &stubSkillLayout{destinations: map[string][]string{
				"alpha-notes": {".config/opencode/skills/colliding/SKILL.md"},
				"beta-notes":  {".config/opencode/skills/colliding/SKILL.md"},
			}},
			skills: func(t *testing.T) []skillcore.Skill {
				return []skillcore.Skill{customSkillFixture(t, "alpha-notes", "# Alpha\n"), customSkillFixture(t, "beta-notes", "# Beta\n")}
			},
			wantError: ErrorDuplicateAsset,
		},
	}
	unsafeDestinations := []string{
		"../escape/SKILL.md",
		"/absolute/SKILL.md",
		`.config\opencode\skills\review-checklist\SKILL.md`,
		"C:/skills/review-checklist/SKILL.md",
		".config/opencode/skills//review-checklist/SKILL.md",
		".config/opencode/skills/review-checklist/agent.md",
		".config/opencode/skills/review-checklist/opencode.json",
		".config/opencode/skills/review-checklist/subagent.md",
		"internal/skills/review-checklist/SKILL.md",
		"src/skills/review-checklist/SKILL.md",
		"testdata/skills/review-checklist/SKILL.md",
	}
	for _, destination := range unsafeDestinations {
		errorCases = append(errorCases, struct {
			name       string
			layout     agents.SkillLayoutProvider
			skills     func(t *testing.T) []skillcore.Skill
			wantError  ir.SemanticID
			wantSubstr string
		}{
			name: "unsafe destination " + destination,
			layout: &stubSkillLayout{destinations: map[string][]string{
				"review-checklist": {destination},
			}},
			skills:     func(t *testing.T) []skillcore.Skill { return []skillcore.Skill{skill} },
			wantError:  ErrorUnsafeSkillDestination,
			wantSubstr: "SKILL.md",
		})
	}
	for _, testCase := range errorCases {
		t.Run(testCase.name, func(t *testing.T) {
			validationErr := requireRendererError(t, func() error {
				_, err := LowerCustomSkills(testCase.layout, testCase.skills(t))
				return err
			}(), testCase.wantError)
			if testCase.wantSubstr != "" && !strings.Contains(validationErr.Error(), testCase.wantSubstr) {
				t.Errorf("error %q does not mention %q", validationErr.Error(), testCase.wantSubstr)
			}
		})
	}
}

// TestRenderers_NoBindingOrPermission covers AC-ADAPT-3: custom skills lower as
// plain SKILL.md data only. No command, subagent, config, tool, permission, or
// binding overlay may appear in the lowered output, the bytes are never
// rewritten to grant authority, and the result validates against a bundle
// scope that allows only the skill kind and no permissions at all.
func TestRenderers_NoBindingOrPermission(t *testing.T) {
	skills := []skillcore.Skill{
		customSkillFixture(t, "audit-helper", "---\nname: audit-helper\ndescription: Audit steps\n---\n\nStep one.\n"),
		customSkillFixture(t, "release-notes", "# Release notes\n\nCollect changes.\n"),
		customSkillFixture(t, "hostile-audit", "---\nname: hostile-audit\ndescription: Hostile frontmatter\npermissions:\n  bash: allow\ntools:\n  shell: true\nagents:\n  auditor\nbindings:\n  role/auditor: skill/hostile-audit\n---\n\nInert data bytes.\n"),
	}
	layout := &stubSkillLayout{destinations: map[string][]string{
		"audit-helper":  {".config/opencode/skills/audit-helper/SKILL.md"},
		"release-notes": {".config/opencode/skills/release-notes/SKILL.md"},
		"hostile-audit": {".config/opencode/skills/hostile-audit/SKILL.md"},
	}}

	assets, err := LowerCustomSkills(layout, skills)
	if err != nil {
		t.Fatalf("lower custom skills: %v", err)
	}
	if len(assets) != len(skills) {
		t.Fatalf("lowered %d assets, want %d", len(assets), len(skills))
	}

	authorityKinds := []AssetKind{AssetCommand, AssetAgent, AssetPermission, AssetHook, AssetMCP, AssetInstruction, AssetRule, AssetModel, AssetSchema, AssetFixture}
	byPath := make(map[string]skillcore.Skill, len(skills))
	for _, skill := range skills {
		byPath[".config/opencode/skills/"+string(skill.ID)+"/SKILL.md"] = skill
	}
	for _, asset := range assets {
		if asset.Kind != AssetSkill {
			t.Errorf("asset %s lowered as kind %q: custom skills are plain skill data, never %v overlays", asset.SemanticID, asset.Kind, authorityKinds)
		}
		if slices.Contains(authorityKinds, asset.Kind) {
			t.Errorf("asset %s lowered as authority kind %q", asset.SemanticID, asset.Kind)
		}
		if len(asset.Permissions) != 0 {
			t.Errorf("asset %s carries permissions %v: custom skills never widen permissions", asset.SemanticID, asset.Permissions)
		}
		if len(asset.Extensions) != 0 {
			t.Errorf("asset %s declares extensions %v: custom skills never bind native extensions", asset.SemanticID, asset.Extensions)
		}
		if path.Base(asset.Path) != "SKILL.md" {
			t.Errorf("asset %s landed at %q: custom skills may only become SKILL.md data files", asset.SemanticID, asset.Path)
		}
		declared, ok := byPath[asset.Path]
		if !ok {
			t.Fatalf("asset %s landed at undeclared destination %q", asset.SemanticID, asset.Path)
		}
		if !bytes.Equal(asset.Content, declared.Content) {
			t.Errorf("asset %s content was rewritten from %q to %q: the renderer never adds authority, binding, or permission bytes", asset.SemanticID, declared.Content, asset.Content)
		}
	}

	resolved := ResolvedWorkflow{
		Target:             "opencode",
		Profile:            "portable-flat",
		AllowedAssetKinds:  []AssetKind{AssetSkill},
		AllowedPermissions: []string{},
	}
	normalized, err := ValidateBundle(resolved, Bundle{Assets: assets})
	if err != nil {
		t.Fatalf("lowered custom skills must validate in a skill-only, permission-free scope: %v", err)
	}
	if len(normalized.Assets) != len(assets) {
		t.Errorf("validation changed the asset count from %d to %d", len(assets), len(normalized.Assets))
	}
	for _, asset := range normalized.Assets {
		if asset.Kind != AssetSkill || len(asset.Permissions) != 0 || len(asset.Extensions) != 0 {
			t.Errorf("validated asset %s = %+v, want unchanged plain skill data", asset.SemanticID, asset)
		}
	}
}

// TestRenderers_CompositionCustomSkillsFailClosed pins the wiring between the
// resolved workflow and the lowering: custom skills carried by the composition
// reach the adapter-declared lowering inside the shared composition path, are
// rejected when no layout was declared (no destination is ever guessed), and
// never let an incomplete composition slip through.
func TestRenderers_CompositionCustomSkillsFailClosed(t *testing.T) {
	skill := customSkillFixture(t, "audit-helper", "# Audit helper\n\nChecklist.\n")
	resolved := ResolvedWorkflow{
		Target:      "opencode",
		Profile:     "portable-flat",
		Composition: Composition{CustomSkills: []skillcore.Skill{skill}},
	}

	t.Run("custom skills without an adapter layout are rejected, not guessed", func(t *testing.T) {
		_, err := appendCompositionAsset(resolved, nil)
		requireRendererError(t, err, ErrorUndeclaredSkillLayout)
	})

	t.Run("custom skills do not bypass composition completeness", func(t *testing.T) {
		withLayout := resolved
		withLayout.SkillLayout = &stubSkillLayout{destinations: map[string][]string{
			"audit-helper": {".config/opencode/skills/audit-helper/SKILL.md"},
		}}
		_, err := appendCompositionAsset(withLayout, nil)
		if err == nil {
			t.Fatalf("incomplete composition with custom skills was accepted, want fail-closed rejection")
		}
		if !strings.Contains(err.Error(), "is missing or unsafe") {
			t.Errorf("error %v does not report the incomplete composition fields", err)
		}
	})
}
