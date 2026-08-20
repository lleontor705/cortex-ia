package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// planCommon resolves the home, derives the desired asset set from the
// embedded inventory and the OpenCode layout, and loads the ownership
// context (agreed v2 metadata or a read-only legacy migration assessment).
// It performs reads only.
func planCommon(req Request) (*Plan, error) {
	if strings.TrimSpace(req.HomeDir) == "" {
		return nil, errors.New("home directory is required: the engine never falls back to the process home")
	}
	home, err := filepath.Abs(req.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	home = filepath.Clean(home)
	root, err := opencodeRootAbs(home)
	if err != nil {
		return nil, err
	}

	inventory, err := assets.Inventory()
	if err != nil {
		return nil, fmt.Errorf("inventory embedded assets: %w", err)
	}
	native := make([]assets.File, 0, len(inventory))
	for _, file := range inventory {
		if opencode.IsNativeAsset(file) {
			native = append(native, file)
		}
	}
	mappings, err := opencode.MapAssets(native)
	if err != nil {
		return nil, fmt.Errorf("map OpenCode destinations: %w", err)
	}

	plan := &Plan{
		HomeDir:      home,
		OpencodeRoot: root,
	}

	metaLoad := state.LoadMetadataV2(home)
	lockLoad := state.LoadLockV2(home)
	plan.MetadataPresence = metaLoad.Presence
	plan.LockPresence = lockLoad.Presence

	switch {
	case metaLoad.Presence == state.PresenceMalformed:
		return nil, fmt.Errorf("state metadata is malformed: %s", metaLoad.Detail)
	case lockLoad.Presence == state.PresenceMalformed:
		return nil, fmt.Errorf("lock metadata is malformed: %s", lockLoad.Detail)
	case metaLoad.Presence == state.PresenceV2 && lockLoad.Presence != state.PresenceV2:
		return nil, errors.New("v2 state without an agreeing v2 lock: fail closed; recover the lock from the last verified backup or rerun sync after repair")
	case metaLoad.Presence != state.PresenceV2 && lockLoad.Presence == state.PresenceV2:
		return nil, errors.New("v2 lock without an agreeing v2 state: fail closed; recover the state from the last verified backup or rerun sync after repair")
	case metaLoad.Presence == state.PresenceV2:
		if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
			return nil, fmt.Errorf("state/lock disagreement: %w", err)
		}
		plan.Metadata = metaLoad.Metadata
		plan.mappings = mappings
		plan.evidence = ownershipEvidence(plan.Metadata)
	default:
		decision := state.AssessMigration(home, root, req.now())
		if decision.Blocker != nil {
			return nil, fmt.Errorf("legacy metadata blocks planning: %s; %s (evidence: %s)",
				decision.Blocker.Reason, decision.Blocker.Remediation,
				strings.Join(decision.Blocker.Evidence, ", "))
		}
		plan.mappings = mappings
		plan.Migration = &decision
	}
	return plan, nil
}

// ownershipEvidence converts recorded MCP ownership into manager records.
// The absolute config path is reconstructed from the recorded OpenCode root
// so accreditation stays bound to one file.
func ownershipEvidence(meta state.MetadataV2) []mcpmanager.OwnershipRecord {
	records := make([]mcpmanager.OwnershipRecord, 0, len(meta.MCPs))
	for _, mcp := range meta.MCPs {
		if mcp.Ownership != state.OwnershipManaged {
			continue
		}
		records = append(records, mcpmanager.OwnershipRecord{
			Name:       mcp.Name,
			Digest:     mcp.SemanticDigest,
			ConfigPath: filepath.Join(meta.OpencodeRoot, filepath.FromSlash(mcp.ConfigPath)),
		})
	}
	return records
}

