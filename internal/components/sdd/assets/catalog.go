// Package assets owns the single operational asset catalog used by the typed
// compiler and installer. Renderers consume this inventory; they do not scan
// embedded files independently.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	embedded "github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/canonical"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/contractgen"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
	"github.com/lleontor705/cortex-ia/internal/model"
)

var canonicalSkills = []string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize", "debate", "parallel-dispatch", "fast-tdd", "hotfix-triage", "spike-prototype", "code-review-adversary"}
var rootModules = []string{}

// MaterializedCatalog includes immutable bytes alongside the receipt-visible
// catalog metadata so callers never need a second source-of-truth lookup.
type MaterializedCatalog struct {
	Catalog           ir.AssetCatalog
	Contents          map[ir.SemanticID][]byte
	Generated         []contractgen.GeneratedAsset
	GeneratorVersion  string
	SourceFingerprint string
}

// Count returns the number of catalog entries for a semantic ID.
func (c MaterializedCatalog) Count(id string) int {
	count := 0
	for _, asset := range c.Catalog.Assets {
		if string(asset.ID) == id {
			count++
		}
	}
	return count
}

// CustomSkill is one externally validated custom skill record offered as an
// additive overlay on the embedded baseline catalog. Records arrive from the
// registry already provenance-verified and normalized; this catalog enforces
// only the structural overlay rules: additions must be unique, non-empty, and
// must never replace an embedded canonical asset.
type CustomSkill struct {
	// ID is the normalized skill identifier. It derives the catalog semantic
	// ID and the installed skill directory name.
	ID model.SkillID
	// Content is the canonical UTF-8/LF skill body.
	Content []byte
}

// OverrideError reports a custom skill whose ID would replace an embedded
// canonical asset. Embedded assets are always the base and are never
// replaced: the overlay is strictly additive.
type OverrideError struct {
	// SkillID is the offending custom skill ID.
	SkillID string
}

func (e *OverrideError) Error() string {
	return fmt.Sprintf("custom skill %q overrides embedded asset %q: custom skills are additive and never replace embedded assets", e.SkillID, customSkillSemanticID(model.SkillID(e.SkillID)))
}

// CollisionError reports two custom skill declarations sharing one effective
// ID, even when their content is identical.
type CollisionError struct {
	// SkillID is the duplicated custom skill ID.
	SkillID string
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("duplicate custom skill %q: custom IDs must be unique even with identical content", e.SkillID)
}

// customSkillSemanticID derives the catalog semantic ID for a custom skill ID.
func customSkillSemanticID(id model.SkillID) ir.SemanticID {
	return ir.SemanticID("asset/skill/" + string(id))
}

// customSkillSourcePath derives the adapter-neutral skill layout path for a
// custom skill ID, mirroring the embedded skill directory layout.
func customSkillSourcePath(id model.SkillID) string {
	return "skills/" + string(id) + "/SKILL.md"
}

// BuildOperationalCatalog materializes the embedded retained operational set.
func BuildOperationalCatalog() (MaterializedCatalog, error) {
	return BuildOperationalCatalogFromFS(embedded.FS)
}

// BuildOperationalCatalogFromFS is injectable for deterministic catalog tests.
func BuildOperationalCatalogFromFS(source fs.FS) (MaterializedCatalog, error) {
	return BuildEffectiveCatalogFromFS(source, nil)
}

// BuildEffectiveCatalog materializes the effective operational catalog: the
// embedded baseline plus validated custom skill additions. A nil or empty
// overlay yields the byte-for-byte embedded baseline catalog.
func BuildEffectiveCatalog(custom []CustomSkill) (MaterializedCatalog, error) {
	return BuildEffectiveCatalogFromFS(embedded.FS, custom)
}

