package registry

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// This file is the merge boundary and the Resolve orchestrator. It composes
// the pure stages owned by loader.go, normalize.go, and receipt.go into one
// deterministic resolution: every declared local source is verified, every
// verified source is normalized into a custom skill declaration, the
// declarations are merged additively onto the embedded baseline catalog, and
// the effective input is sealed into the canonical receipt. Nothing here
// mutates the filesystem, so every rejection is a pure diagnostic.

// Merge-stage rule names (see loader.go and normalize.go for the other
// stages' rules). Diagnostics cite these instead of restating the merge
// policy so reports stay stable across policy revisions.
const (
	// RuleBaselineCatalogValidity guards the embedded baseline catalog
	// before any merge work happens (fail-closed per SC-BASE-F).
	RuleBaselineCatalogValidity = "baseline-catalog-validity"
	// RuleFrontmatterName guards the SKILL.md frontmatter block that
	// declares the custom skill identity (design D2). It is evaluated in
	// the normalize stage of the Resolve orchestration.
	RuleFrontmatterName = "frontmatter-name"
	// RuleCanonicalOverride guards custom IDs that would replace an
	// embedded canonical asset. The merge is strictly additive.
	RuleCanonicalOverride = "canonical-override"
	// RuleCustomCollision guards two effective custom declarations sharing
	// one ID, even with identical content.
	RuleCustomCollision = "custom-collision"
	// RuleProtectedDisable guards disable attempts against protected
	// component classes, including unclassified components (fail-closed).
	RuleProtectedDisable = "protected-disable"
	// RuleMergedCatalogValidity is the defensive post-merge invariant: the
	// effective catalog must validate exactly like the embedded baseline.
	RuleMergedCatalogValidity = "merged-catalog-validity"
)

// Known-safe remediations cited by merge-stage diagnostics. They only ever
// name actions on the declared local configuration; the merge never invents
// remediations that touch anything else.
const (
	remediationBaselineInvalid = "rebuild or reinstall the embedded baseline catalog before declaring custom skills"
	remediationFrontmatterName = "give the custom skill SKILL.md a YAML frontmatter block with a name field"
	remediationContentEncoding = "re-save the custom skill SKILL.md as valid UTF-8"
	remediationOverride        = "rename the custom skill to an ID that is not an embedded canonical skill ID"
	remediationCollision       = "declare each custom skill ID once and remove the duplicate declaration"
)

// Frontmatter shape constants (design D2): the custom skill identity is
// declared by the SKILL.md frontmatter, never by directory or path spelling.
const frontmatterDelimiter = "---"

// Catalog merge conventions. They mirror the unexported conventions the
// assets catalog applies to custom skills so the merged catalog is identical
// to one built by assets.BuildEffectiveCatalogFromFS; the constants are
// restated here because the assets package does not export them.
const (
	// skillAssetIDPrefix is the semantic ID namespace of skill assets.
	skillAssetIDPrefix = "asset/skill/"
	// customSkillMaxTokens mirrors the embedded skill token budget.
	customSkillMaxTokens = 3500
)

