package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sddmerge "github.com/lleontor705/cortex-ia/internal/components/sdd/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

// ManagedAsset is the install-state evidence from the previous successful
// installation. Base and Ownership are used to distinguish clean managed
// content from customization or corrupt ownership evidence.
type ManagedAsset struct {
	Path            string
	Ownership       Ownership
	Base            []byte
	Mode            fs.FileMode
	OwnershipPath   string
	BasePath        string
	OwnershipSHA256 string
	BaseSHA256      string
	LegacyOwnership bool
}

// PlanRequest contains all inputs that affect persistent installation. The
// same request is planned for dry-run and install; execution mode cannot alter
// the computed effects.
type PlanRequest struct {
	Bundle       renderers.Bundle
	Managed      []ManagedAsset
	Profile      string
	Degradations []string
	// CompatibilityMetadata lists relative compatibility/manifest records
	// whose prior state must be restorable with any managed mutation.
	CompatibilityMetadata    []string
	ForgeSpecMode            string
	CapabilitySnapshotSHA256 string
	RequiredAssets           []ir.AssetSpec
	OwnershipMarkers         bool
	GeneratorVersion         string
	Metadata                 json.RawMessage
}

// Effect is one apply-ready managed filesystem mutation.
type Effect struct {
	Path            string
	SemanticID      ir.SemanticID
	BeforeSHA256    string
	AfterSHA256     string
	BeforeMode      fs.FileMode
	AfterMode       fs.FileMode
	Content         []byte
	OwnershipSHA256 string
	BaseSHA256      string
	OwnershipPath   string
	BasePath        string
}

type PlanConflict struct {
	Path        string
	SemanticID  ir.SemanticID
	State       OwnershipState
	Reason      string
	CurrentHash string
	DesiredHash string
}

type PermissionChange struct {
	Path string
	From fs.FileMode
	To   fs.FileMode
}

type BackupScope struct {
	Required bool
	Paths    []string
}

// Plan is an exact, deterministic disclosure of all effects and blockers.
// Creates do not require backup because no prior target exists; every update
// and delete is included in Backup.
type Plan struct {
	Creates                  []Effect
	Updates                  []Effect
	Deletes                  []Effect
	Conflicts                []PlanConflict
	PermissionChanges        []PermissionChange
	Degradations             []string
	Backup                   BackupScope
	Profile                  string
	Fingerprint              string
	ProtectedPaths           []string
	ForgeSpecMode            string
	CapabilitySnapshotSHA256 string
	OwnershipMarkers         bool
	GeneratorVersion         string
	Metadata                 json.RawMessage
	Inventory                []AssetInventory
	OwnershipMigrations      []OwnershipMigration
}

// OwnershipMigration promotes selected legacy adjacent evidence to the
// canonical store. Apply verifies and backs up both locations before retiring
// either legacy file.
type OwnershipMigration struct {
	AssetPath              string
	SemanticID             ir.SemanticID
	LegacyOwnershipPath    string
	LegacyBasePath         string
	CanonicalOwnershipPath string
	CanonicalBasePath      string
	LegacyOwnershipSHA256  string
	LegacyBaseSHA256       string
	Ownership              Ownership
	Base                   []byte
}

// AssetInventory is the receipt/planner evidence for every selected asset,
// including assets that are unchanged during an idempotent reinstall.
type AssetInventory struct {
	Path       string
	SemanticID ir.SemanticID
	SHA256     string
}

func (p Plan) HasBlockingConflicts() bool { return len(p.Conflicts) != 0 }

type Planner struct{ root string }

func NewPlanner(root string) Planner { return Planner{root: root} }