// BuildEffectiveCatalogFromFS is injectable for deterministic catalog tests.
// Custom skills are appended to the embedded base, validated against embedded
// IDs (override rejection) and among themselves (collision rejection), and
// the combined asset list is re-sorted so an empty overlay is byte-for-byte
// identical to the embedded baseline.
func BuildEffectiveCatalogFromFS(source fs.FS, custom []CustomSkill) (MaterializedCatalog, error) {
	workflow := canonical.Workflow()
	contents := make(map[ir.SemanticID][]byte)
	policy := phasecontract.PolicySnapshot()
	generatorVersion := phasecontract.ContractVersion
	generated, err := contractgen.GenerateReferences(contractgen.GeneratorInput{
		Definitions:       phasecontract.CanonicalDefinitions(),
		Version:           generatorVersion,
		SourceFingerprint: policy.SourceFingerprint,
	})
	if err != nil {
		return MaterializedCatalog{}, fmt.Errorf("generate contract references: %w", err)
	}
	specs := make([]ir.AssetSpec, 0, 64)
	add := func(id ir.SemanticID, class ir.AssetClass, sourcePath string, content []byte, required bool, maxTokens int) error {
		if len(content) == 0 {
			return fmt.Errorf("asset %q is empty", id)
		}
		spec := ir.AssetSpec{ID: id, Class: class, SourcePath: sourcePath, Required: required, MaxTokens: maxTokens, SHA256: ir.FingerprintContent(content)}
		if err := spec.Validate(); err != nil {
			return err
		}
		specs = append(specs, spec)
		contents[id] = append([]byte(nil), content...)
		return nil
	}
	read := func(path string) ([]byte, error) { return fs.ReadFile(source, path) }
	root, err := read("AGENTS.md")
	if err != nil {
		return MaterializedCatalog{}, err
	}
	if err := add("asset/root-index", ir.AssetRootIndex, "AGENTS.md", root, true, 1500); err != nil {
		return MaterializedCatalog{}, err
	}
	orchestrator, err := read("skills/orchestrator/SKILL.md")
	if err != nil {
		return MaterializedCatalog{}, err
	}
	if err := add("asset/skill/orchestrator", ir.AssetSkill, "skills/orchestrator/SKILL.md", orchestrator, true, 1500); err != nil {
		return MaterializedCatalog{}, err
	}
	for _, generatedAsset := range generated {
		if err := add(generatedAsset.SemanticID, generatedAsset.Class, generatedAsset.RelativePath, generatedAsset.Content, true, 1200); err != nil {
			return MaterializedCatalog{}, fmt.Errorf("catalog generated asset %q: %w", generatedAsset.SemanticID, err)
		}
	}
	sharedPath := "_shared/sdd-phase-contract.md"
	for _, module := range rootModules {
		modPath := "root-module/" + module + ".md"
		content, readErr := read(sharedPath)
		if readErr == nil {
			_ = add(ir.SemanticID("asset/root-module/"+module), ir.AssetRootModule, modPath, content, false, 1000)
		}
	}
	shared, err := read(sharedPath)
	if err != nil {
		return MaterializedCatalog{}, err
	}
	if err := add("asset/shared-contract", ir.AssetSharedContract, sharedPath, shared, true, 1000); err != nil {
		return MaterializedCatalog{}, err
	}
	for _, sharedSkill := range []struct {
		name      string
		maxTokens int
	}{
		{name: "cortex-convention", maxTokens: 1000},
		{name: "cortex-advanced", maxTokens: 300},
	} {
		path := "skills/_shared/" + sharedSkill.name + ".md"
		content, readErr := read(path)
		if readErr != nil {
			return MaterializedCatalog{}, readErr
		}
		if err := add(ir.SemanticID("asset/skill/shared/"+sharedSkill.name), ir.AssetSkill, path, content, true, sharedSkill.maxTokens); err != nil {
			return MaterializedCatalog{}, err
		}
	}
	for _, skill := range canonicalSkills {
		path := "skills/" + skill + "/SKILL.md"
		content, readErr := read(path)
		if readErr != nil {
			return MaterializedCatalog{}, readErr
		}
		if err := add(ir.SemanticID("asset/skill/"+skill), ir.AssetSkill, path, content, true, 3500); err != nil {
			return MaterializedCatalog{}, err
		}
	}
	for _, role := range workflow.Roles {
		name := strings.TrimPrefix(string(role.ID), "role/")
		if name == "" {
			return MaterializedCatalog{}, fmt.Errorf("workflow role %q is not canonical", role.ID)
		}
		title := "Phase Role"
		decisionScope := "phase decisions"
		if role.ID == "role/orchestrator" || role.ID == "role/debate" || role.ID == "role/parallel-dispatch" {
			title = "Transverse Role"
			decisionScope = "decisions"
		}
		content := []byte(fmt.Sprintf("# %s: %s\n\n- Semantic ID: `%s`\n- Objective: %s\n- Load exactly one canonical skill before %s.\n", title, name, role.ID, role.Objective, decisionScope))
		if err := add(ir.SemanticID("asset/role/"+name), ir.AssetRoleStub, "generated/roles/"+name+".md", content, true, 300); err != nil {
			return MaterializedCatalog{}, err
		}
	}
	for _, profile := range []string{"portable-sequential", "portable-flat", "native-advanced"} {
		profPath := "profile-overlay/" + profile + ".md"
		content, readErr := read(sharedPath)
		if readErr == nil {
			specID := ir.SemanticID("asset/profile-overlay/" + profile)
			if err := add(specID, ir.AssetProfileOverlay, profPath, content, true, 800); err == nil {
				specs[len(specs)-1].Profiles = []ir.SemanticID{ir.SemanticID(profile)}
			}
		}
	}
	quality := []byte("# QualityPlan template\n\nApply the selected QualityPlan and record evidence before handoff.\n")
	if err := add("asset/quality-template", ir.AssetQualityTemplate, "generated/quality-plan-template.md", quality, true, 800); err != nil {
		return MaterializedCatalog{}, err
	}
	if entries, readErr := fs.ReadDir(embedded.FS, "commands"); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				cmdName := strings.TrimSuffix(entry.Name(), ".md")
				sourcePath := "commands/" + entry.Name()
				content, readErr := read(sourcePath)
				if readErr != nil {
					return MaterializedCatalog{}, readErr
				}
				if err := add(ir.SemanticID("command/"+cmdName), ir.AssetCommand, sourcePath, content, true, 1000); err != nil {
					return MaterializedCatalog{}, err
				}
			}
		}
	}
	for _, item := range []struct {
		id   ir.SemanticID
		body string
	}{
		{"asset/manifest/security", "{\"kind\":\"permission-intersection\"}\n"},
		{"asset/manifest/model-routes", "{\"kind\":\"primary-fallback-routes\"}\n"},
		{"asset/manifest/receipt-inventory", "{\"kind\":\"receipt-inventory\"}\n"},
	} {
		if err := add(item.id, ir.AssetManifest, "generated/"+string(item.id)+".json", []byte(item.body), true, 500); err != nil {
			return MaterializedCatalog{}, err
		}
	}
	embeddedIDs := make(map[ir.SemanticID]struct{}, len(specs))
	for _, spec := range specs {
		embeddedIDs[spec.ID] = struct{}{}
	}
	customIDs := make(map[ir.SemanticID]struct{}, len(custom))
	for _, skill := range custom {
		id := customSkillSemanticID(skill.ID)
		if _, exists := embeddedIDs[id]; exists {
			return MaterializedCatalog{}, &OverrideError{SkillID: string(skill.ID)}
		}
		if _, exists := customIDs[id]; exists {
			return MaterializedCatalog{}, &CollisionError{SkillID: string(skill.ID)}
		}
		if err := add(id, ir.AssetSkill, customSkillSourcePath(skill.ID), skill.Content, true, 3500); err != nil {
			return MaterializedCatalog{}, fmt.Errorf("catalog custom skill %q: %w", skill.ID, err)
		}
		customIDs[id] = struct{}{}
	}
	slices.SortFunc(specs, func(a, b ir.AssetSpec) int { return strings.Compare(string(a.ID), string(b.ID)) })
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: specs}
	if err := catalog.Validate(); err != nil {
		return MaterializedCatalog{}, err
	}
	return MaterializedCatalog{Catalog: catalog, Contents: contents, Generated: generated, GeneratorVersion: generatorVersion, SourceFingerprint: policy.SourceFingerprint}, nil
}

// Fingerprint returns the stable identity of the ordered catalog metadata.
func (c MaterializedCatalog) Fingerprint() string {
	data, _ := json.Marshal(c.Catalog)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