// Resolve verifies, normalizes, and additively merges every custom skill
// declared by req onto the embedded baseline catalog under policy, then seals
// the effective input into the canonical receipt. It is the single registry
// ingress orchestrator: config stays transport-only, adapters stay host-only,
// and no stage mutates the filesystem, so a non-empty Diagnostics result is
// always a pre-write rejection with a deterministic primary cause.
//
// The ctx parameter is accepted for signature stability and future
// cancellation propagation; every stage performed here is pure and local.
func Resolve(ctx context.Context, req Request, embedded assets.MaterializedCatalog, policy Policy) (Resolved, Diagnostics) {
	_ = ctx

	// The baseline must be valid before any merge work: an unusable
	// embedded catalog can never anchor custom additions (SC-BASE-F).
	if err := embedded.Catalog.Validate(); err != nil {
		return Resolved{}, SortDiagnostics(Diagnostics{{
			Class:           ErrorInvalid,
			Stage:           StageMerge,
			Rule:            RuleBaselineCatalogValidity,
			SafeRemediation: remediationBaselineInvalid,
			Cause:           fmt.Errorf("rule %s: %w", RuleBaselineCatalogValidity, err),
		}})
	}

	loaded, loadDiags := Load(req)
	diags := append(Diagnostics(nil), loadDiags...)

	// Normalize every verified source into a custom skill declaration.
	type declaration struct {
		index int
		skill Skill
	}
	declarations := make([]declaration, 0, len(loaded.Sources))
	for _, source := range loaded.Sources {
		if !source.Evidence.Verified {
			continue
		}
		skill, diag := normalizeDeclaration(source)
		if diag != nil {
			diags = append(diags, *diag)
			continue
		}
		declarations = append(declarations, declaration{index: source.DeclarationIndex, skill: skill})
	}

	// Protected-disable checks are fail-closed: only components explicitly
	// classified Optional may be disabled; a repeated accepted disable is
	// the same deterministic fact as one (SC-SEL-E).
	disabled := make([]model.ComponentID, 0, len(req.Selection.DisabledComponents))
	for index, id := range req.Selection.DisabledComponents {
		if slices.Contains(disabled, id) {
			continue
		}
		if class, classified := policy.ComponentClasses[id]; classified && class == Optional {
			disabled = append(disabled, id)
			continue
		}
		diags = append(diags, protectedDisableDiagnostic(id, index, policy))
	}

	// Additive merge checks: a custom ID may never replace an embedded
	// canonical asset (SC-REG2-F) and must be unique even with identical
	// bytes (SC-REG2-E).
	embeddedIDs := make(map[ir.SemanticID]struct{}, len(embedded.Catalog.Assets))
	for _, spec := range embedded.Catalog.Assets {
		embeddedIDs[spec.ID] = struct{}{}
	}
	firstSeen := make(map[model.SkillID]int, len(declarations))
	customs := make([]Skill, 0, len(declarations))
	for _, decl := range declarations {
		semantic := ir.SemanticID(skillAssetIDPrefix + string(decl.skill.ID))
		if _, exists := embeddedIDs[semantic]; exists {
			diags = append(diags, overrideDiagnostic(decl.skill.ID, semantic, decl.index))
			continue
		}
		if first, exists := firstSeen[decl.skill.ID]; exists {
			diags = append(diags, collisionDiagnostic(decl.skill.ID, first, decl.index))
			continue
		}
		firstSeen[decl.skill.ID] = decl.index
		customs = append(customs, decl.skill)
	}

	if len(diags) > 0 {
		return Resolved{}, SortDiagnostics(diags)
	}

	merged, diag := mergeCatalog(embedded, customs)
	if diag != nil {
		return Resolved{}, SortDiagnostics(append(diags, *diag))
	}

	// The effective skill set keeps embedded and custom skills in one
	// canonical order so equivalent inputs produce identical evidence.
	effective := append(embeddedSkills(embedded), customs...)
	skillSet := BuildSkillSet(effective)

	disabledSet := make(map[model.ComponentID]struct{}, len(disabled))
	for _, id := range disabled {
		disabledSet[id] = struct{}{}
	}
	// Receipt truthfulness (design D4, REQ-REM-B3): EffectiveComponents is
	// derived only from the retained selection handed over explicitly by
	// the pipeline. Policy.ComponentClasses authorizes disables; it
	// classifies every catalog component by design and therefore never
	// defines the effective selection.
	effectiveComponents := retainedEffectiveComponents(req.RetainedComponents, disabledSet)
	slices.Sort(disabled)

	receipt := SealReceipt(Receipt{
		SchemaVersion:       ReceiptSchemaVersion,
		PolicyDigest:        FingerprintPolicy(policy),
		BaselineDigest:      embedded.Fingerprint(),
		EffectiveComponents: effectiveComponents,
		EffectiveSkills:     skillSet,
		// HostOutputs stay empty here: adapter-relative outputs are added
		// by the renderer and bundle stages after Resolve.
	})

	return Resolved{
		Catalog:          merged,
		EffectiveSkills:  skillSet.Ordered,
		Disabled:         disabled,
		Provenance:       provenance(loaded),
		CanonicalReceipt: receipt,
	}, nil
}