// Plan computes installation effects without writing targets, ownership
// metadata, or backups. Install execution must consume this result rather than
// recomputing a second plan.
func (p Planner) Plan(request PlanRequest) (Plan, error) {
	if strings.TrimSpace(p.root) == "" {
		return Plan{}, errors.New("install planner root is required")
	}
	if strings.TrimSpace(request.Profile) == "" {
		return Plan{}, errors.New("install planner profile is required")
	}

	managed, err := indexManaged(request.Managed)
	if err != nil {
		return Plan{}, err
	}
	desired := make(map[string]struct{}, len(request.Bundle.Assets))
	plan := Plan{
		Profile: strings.TrimSpace(request.Profile), Degradations: sortedUniqueStrings(request.Degradations),
		ForgeSpecMode:            strings.TrimSpace(request.ForgeSpecMode),
		CapabilitySnapshotSHA256: strings.ToLower(strings.TrimSpace(request.CapabilitySnapshotSHA256)),
		OwnershipMarkers:         request.OwnershipMarkers,
		GeneratorVersion:         strings.TrimSpace(request.GeneratorVersion),
		Metadata:                 slices.Clone(request.Metadata),
	}
	if plan.GeneratorVersion == "" {
		plan.GeneratorVersion = "1.0.0"
	}
	for _, asset := range request.Bundle.Assets {
		if err := validateAssetPath(asset.Path); err != nil {
			return Plan{}, err
		}
		if _, duplicate := desired[asset.Path]; duplicate {
			return Plan{}, fmt.Errorf("duplicate desired asset path %q", asset.Path)
		}
		desired[asset.Path] = struct{}{}
		plan.Inventory = append(plan.Inventory, AssetInventory{Path: asset.Path, SemanticID: asset.SemanticID, SHA256: SHA256(asset.Content)})
		current, mode, exists, err := p.read(asset.Path)
		if err != nil {
			return Plan{}, err
		}
		if previous, ok := managed[asset.Path]; ok && previous.LegacyOwnership {
			migration, migrationErr := newOwnershipMigration(previous)
			if migrationErr != nil {
				return Plan{}, migrationErr
			}
			plan.OwnershipMigrations = append(plan.OwnershipMigrations, migration)
		}
		if !exists {
			plan.Creates = append(plan.Creates, newEffect(asset.Path, asset.SemanticID, nil, asset.Content, 0, asset.Mode))
			continue
		}
		desiredContent := bytes.Clone(asset.Content)
		contentChanged := !bytes.Equal(current, desiredContent)
		currentMode := mode.Perm()
		if previous, ok := managed[asset.Path]; ok && previous.Mode.Perm() != 0 {
			currentMode = previous.Mode.Perm()
		}
		modeChanged := currentMode != asset.Mode.Perm()
		if !contentChanged && !modeChanged {
			continue
		}
		if previous, ok := managed[asset.Path]; ok {
			inspection := InspectOwnership(current, &previous.Ownership, previous.Base)
			switch inspection.State {
			case OwnershipClean:
			case OwnershipCustomized:
				merged, mergeErr := sddmerge.Merge(current, []sddmerge.ManagedRegion{{
					SemanticID: string(asset.SemanticID), Start: 0, End: len(current),
					RecordedBase: previous.Base, Generated: asset.Content,
				}})
				if mergeErr != nil {
					return Plan{}, fmt.Errorf("merge customized managed asset %q: %w", asset.Path, mergeErr)
				}
				if len(merged.Conflicts) != 0 {
					inspection.Reason = "current and generated changes overlap the recorded ownership base"
					plan.Conflicts = append(plan.Conflicts, conflict(asset.Path, asset.SemanticID, inspection, current, asset.Content))
					continue
				}
				desiredContent = merged.Content
			case OwnershipUnknown, OwnershipCorrupt:
				plan.Conflicts = append(plan.Conflicts, conflict(asset.Path, asset.SemanticID, inspection, current, asset.Content))
				continue
			default:
				return Plan{}, fmt.Errorf("managed asset %q has unsupported ownership state %q", asset.Path, inspection.State)
			}
		} else if contentChanged {
			inspection := Inspection{State: OwnershipUnknown, Reason: "ownership metadata is absent"}
			plan.Conflicts = append(plan.Conflicts, conflict(asset.Path, asset.SemanticID, inspection, current, asset.Content))
			continue
		}
		effect := newEffect(asset.Path, asset.SemanticID, current, desiredContent, currentMode, asset.Mode)
		if err := p.attachOwnershipPreconditions(&effect, managed[asset.Path]); err != nil {
			return Plan{}, err
		}
		plan.Updates = append(plan.Updates, effect)
		if modeChanged {
			plan.PermissionChanges = append(plan.PermissionChanges, PermissionChange{Path: asset.Path, From: currentMode, To: asset.Mode.Perm()})
		}
	}
	if err := validateRequiredAssets(request.RequiredAssets, request.Bundle); err != nil {
		return Plan{}, err
	}

	for path, previous := range managed {
		if _, retained := desired[path]; retained {
			continue
		}
		current, mode, exists, err := p.read(path)
		if err != nil {
			return Plan{}, err
		}
		if !exists {
			continue
		}
		inspection := InspectOwnership(current, &previous.Ownership, previous.Base)
		if inspection.State != OwnershipClean {
			plan.Conflicts = append(plan.Conflicts, conflict(path, previous.Ownership.SemanticID, inspection, current, nil))
			continue
		}
		currentMode := mode.Perm()
		if previous.Mode.Perm() != 0 {
			currentMode = previous.Mode.Perm()
		}
		effect := newEffect(path, previous.Ownership.SemanticID, current, nil, currentMode, 0)
		if err := p.attachOwnershipPreconditions(&effect, previous); err != nil {
			return Plan{}, err
		}
		plan.Deletes = append(plan.Deletes, effect)
	}

	normalizePlan(&plan)
	backupPaths := make([]string, 0, len(plan.Creates)+len(plan.Updates)*3+len(plan.Deletes)*3+len(request.CompatibilityMetadata))
	for _, effect := range plan.Creates {
		backupPaths = append(backupPaths, effect.Path)
		if request.OwnershipMarkers {
			ownershipPath, basePath, pathErr := ownershipPaths(effect.Path, OwnershipScopeAsset, effect.SemanticID, false)
			if pathErr != nil {
				return Plan{}, pathErr
			}
			backupPaths = append(backupPaths, ownershipPath, basePath)
		}
	}
	for _, effect := range plan.Updates {
		backupPaths = append(backupPaths, effect.Path)
		backupPaths = append(backupPaths, ownershipBackupPaths(managed[effect.Path])...)
	}
	for _, effect := range plan.Deletes {
		backupPaths = append(backupPaths, effect.Path)
		backupPaths = append(backupPaths, ownershipBackupPaths(managed[effect.Path])...)
	}
	for _, migration := range plan.OwnershipMigrations {
		backupPaths = append(backupPaths,
			migration.LegacyOwnershipPath, migration.LegacyBasePath,
			migration.CanonicalOwnershipPath, migration.CanonicalBasePath,
		)
	}
	if len(backupPaths) != 0 {
		for _, path := range request.CompatibilityMetadata {
			if err := validateAssetPath(path); err != nil {
				return Plan{}, fmt.Errorf("compatibility metadata: %w", err)
			}
			backupPaths = append(backupPaths, path)
		}
	}
	slices.Sort(backupPaths)
	backupPaths = slices.Compact(backupPaths)
	plan.Backup = BackupScope{Required: len(backupPaths) != 0, Paths: backupPaths}
	plan.Fingerprint = FingerprintPlan(plan)
	return plan, nil
}

