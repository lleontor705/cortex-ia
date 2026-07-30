// Package cutover coordinates the one-way major-version replacement of
// generated repository assets. It plans first, writes only the disclosed plan,
// and deliberately has no runtime-session state input or migration authority.
package cutover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	phaseassets "github.com/lleontor705/cortex-ia/internal/assets/schemas/generator"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

const (
	// ManifestPath is the deterministic inventory installed with every managed
	// major-boundary bundle.
	ManifestPath = "schemas/generated-assets.manifest.json"
	manifestID   = ir.SemanticID("asset/manifest/generated-assets")
)

// Request contains only generated-asset migration inputs. Runtime execution
// and active-session state are intentionally absent; ActiveSession controls a
// warning only.
type Request struct {
	Root               string
	BackupRoot         string
	Workflow           ir.WorkflowIR
	Options            phaseassets.Options
	GeneratorVersion   string
	Managed            []install.ManagedAsset
	CompatibilityPaths []string
	ActiveSession      bool
	PreDoctor          func() error
	StaticConformance  func(renderers.Bundle) error
	PostDoctor         func(renderers.Bundle) error
}

// Prepared is the immutable output of the mandatory dry-run boundary. Apply
// accepts this exact value and never recomputes a second plan.
type Prepared struct {
	Plan    install.Plan
	Bundle  renderers.Bundle
	Warning string

	root             string
	backupRoot       string
	generatorVersion string
	postDoctor       func(renderers.Bundle) error
	prepared         bool
}

type Result struct {
	Bundle  renderers.Bundle
	Receipt install.Receipt
	Warning string
}

type manifest struct {
	SchemaVersion    string          `json:"schema_version"`
	GeneratorVersion string          `json:"generator_version"`
	WorkflowID       ir.SemanticID   `json:"workflow_id"`
	WorkflowVersion  ir.Version      `json:"workflow_version"`
	IRVersion        ir.Version      `json:"ir_version"`
	Profile          string          `json:"profile"`
	BundleSHA256     string          `json:"bundle_sha256"`
	Assets           []manifestAsset `json:"assets"`
}

type manifestAsset struct {
	Path       string        `json:"path"`
	SemanticID ir.SemanticID `json:"semantic_id"`
	SHA256     string        `json:"sha256"`
	Mode       uint32        `json:"mode"`
}

// Prepare performs generation, pre-cutover doctor, static conformance, and the
// exact read-only install plan. A conflict remains in the returned plan so the
// dry-run can disclose all three-way ownership evidence without mutation.
func Prepare(request Request) (Prepared, error) {
	if strings.TrimSpace(request.Root) == "" || strings.TrimSpace(request.BackupRoot) == "" {
		return Prepared{}, errors.New("cutover target root and backup root are required")
	}
	if _, err := ir.ParseVersion(request.GeneratorVersion); err != nil {
		return Prepared{}, fmt.Errorf("cutover generator version: %w", err)
	}
	if request.PreDoctor == nil || request.StaticConformance == nil || request.PostDoctor == nil {
		return Prepared{}, errors.New("cutover requires pre/post doctor and static conformance checks")
	}
	if err := request.PreDoctor(); err != nil {
		return Prepared{}, fmt.Errorf("pre-cutover doctor: %w", err)
	}
	generated, err := phaseassets.Generate(request.Workflow, request.Options)
	if err != nil {
		return Prepared{}, fmt.Errorf("generate major-boundary assets: %w", err)
	}
	bundle, err := generatedBundle(request, generated)
	if err != nil {
		return Prepared{}, err
	}
	if err := request.StaticConformance(cloneBundle(bundle)); err != nil {
		return Prepared{}, fmt.Errorf("pre-cutover static conformance: %w", err)
	}
	compatibility := append(slices.Clone(request.CompatibilityPaths), ownershipPaths(bundle)...)
	plan, err := install.NewPlanner(request.Root).Plan(install.PlanRequest{
		Bundle: bundle, Managed: slices.Clone(request.Managed), Profile: string(request.Options.Profile),
		CompatibilityMetadata: compatibility,
	})
	if err != nil {
		return Prepared{}, fmt.Errorf("plan major-boundary cutover: %w", err)
	}

	warning := ""
	if request.ActiveSession {
		warning = "Generated assets changed during an active session; reload the active session before using the new workflow contracts."
	}
	return Prepared{
		Plan: plan, Bundle: cloneBundle(bundle), Warning: warning,
		root: request.Root, backupRoot: request.BackupRoot, generatorVersion: request.GeneratorVersion,
		postDoctor: request.PostDoctor, prepared: true,
	}, nil
}

