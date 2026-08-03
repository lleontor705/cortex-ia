package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/backup"
	atomicfile "github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	sddmerge "github.com/lleontor705/cortex-ia/internal/components/sdd/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var (
	ErrRollbackConflict = errors.New("rollback blocked by managed content conflict")
	ErrRollbackDoctor   = errors.New("restored bundle failed doctor")
)

// RollbackConflict retains the prior, installed, and current versions. The
// live target remains unchanged whenever any conflict is reported.
type RollbackConflict struct {
	Path         string
	SemanticID   ir.SemanticID
	Prior        []byte
	Installed    []byte
	Current      []byte
	PriorRef     string
	InstalledRef string
	CurrentRef   string
}

type RollbackResult struct {
	Restored     []string
	Conflicts    []RollbackConflict
	DoctorPassed bool
	Findings     []string
}

type rollbackMutation struct {
	path    string
	content []byte
	mode    fs.FileMode
	existed bool
}

// Rollback restores the selected verified pre-install snapshot. Managed assets
// are merged from (installed,current,prior), while ownership and compatibility
// metadata are restored exactly. All merges are preflighted before mutation so
// one conflict cannot cause a partial rollback.
func Rollback(receipt Receipt, doctor func() error) (RollbackResult, error) {
	var result RollbackResult
	if !receipt.RestoreAvailable || !receipt.BackupVerified || receipt.Backup.ID == "" {
		return result, errors.New("install receipt has no verified restoration")
	}
	if err := backup.Verify(receipt.Backup); err != nil {
		return result, fmt.Errorf("verify rollback backup: %w", err)
	}

	installed := make(map[string]InstalledAsset, len(receipt.Installed))
	for _, asset := range receipt.Installed {
		installed[filepath.Clean(asset.OriginalPath)] = asset
	}
	mutations := make([]rollbackMutation, 0, len(receipt.Backup.Entries))
	for _, entry := range receipt.Backup.Entries {
		prior, err := snapshotContent(entry)
		if err != nil {
			return result, err
		}
		asset, managed := installed[filepath.Clean(entry.OriginalPath)]
		if !managed {
			mutations = append(mutations, rollbackMutation{path: entry.OriginalPath, content: prior, mode: os.FileMode(entry.Mode), existed: entry.Existed})
			continue
		}

		current, _, err := readOptional(entry.OriginalPath)
		if err != nil {
			return result, err
		}
		merged, conflict, err := mergeRollbackAsset(asset, prior, current)
		if err != nil {
			return result, err
		}
		if conflict != nil {
			result.Conflicts = append(result.Conflicts, *conflict)
			continue
		}
		mutations = append(mutations, rollbackMutation{path: entry.OriginalPath, content: merged, mode: os.FileMode(entry.Mode), existed: entry.Existed || len(merged) != 0})
	}
	if len(result.Conflicts) != 0 {
		slices.SortFunc(result.Conflicts, func(left, right RollbackConflict) int { return strings.Compare(left.Path, right.Path) })
		return result, ErrRollbackConflict
	}

	for _, mutation := range mutations {
		if !mutation.existed {
			if err := os.Remove(mutation.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return result, fmt.Errorf("remove rollback target %q: %w", mutation.path, err)
			}
		} else {
			mode := mutation.mode.Perm()
			if mode == 0 {
				mode = 0o600
			}
			if _, err := atomicfile.WriteFileAtomic(mutation.path, mutation.content, mode); err != nil {
				return result, fmt.Errorf("restore rollback target %q: %w", mutation.path, err)
			}
			if err := os.Chmod(mutation.path, mode); err != nil {
				return result, fmt.Errorf("restore rollback mode %q: %w", mutation.path, err)
			}
		}
		result.Restored = append(result.Restored, mutation.path)
		if bytes.Contains(mutation.content, []byte("agent-mailbox")) {
			result.Findings = append(result.Findings, "restored Mailbox registration is legacy/unqualified; reload or restart the runtime")
		}
	}
	if doctor != nil {
		if err := doctor(); err != nil {
			return result, errors.Join(ErrRollbackDoctor, err)
		}
		result.DoctorPassed = true
	}
	return result, nil
}