func validateRequiredAssets(required []ir.AssetSpec, bundle renderers.Bundle) error {
	for _, spec := range required {
		if !spec.Required {
			continue
		}
		if err := spec.Validate(); err != nil {
			return fmt.Errorf("required asset %q: %w", spec.ID, err)
		}
		found := false
		for _, asset := range bundle.Assets {
			if asset.Path != spec.SourcePath && !strings.HasSuffix(asset.Path, "/"+strings.TrimPrefix(spec.SourcePath, "/")) && asset.SemanticID != spec.ID {
				continue
			}
			found = true
			if spec.SHA256 != "" && SHA256(asset.Content) != strings.ToLower(spec.SHA256) {
				return fmt.Errorf("required asset %q content fingerprint mismatch", spec.ID)
			}
			break
		}
		if !found {
			return fmt.Errorf("required asset %q is absent from install bundle", spec.ID)
		}
	}
	return nil
}

// FingerprintPlan returns the canonical immutable plan identity. The
// fingerprint field itself is excluded so preview and apply compare the same
// exact content rather than recursively hashing the digest.
func FingerprintPlan(plan Plan) string {
	plan.Fingerprint = ""
	encoded, err := json.Marshal(plan)
	if err != nil {
		panic(fmt.Sprintf("marshal install plan fingerprint: %v", err))
	}
	return SHA256(encoded)
}