// planAssetEffects derives the copy/merge effects for every mapped embedded
// asset by inspecting the current destinations. Reads only: the config merge
// is computed in memory so a converged config plans as a no-op. An explicit
// overwrite authorization converts unmanaged conflicts into EffectOverwrite;
// apply still captures and verifies a restorable backup first.
func planAssetEffects(plan *Plan, overwrite bool) error {
	recorded := artifactIndex(plan.Metadata.Artifacts)
	home := plan.HomeDir

	for _, mapping := range plan.mappings {
		rel := mapping.Dest
		abs := filepath.Join(home, filepath.FromSlash(rel))
		exists, digest, err := inspectFileTarget(abs)
		if err != nil {
			return fmt.Errorf("inspect destination %q: %w", rel, err)
		}
		effect := Effect{
			Dest: rel, Source: mapping.Source, SourceSHA: mapping.SHA256,
			CurrentSHA: digest, PriorExists: exists,
		}
		switch {
		case !exists:
			effect.Kind = EffectCreate
		case digest == mapping.SHA256:
			effect.Kind = EffectNoop
		case mapping.Kind == assets.KindConfig:
			effect.Kind = EffectSafeMerge
			converged, reason, err := configMergeConverged(abs, mapping.Source)
			if err != nil {
				plan.Conflicts = append(plan.Conflicts, Conflict{
					Target: rel, Kind: ConflictMalformedConfig, Reason: reason,
				})
				continue
			}
			if converged {
				effect.Kind = EffectNoop
				effect.Reason = "settings template already merged"
			}
		default:
			artifact, owned := recorded[mapping.Source]
			stillOwned := owned && artifact.Ownership == state.OwnershipManaged && artifact.Digest == digest
			switch {
			case stillOwned:
				effect.Kind = EffectManagedUpdate
			case overwrite:
				effect.Kind = EffectOverwrite
				effect.Reason = "explicit overwrite authorization"
			default:
				kind, reason := ConflictUnmanagedExisting, "destination holds a file with no recorded cortex-ia ownership"
				if owned {
					kind, reason = ConflictUnmanagedDrift, "managed file no longer matches its recorded digest; cortex-ia no longer owns it"
				}
				plan.Conflicts = append(plan.Conflicts, Conflict{
					Target: rel, Kind: kind, Reason: reason, OverwriteAuthorized: true,
				})
				continue
			}
		}
		plan.Effects = append(plan.Effects, effect)
	}
	return nil
}

// configMergeConverged computes the template merge in memory and reports
// whether the existing settings file already carries every template key.
func configMergeConverged(absConfig, source string) (bool, string, error) {
	base, err := os.ReadFile(absConfig)
	if err != nil {
		return false, "", err
	}
	overlay, err := settingsOverlay(source)
	if err != nil {
		return false, "", err
	}
	merged, err := filemerge.MutateJSONDocument(absConfig, base, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return false, fmt.Sprintf("existing settings file is not a valid JSON(C) object: %v", err), nil
	}
	return bytes.Equal(base, merged), "", nil
}

// settingsOverlay reads the embedded settings template and re-encodes it as
// strict JSON: the template itself is authorable JSONC (comments, trailing
// commas), while the filemerge overlay contract accepts strict JSON only.
// The merge stays value-level, so re-encoding never loses comments in the
// destination file.
func settingsOverlay(source string) ([]byte, error) {
	raw, err := assets.ReadBytes(source)
	if err != nil {
		return nil, err
	}
	decoded, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("embedded settings template is not decodable JSONC: %w", err)
	}
	strict, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("encode settings overlay: %w", err)
	}
	return strict, nil
}

// planMCPEffects derives the managed MCP effects from the manager catalog
// and the accredited ownership evidence. Reads only.
func planMCPEffects(plan *Plan, req Request) error {
	manager := mcpmanager.New(plan.HomeDir)
	listing, err := manager.List(plan.evidence)
	if err != nil {
		var conflict *mcpmanager.ConflictError
		if errors.As(err, &conflict) && conflict.Kind == mcpmanager.ConflictMalformed {
			return fmt.Errorf("OpenCode config is malformed: %s", conflict.Detail)
		}
		return err
	}
	configRel := relFromHome(plan.HomeDir, manager.ConfigPath())
	plan.MCPConfigRel = configRel

	selection := req.mcpSelection()
	statuses := make(map[string]mcpmanager.EntryStatus, len(listing.Entries))
	for _, report := range listing.Entries {
		statuses[report.Name] = report.Status
	}
	presetNames := make([]string, 0, len(selection))
	for name := range selection {
		presetNames = append(presetNames, name)
	}
	sort.Strings(presetNames)
	for _, name := range presetNames {
		desired := selection[name]
		status := statuses[name]
		switch {
		case desired && status == mcpmanager.StatusManaged:
			plan.Effects = append(plan.Effects, Effect{Kind: EffectMCPNoop, Dest: name})
		case desired && status == mcpmanager.StatusAbsent:
			plan.Effects = append(plan.Effects, Effect{Kind: EffectMCPAdd, Dest: name})
		case desired:
			plan.Conflicts = append(plan.Conflicts, Conflict{
				Target: name, Kind: ConflictMCP,
				Reason: fmt.Sprintf("MCP entry %q is user-owned (%s); the manager never appropriates or overwrites it", name, status),
			})
		case status == mcpmanager.StatusManaged:
			plan.Effects = append(plan.Effects, Effect{Kind: EffectMCPRemove, Dest: name})
		}
	}
	return nil
}

