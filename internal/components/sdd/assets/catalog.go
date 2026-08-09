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
)

var canonicalSkills = []string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize", "debate", "parallel-dispatch"}
var rootModules = []string{"routing-and-risk", "contracts-and-thresholds", "recovery-and-reflection", "parallel-apply", "memory-and-state", "model-routing"}

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

// BuildOperationalCatalog materializes the embedded retained operational set.
func BuildOperationalCatalog() (MaterializedCatalog, error) {
	return BuildOperationalCatalogFromFS(embedded.FS)
}

// BuildOperationalCatalogFromFS is injectable for deterministic catalog tests.
func BuildOperationalCatalogFromFS(source fs.FS) (MaterializedCatalog, error) {
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
	root, err := read("generic/sdd-orchestrator-root-index.md")
	if err != nil {
		return MaterializedCatalog{}, err
	}
	if err := add("asset/root-index", ir.AssetRootIndex, "generic/sdd-orchestrator-root-index.md", root, true, 1500); err != nil {
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
	for _, module := range rootModules {
		path := "generic/sdd-root/" + module + ".md"
		content, readErr := read(path)
		if readErr != nil {
			return MaterializedCatalog{}, readErr
		}
		if err := add(ir.SemanticID("asset/root-module/"+module), ir.AssetRootModule, path, content, true, 1000); err != nil {
			return MaterializedCatalog{}, err
		}
	}
	sharedPath := "skills/_shared/sdd-phase-contract.md"
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
		path := "generic/profiles/" + profile + ".md"
		content, readErr := read(path)
		if readErr != nil {
			return MaterializedCatalog{}, readErr
		}
		specID := ir.SemanticID("asset/profile-overlay/" + profile)
		if err := add(ir.SemanticID("asset/profile-overlay/"+profile), ir.AssetProfileOverlay, path, content, true, 800); err != nil {
			return MaterializedCatalog{}, err
		}
		for i := range specs {
			if specs[i].ID == specID {
				specs[i].Profiles = []ir.SemanticID{ir.SemanticID(profile)}
			}
		}
	}
	quality := []byte("# QualityPlan template\n\nApply the selected QualityPlan and record evidence before handoff.\n")
	if err := add("asset/quality-template", ir.AssetQualityTemplate, "generated/quality-plan-template.md", quality, true, 800); err != nil {
		return MaterializedCatalog{}, err
	}
	for _, command := range embedded.CommandAssetSpecs() {
		content, readErr := read(command.SourcePath)
		if readErr != nil {
			return MaterializedCatalog{}, readErr
		}
		if err := add(command.ID, command.Class, command.SourcePath, content, command.Required, command.MaxTokens); err != nil {
			return MaterializedCatalog{}, err
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