func ownershipBackupPaths(asset ManagedAsset) []string {
	if asset.Path == "" {
		return nil
	}
	if asset.OwnershipPath != "" || asset.BasePath != "" {
		return []string{asset.OwnershipPath, asset.BasePath}
	}
	paths, basePaths, err := ownershipPaths(asset.Path, asset.Ownership.Scope, asset.Ownership.SemanticID, false)
	if err != nil {
		return nil
	}
	return []string{paths, basePaths}
}

func (p Planner) attachOwnershipPreconditions(effect *Effect, asset ManagedAsset) error {
	paths := ownershipBackupPaths(asset)
	if len(paths) != 2 {
		return nil
	}
	ownership, _, ownershipExists, err := p.read(paths[0])
	if err != nil {
		return err
	}
	base, _, baseExists, err := p.read(paths[1])
	if err != nil {
		return err
	}
	// Historical in-memory ownership without persisted evidence remains
	// compatible; once either evidence file exists both become mandatory CAS.
	if !ownershipExists && !baseExists {
		return nil
	}
	if !ownershipExists || !baseExists {
		return fmt.Errorf("managed asset %q has incomplete persisted ownership evidence", asset.Path)
	}
	effect.OwnershipPath = paths[0]
	effect.BasePath = paths[1]
	effect.OwnershipSHA256 = asset.OwnershipSHA256
	if effect.OwnershipSHA256 == "" {
		effect.OwnershipSHA256 = SHA256(ownership)
	}
	effect.BaseSHA256 = asset.BaseSHA256
	if effect.BaseSHA256 == "" {
		effect.BaseSHA256 = SHA256(base)
	}
	return nil
}

func newOwnershipMigration(asset ManagedAsset) (OwnershipMigration, error) {
	if !asset.LegacyOwnership {
		return OwnershipMigration{}, errors.New("ownership migration requires legacy evidence")
	}
	if !isOpenCodeAsset(asset.Path) || asset.Ownership.AssetPath != asset.Path {
		return OwnershipMigration{}, fmt.Errorf("managed asset %q has invalid legacy ownership identity", asset.Path)
	}
	if !validSHA256(asset.OwnershipSHA256) || !validSHA256(asset.BaseSHA256) {
		return OwnershipMigration{}, fmt.Errorf("managed asset %q has invalid legacy evidence hashes", asset.Path)
	}
	if SHA256(asset.Base) != asset.Ownership.BaseSHA256 {
		return OwnershipMigration{}, fmt.Errorf("managed asset %q has corrupt legacy ownership base", asset.Path)
	}
	legacyOwnershipPath, legacyBasePath, err := ownershipPaths(asset.Path, asset.Ownership.Scope, asset.Ownership.SemanticID, true)
	if err != nil {
		return OwnershipMigration{}, err
	}
	canonicalOwnershipPath, canonicalBasePath, err := ownershipPaths(asset.Path, asset.Ownership.Scope, asset.Ownership.SemanticID, false)
	if err != nil {
		return OwnershipMigration{}, err
	}
	if asset.OwnershipPath != "" {
		legacyOwnershipPath = asset.OwnershipPath
	}
	if asset.BasePath != "" {
		legacyBasePath = asset.BasePath
	}
	return OwnershipMigration{
		AssetPath: asset.Path, SemanticID: asset.Ownership.SemanticID,
		LegacyOwnershipPath: legacyOwnershipPath, LegacyBasePath: legacyBasePath,
		CanonicalOwnershipPath: canonicalOwnershipPath, CanonicalBasePath: canonicalBasePath,
		LegacyOwnershipSHA256: asset.OwnershipSHA256, LegacyBaseSHA256: asset.BaseSHA256,
		Ownership: asset.Ownership, Base: bytes.Clone(asset.Base),
	}, nil
}