// planStaleEffects derives sync deletions for recorded artifacts whose
// embedded source disappeared. A stale file is deleted only when cortex-ia
// still owns it: recorded managed ownership and an intact digest. Reads only.
func planStaleEffects(plan *Plan) error {
	if plan.MetadataPresence != state.PresenceV2 {
		return nil
	}
	desired := desiredSourceSet(plan.mappings)
	home := plan.HomeDir

	for _, artifact := range plan.Metadata.Artifacts {
		if artifact.Ownership != state.OwnershipManaged {
			continue
		}
		if _, wanted := desired[artifact.Path]; wanted {
			continue
		}
		rel := path.Join(filepath.ToSlash(".config/opencode"), artifact.Path)
		abs := filepath.Join(home, filepath.FromSlash(rel))
		exists, digest, err := inspectFileTarget(abs)
		if err != nil {
			return fmt.Errorf("inspect stale destination %q: %w", rel, err)
		}
		if !exists || digest == artifact.Digest {
			plan.Effects = append(plan.Effects, Effect{
				Kind: EffectDelete, Dest: rel, CurrentSHA: digest, PriorExists: exists,
				SourceSHA: artifact.Digest, Reason: "recorded artifact has no embedded source",
			})
			continue
		}
		plan.Conflicts = append(plan.Conflicts, Conflict{
			Target: rel, Kind: ConflictStaleDrift,
			Reason: "stale artifact was modified after installation; it is kept and never deleted",
		})
	}
	return nil
}

func desiredSourceSet(mappings []opencode.Mapping) map[string]struct{} {
	set := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		set[mapping.Source] = struct{}{}
	}
	return set
}

// desiredArtifacts projects the current mappings onto the v2 artifact model.
func desiredArtifacts(plan *Plan) map[string]state.ArtifactV2 {
	desired := make(map[string]state.ArtifactV2, len(plan.mappings))
	for _, mapping := range plan.mappings {
		desired[mapping.Source] = state.ArtifactV2{
			Path:      mapping.Source,
			Kind:      artifactKind(mapping.Kind),
			Origin:    "embedded",
			Digest:    mapping.SHA256,
			Ownership: state.OwnershipManaged,
		}
	}
	return desired
}

// artifactKind translates a structural asset kind into the v2 metadata
// semantic classification.
func artifactKind(kind assets.Kind) state.ArtifactKind {
	switch kind {
	case assets.KindSkill:
		return state.KindSkill
	case assets.KindAgent:
		return state.KindAgent
	case assets.KindCommand:
		return state.KindCommand
	case assets.KindConfig:
		return state.KindMCPConfig
	case assets.KindAgentsDoc:
		return state.KindPrompt
	default:
		return state.KindOther
	}
}

// inspectFileTarget reports whether the path holds a regular file and its
// lowercase hex sha256. Directories, symlinks, and other non-regular shapes
// fail closed.
func inspectFileTarget(abs string) (bool, string, error) {
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if info.IsDir() {
		return true, "", fmt.Errorf("destination is a directory, not a regular file")
	}
	if !info.Mode().IsRegular() {
		return true, "", fmt.Errorf("destination is not a regular file")
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return true, "", err
	}
	return true, journalSHA256(content), nil
}

// relFromHome converts an absolute path under the home into a slash-relative
// path for plans and receipts.
func relFromHome(home, abs string) string {
	relative, err := filepath.Rel(home, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(relative)
}