// Apply writes the exact dry-run plan, persists ownership sidecars and merge
// bases, then runs post-cutover doctor. Any failure after backup creation keeps
// the receipt restorable; conflicts fail before the first write.
func Apply(prepared Prepared) (Result, error) {
	result := Result{Bundle: cloneBundle(prepared.Bundle), Warning: prepared.Warning}
	if !prepared.prepared {
		return result, errors.New("cutover apply requires a successful dry-run preparation")
	}
	if prepared.Plan.HasBlockingConflicts() {
		return result, fmt.Errorf("cutover blocked by %d customization conflict(s)", len(prepared.Plan.Conflicts))
	}
	receipt, err := install.NewApplier(prepared.root, prepared.backupRoot).Apply(prepared.Plan)
	result.Receipt = receipt
	if err != nil {
		return result, fmt.Errorf("apply major-boundary bundle: %w", err)
	}
	if hasMutations(prepared.Plan) {
		store := install.NewOwnershipStore(prepared.root)
		for _, asset := range prepared.Bundle.Assets {
			installedContent, contentErr := plannedInstalledContent(prepared.root, prepared.Plan, asset)
			if contentErr != nil {
				return result, contentErr
			}
			metadata, metadataErr := install.NewOwnership(asset.Path, prepared.generatorVersion, asset.SemanticID, asset.Content, installedContent)
			if metadataErr != nil {
				return result, fmt.Errorf("build ownership for %q: %w", asset.Path, metadataErr)
			}
			if metadataErr = store.Write(metadata, asset.Content); metadataErr != nil {
				return result, fmt.Errorf("persist ownership for %q: %w", asset.Path, metadataErr)
			}
		}
	}
	if err := prepared.postDoctor(cloneBundle(prepared.Bundle)); err != nil {
		return result, fmt.Errorf("post-cutover doctor: %w", err)
	}
	return result, nil
}

func generatedBundle(request Request, generated []phaseassets.Asset) (renderers.Bundle, error) {
	assets := make([]renderers.Asset, 0, len(generated)+1)
	entries := make([]manifestAsset, 0, len(generated))
	for _, generatedAsset := range generated {
		mode := fs.FileMode(0o644)
		asset := renderers.Asset{
			Path: generatedAsset.Path, SemanticID: generatedAsset.SemanticID,
			Kind: kindForPath(generatedAsset.Path), Content: slices.Clone(generatedAsset.Content), Mode: mode,
		}
		assets = append(assets, asset)
		entries = append(entries, manifestAsset{
			Path: asset.Path, SemanticID: asset.SemanticID, SHA256: install.SHA256(asset.Content), Mode: uint32(mode.Perm()),
		})
	}
	slices.SortFunc(entries, func(left, right manifestAsset) int { return strings.Compare(left.Path, right.Path) })
	fingerprintInput, err := json.Marshal(entries)
	if err != nil {
		return renderers.Bundle{}, fmt.Errorf("marshal cutover fingerprint: %w", err)
	}
	digest := sha256.Sum256(fingerprintInput)
	document := manifest{
		SchemaVersion: "1.0.0", GeneratorVersion: request.GeneratorVersion,
		WorkflowID: request.Workflow.ID, WorkflowVersion: request.Workflow.Version, IRVersion: request.Workflow.SchemaVersion,
		Profile: string(request.Options.Profile), BundleSHA256: hex.EncodeToString(digest[:]), Assets: entries,
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return renderers.Bundle{}, fmt.Errorf("marshal generated-assets manifest: %w", err)
	}
	assets = append(assets, renderers.Asset{
		Path: ManifestPath, SemanticID: manifestID, Kind: renderers.AssetSchema, Content: append(content, '\n'), Mode: 0o644,
	})
	slices.SortFunc(assets, func(left, right renderers.Asset) int {
		if difference := strings.Compare(left.Path, right.Path); difference != 0 {
			return difference
		}
		return strings.Compare(string(left.SemanticID), string(right.SemanticID))
	})
	return renderers.Bundle{Assets: assets}, nil
}

func kindForPath(assetPath string) renderers.AssetKind {
	switch {
	case strings.HasPrefix(assetPath, "skills/"):
		return renderers.AssetSkill
	case strings.HasPrefix(assetPath, "opencode/commands/"):
		return renderers.AssetCommand
	case strings.HasPrefix(assetPath, "schemas/fixtures/"):
		return renderers.AssetFixture
	default:
		return renderers.AssetSchema
	}
}

func ownershipPaths(bundle renderers.Bundle) []string {
	paths := make([]string, 0, len(bundle.Assets)*2)
	for _, asset := range bundle.Assets {
		paths = append(paths, asset.Path+".cortex-ia.base", asset.Path+".cortex-ia.json")
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func hasMutations(plan install.Plan) bool {
	return len(plan.Creates)+len(plan.Updates)+len(plan.Deletes) != 0
}

func cloneBundle(bundle renderers.Bundle) renderers.Bundle {
	assets := make([]renderers.Asset, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		assets[index] = asset
		assets[index].Content = slices.Clone(asset.Content)
		assets[index].Permissions = slices.Clone(asset.Permissions)
		assets[index].Extensions = slices.Clone(asset.Extensions)
	}
	return renderers.Bundle{Assets: assets}
}

func plannedInstalledContent(root string, plan install.Plan, asset renderers.Asset) ([]byte, error) {
	for _, effects := range [][]install.Effect{plan.Creates, plan.Updates} {
		for _, effect := range effects {
			if effect.Path == asset.Path {
				return slices.Clone(effect.Content), nil
			}
		}
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(asset.Path)))
	if err != nil {
		return nil, fmt.Errorf("read unchanged managed asset %q: %w", asset.Path, err)
	}
	return content, nil
}