func (p Planner) read(path string) ([]byte, fs.FileMode, bool, error) {
	fullPath := filepath.Join(p.root, filepath.FromSlash(path))
	info, err := os.Lstat(fullPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, fmt.Errorf("inspect install target %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, fmt.Errorf("install target %q is not a regular file", path)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read install target %q: %w", path, err)
	}
	return content, info.Mode().Perm(), true, nil
}

func indexManaged(assets []ManagedAsset) (map[string]ManagedAsset, error) {
	result := make(map[string]ManagedAsset, len(assets))
	for _, asset := range assets {
		if err := validateAssetPath(asset.Path); err != nil {
			return nil, err
		}
		if asset.Ownership.AssetPath != asset.Path {
			return nil, fmt.Errorf("managed asset %q ownership path is %q", asset.Path, asset.Ownership.AssetPath)
		}
		if _, duplicate := result[asset.Path]; duplicate {
			return nil, fmt.Errorf("duplicate managed asset path %q", asset.Path)
		}
		copy := asset
		copy.Base = bytes.Clone(asset.Base)
		result[asset.Path] = copy
	}
	return result, nil
}

func newEffect(path string, semanticID ir.SemanticID, before, after []byte, beforeMode, afterMode fs.FileMode) Effect {
	effect := Effect{Path: path, SemanticID: semanticID, BeforeMode: beforeMode.Perm(), AfterMode: afterMode.Perm(), Content: bytes.Clone(after)}
	if before != nil {
		effect.BeforeSHA256 = SHA256(before)
	}
	if after != nil {
		effect.AfterSHA256 = SHA256(after)
	}
	return effect
}

func conflict(path string, semanticID ir.SemanticID, inspection Inspection, current, desired []byte) PlanConflict {
	result := PlanConflict{Path: path, SemanticID: semanticID, State: inspection.State, Reason: inspection.Reason, CurrentHash: SHA256(current)}
	if desired != nil {
		result.DesiredHash = SHA256(desired)
	}
	return result
}

func normalizePlan(plan *Plan) {
	slices.SortFunc(plan.Creates, compareEffects)
	slices.SortFunc(plan.Updates, compareEffects)
	slices.SortFunc(plan.Deletes, compareEffects)
	slices.SortFunc(plan.Conflicts, func(left, right PlanConflict) int { return strings.Compare(left.Path, right.Path) })
	slices.SortFunc(plan.PermissionChanges, func(left, right PermissionChange) int { return strings.Compare(left.Path, right.Path) })
	slices.SortFunc(plan.Inventory, func(left, right AssetInventory) int { return strings.Compare(left.Path, right.Path) })
	slices.SortFunc(plan.OwnershipMigrations, func(left, right OwnershipMigration) int { return strings.Compare(left.AssetPath, right.AssetPath) })
}

func compareEffects(left, right Effect) int { return strings.Compare(left.Path, right.Path) }

func sortedUniqueStrings(values []string) []string {
	result := slices.Clone(values)
	for index := range result {
		result[index] = strings.TrimSpace(result[index])
	}
	slices.Sort(result)
	return slices.Compact(result)
}