// retainedEffectiveComponents projects the explicit retained selection into
// the receipt's effective component list: accepted disables are subtracted
// defensively, duplicates collapse, and the result is sorted so equivalent
// selections seal identical evidence. Policy classifications never
// contribute: the map authorizes disables, it does not define selection.
func retainedEffectiveComponents(retained []model.ComponentID, disabledSet map[model.ComponentID]struct{}) []model.ComponentID {
	effective := make([]model.ComponentID, 0, len(retained))
	seen := make(map[model.ComponentID]struct{}, len(retained))
	for _, id := range retained {
		if _, off := disabledSet[id]; off {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		effective = append(effective, id)
	}
	slices.Sort(effective)
	return effective
}

// normalizeDeclaration turns one verified loaded source into a custom Skill.
// The identity comes from the validated SKILL.md frontmatter (design D2) and
// the bytes are canonicalized by the normalize stage before hashing, so the
// declaration is exactly what the merge and receipt observe.
func normalizeDeclaration(source LoadedSource) (Skill, *Diagnostic) {
	content, digest, err := NormalizeContent(source.Raw)
	if err != nil {
		return Skill{}, &Diagnostic{
			Class:            ErrorInvalid,
			Stage:            StageNormalize,
			Rule:             RuleContentEncoding,
			DeclarationIndex: source.DeclarationIndex,
			SafeRemediation:  remediationContentEncoding,
			Cause:            err,
		}
	}
	name, err := extractFrontmatterName(content)
	if err != nil {
		return Skill{}, &Diagnostic{
			Class:            ErrorInvalid,
			Stage:            StageNormalize,
			Rule:             RuleFrontmatterName,
			DeclarationIndex: source.DeclarationIndex,
			SafeRemediation:  remediationFrontmatterName,
			Cause:            err,
		}
	}
	id, err := NormalizeSkillID(name)
	if err != nil {
		return Skill{}, &Diagnostic{
			Class:            ErrorInvalid,
			Stage:            StageNormalize,
			Rule:             RuleSkillIDGrammar,
			DeclarationIndex: source.DeclarationIndex,
			SafeRemediation:  remediationFrontmatterName,
			Cause:            err,
		}
	}
	return Skill{ID: id, Content: content, ContentSHA256: digest, Origin: OriginCustom}, nil
}

// extractFrontmatterName returns the declared name from the canonical
// SKILL.md frontmatter block. Content must already be LF-normalized. A
// missing, unterminated, or unparsable frontmatter block is an error; a
// present-but-empty name is returned as "" so the ID grammar rejects it
// instead of the extraction inventing an identity.
func extractFrontmatterName(content []byte) (string, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSuffix(lines[0], "\r") != frontmatterDelimiter {
		return "", fmt.Errorf("rule %s: skill frontmatter must open with a %q delimiter line", RuleFrontmatterName, frontmatterDelimiter)
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] != frontmatterDelimiter {
			continue
		}
		var header struct {
			Name string `yaml:"name"`
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &header); err != nil {
			return "", fmt.Errorf("rule %s: parse skill frontmatter: %w", RuleFrontmatterName, err)
		}
		return header.Name, nil
	}
	return "", fmt.Errorf("rule %s: skill frontmatter must close with a %q delimiter line", RuleFrontmatterName, frontmatterDelimiter)
}

// protectedDisableDiagnostic reports one protected disable attempt with the
// protection category identified (SC-SEL-F). An ID missing from the policy
// map is reported as unclassified, which is also protected.
func protectedDisableDiagnostic(id model.ComponentID, index int, policy Policy) Diagnostic {
	category := "protected-unclassified"
	if class, classified := policy.ComponentClasses[id]; classified {
		category = disableClassLabel(class)
	}
	return Diagnostic{
		Class:            ErrorProtectedDisable,
		Stage:            StageMerge,
		Rule:             RuleProtectedDisable,
		DeclarationIndex: index,
		SafeRemediation:  "remove the disabled-components entry: only components classified optional may be disabled",
		Cause:            fmt.Errorf("rule %s: component %q is %s and cannot be disabled", RuleProtectedDisable, id, category),
	}
}

// overrideDiagnostic reports a custom ID that would replace an embedded
// canonical asset. Embedded assets are always the base and are never
// replaced: the overlay is strictly additive (SC-REG2-F).
func overrideDiagnostic(id model.SkillID, semantic ir.SemanticID, index int) Diagnostic {
	return Diagnostic{
		Class:            ErrorOverride,
		Stage:            StageMerge,
		ID:               &id,
		Rule:             RuleCanonicalOverride,
		DeclarationIndex: index,
		SafeRemediation:  remediationOverride,
		Cause:            fmt.Errorf("rule %s: custom skill %q overrides embedded asset %q", RuleCanonicalOverride, id, semantic),
	}
}

// collisionDiagnostic reports two effective custom declarations sharing one
// ID. The rejection is content-independent: identical bytes still collide
// (SC-REG2-E).
func collisionDiagnostic(id model.SkillID, first, second int) Diagnostic {
	return Diagnostic{
		Class:            ErrorCollision,
		Stage:            StageMerge,
		ID:               &id,
		Rule:             RuleCustomCollision,
		DeclarationIndex: second,
		SafeRemediation:  remediationCollision,
		Cause:            fmt.Errorf("rule %s: custom skill %q is declared at both index %d and index %d", RuleCustomCollision, id, first, second),
	}
}