// BuildInversePlan constructs the exact receipt-backed inverse without
// mutating targets. It preflights every current managed value through the same
// three-way merge used by Rollback and returns conflicts instead of guessing.
func BuildInversePlan(receipt Receipt) (Plan, error) {
	if receipt.ReceiptSHA256 != "" {
		if err := ValidateReceipt(receipt); err != nil {
			return Plan{}, err
		}
	}
	if !receipt.RestoreAvailable || !receipt.BackupVerified {
		return Plan{}, errors.New("install receipt has no verified restoration")
	}
	if err := backup.Verify(receipt.Backup); err != nil {
		return Plan{}, fmt.Errorf("verify inverse-plan backup: %w", err)
	}
	entries := make(map[string]backup.ManifestEntry, len(receipt.Backup.Entries))
	for _, entry := range receipt.Backup.Entries {
		if err := mcpinject.ValidateRetirementPath(entry.OriginalPath); err != nil {
			return Plan{}, err
		}
		for _, protected := range receipt.ProtectedPaths {
			if filepath.Clean(protected) == filepath.Clean(entry.OriginalPath) {
				return Plan{}, fmt.Errorf("inverse plan targets protected path %q", entry.OriginalPath)
			}
		}
		entries[filepath.Clean(entry.OriginalPath)] = entry
	}

	plan := Plan{Profile: "receipt-inverse", ProtectedPaths: slices.Clone(receipt.ProtectedPaths)}
	for _, asset := range receipt.Installed {
		entry, ok := entries[filepath.Clean(asset.OriginalPath)]
		if !ok {
			return Plan{}, fmt.Errorf("inverse plan has no backup entry for %q", asset.Path)
		}
		prior, err := snapshotContent(entry)
		if err != nil {
			return Plan{}, err
		}
		current, currentExists, err := readOptional(entry.OriginalPath)
		if err != nil {
			return Plan{}, err
		}
		merged, rollbackConflict, err := mergeRollbackAsset(asset, prior, current)
		if err != nil {
			return Plan{}, err
		}
		if rollbackConflict != nil {
			plan.Conflicts = append(plan.Conflicts, PlanConflict{
				Path: asset.Path, SemanticID: asset.SemanticID, State: OwnershipCustomized,
				Reason:      "current managed bytes conflict with receipt inverse",
				CurrentHash: SHA256(current), DesiredHash: SHA256(prior),
			})
			continue
		}

		switch {
		case entry.Existed && currentExists:
			plan.Updates = append(plan.Updates, newEffect(asset.Path, asset.SemanticID, current, merged, os.FileMode(entry.Mode), os.FileMode(entry.Mode)))
		case entry.Existed && !currentExists:
			plan.Creates = append(plan.Creates, newEffect(asset.Path, asset.SemanticID, nil, prior, 0, os.FileMode(entry.Mode)))
		case !entry.Existed && currentExists:
			plan.Deletes = append(plan.Deletes, newEffect(asset.Path, asset.SemanticID, current, nil, 0o600, 0))
		}
		plan.Backup.Paths = append(plan.Backup.Paths, asset.Path)
	}
	normalizePlan(&plan)
	slices.Sort(plan.Backup.Paths)
	plan.Backup.Paths = slices.Compact(plan.Backup.Paths)
	plan.Backup.Required = len(plan.Backup.Paths) != 0
	plan.Fingerprint = FingerprintPlan(plan)
	return plan, nil
}

func mergeRollbackAsset(asset InstalledAsset, prior, current []byte) ([]byte, *RollbackConflict, error) {
	installed := asset.Content
	if !asset.Existed {
		installed = nil
	}
	mergeID := string(asset.SemanticID)
	if mergeID == "" {
		mergeID = asset.Path
	}
	merged, err := sddmerge.Merge(current, []sddmerge.ManagedRegion{{
		SemanticID: mergeID, Start: 0, End: len(current), RecordedBase: installed, Generated: prior,
	}})
	if err != nil {
		return nil, nil, fmt.Errorf("merge rollback target %q: %w", asset.Path, err)
	}
	if len(merged.Conflicts) == 0 {
		return merged.Content, nil, nil
	}
	conflict := merged.Conflicts[0]
	path := asset.Path
	if path == "" {
		path = asset.OriginalPath
	}
	return nil, &RollbackConflict{
		Path: path, SemanticID: asset.SemanticID,
		Prior: bytes.Clone(prior), Installed: bytes.Clone(installed), Current: bytes.Clone(current),
		PriorRef: conflict.GeneratedRef, InstalledRef: conflict.RecordedBaseRef, CurrentRef: conflict.CurrentRef,
	}, nil
}

func snapshotContent(entry backup.ManifestEntry) ([]byte, error) {
	if !entry.Existed {
		return nil, nil
	}
	content, err := os.ReadFile(entry.SnapshotPath)
	if err != nil {
		return nil, fmt.Errorf("read rollback snapshot %q: %w", entry.SnapshotPath, err)
	}
	return content, nil
}

func readOptional(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read rollback target %q: %w", path, err)
	}
	return content, true, nil
}