// mergeCatalog returns a cloned effective catalog: the embedded baseline plus
// the accepted custom skills appended as skill assets and re-sorted, so an
// empty overlay is byte-for-byte the embedded baseline (SC-BASE-E). The
// passed catalog is never mutated.
func mergeCatalog(embedded assets.MaterializedCatalog, customs []Skill) (assets.MaterializedCatalog, *Diagnostic) {
	specs := make([]ir.AssetSpec, 0, len(embedded.Catalog.Assets)+len(customs))
	specs = append(specs, embedded.Catalog.Assets...)
	contents := make(map[ir.SemanticID][]byte, len(embedded.Contents)+len(customs))
	for id, content := range embedded.Contents {
		contents[id] = content
	}
	for _, skill := range customs {
		spec := ir.AssetSpec{
			ID:         ir.SemanticID(skillAssetIDPrefix + string(skill.ID)),
			Class:      ir.AssetSkill,
			SourcePath: "skills/" + string(skill.ID) + "/SKILL.md",
			Required:   true,
			MaxTokens:  customSkillMaxTokens,
			SHA256:     skill.ContentSHA256,
		}
		specs = append(specs, spec)
		contents[spec.ID] = skill.Content
	}
	slices.SortFunc(specs, func(a, b ir.AssetSpec) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	catalog := ir.AssetCatalog{SchemaVersion: embedded.Catalog.SchemaVersion, Assets: specs}
	if err := catalog.Validate(); err != nil {
		return assets.MaterializedCatalog{}, &Diagnostic{
			Class:           ErrorInvalid,
			Stage:           StageMerge,
			Rule:            RuleMergedCatalogValidity,
			SafeRemediation: remediationBaselineInvalid,
			Cause:           fmt.Errorf("rule %s: %w", RuleMergedCatalogValidity, err),
		}
	}
	return assets.MaterializedCatalog{
		Catalog:           catalog,
		Contents:          contents,
		Generated:         embedded.Generated,
		GeneratorVersion:  embedded.GeneratorVersion,
		SourceFingerprint: embedded.SourceFingerprint,
	}, nil
}

// embeddedSkills projects the skill-class assets of the embedded baseline
// into Skill records. IDs are derived from the semantic ID namespace; shared
// skill assets keep their namespaced suffix (for example "shared/x") because
// they are embedded facts, not custom declarations subject to the custom ID
// grammar. Order follows the already-sorted catalog.
func embeddedSkills(embedded assets.MaterializedCatalog) []Skill {
	skills := make([]Skill, 0, len(embedded.Catalog.Assets))
	for _, spec := range embedded.Catalog.Assets {
		if spec.Class != ir.AssetSkill || !strings.HasPrefix(string(spec.ID), skillAssetIDPrefix) {
			continue
		}
		id := strings.TrimPrefix(string(spec.ID), skillAssetIDPrefix)
		if id == "" {
			continue
		}
		skills = append(skills, Skill{
			ID:            model.SkillID(id),
			Content:       embedded.Contents[spec.ID],
			ContentSHA256: spec.SHA256,
			Origin:        OriginEmbedded,
		})
	}
	return skills
}

// provenance collects the verification evidence of every declared local
// source: the declaring configuration file first, then each declared custom
// skill source in declaration order.
func provenance(loaded Loaded) []Evidence {
	evidence := make([]Evidence, 0, 1+len(loaded.Sources))
	if loaded.HasConfigSource {
		evidence = append(evidence, loaded.ConfigSource)
	}
	for _, source := range loaded.Sources {
		evidence = append(evidence, source.Evidence)
	}
	return evidence
}

// SortDiagnostics returns the deterministic pure-report ordering required by
// design D6: stage, then class, then skill ID (absent first), then rule, then
// declaration index. Sorting is stable and the input report is never
// mutated, so repeated validation yields the identical primary cause.
func SortDiagnostics(report Diagnostics) Diagnostics {
	ordered := slices.Clone(report)
	slices.SortStableFunc(ordered, compareDiagnostics)
	return ordered
}

func compareDiagnostics(first, second Diagnostic) int {
	if diff := strings.Compare(string(first.Stage), string(second.Stage)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(first.Class), string(second.Class)); diff != 0 {
		return diff
	}
	switch {
	case first.ID == nil && second.ID == nil:
	case first.ID == nil:
		return -1
	case second.ID == nil:
		return 1
	default:
		if diff := strings.Compare(string(*first.ID), string(*second.ID)); diff != 0 {
			return diff
		}
	}
	if diff := strings.Compare(first.Rule, second.Rule); diff != 0 {
		return diff
	}
	return cmp.Compare(first.DeclarationIndex, second.DeclarationIndex)
}
