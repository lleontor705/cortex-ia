package mcpmanager

import (
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

const (
	// mcpKey is OpenCode's native MCP registration object key.
	mcpKey = "mcp"

	// configRoot is the OpenCode global configuration directory, home-relative.
	configRoot = ".config/opencode"

	// jsoncName and jsonName are OpenCode's accepted global config files.
	jsoncName = "opencode.jsonc"
	jsonName  = "opencode.json"
)

// EntryStatus classifies one managed preset against the observed config.
type EntryStatus string

const (
	// StatusAbsent means no entry with the managed name exists.
	StatusAbsent EntryStatus = "absent"

	// StatusManaged means the entry exists, equals the managed preset, and
	// transactional metadata accredits cortex-ia ownership of it.
	StatusManaged EntryStatus = "managed"

	// StatusUnmanagedEquivalent means the entry equals a managed preset but
	// no ownership record accredits it: a user-created identical entry. It
	// is user-owned and is never rewritten or implicitly removed.
	StatusUnmanagedEquivalent EntryStatus = "unmanaged-equivalent"

	// StatusConflict means an entry exists under a managed name whose
	// content differs from the managed preset.
	StatusConflict EntryStatus = "conflict"
)

// ConflictKind distinguishes why an operation failed closed.
type ConflictKind string

const (
	// ConflictModified marks an entry under a managed name whose content
	// differs from the managed template. It is reported identically for
	// user-modified cortex-ia entries and never-installed foreign entries
	// because semantic digests cannot distinguish those histories without
	// transactional metadata; both are user-owned and untouchable.
	ConflictModified ConflictKind = "managed-name-modified"

	// ConflictUnaccredited marks an entry whose content equals the managed
	// preset while no ownership record accredits cortex-ia ownership. The
	// entry may be user-created; cortex-ia never appropriates it on Add and
	// never removes it without a matching record.
	ConflictUnaccredited ConflictKind = "unaccredited-entry"

	// ConflictUnmanaged marks a name outside the managed preset catalog.
	ConflictUnmanaged ConflictKind = "unmanaged-name"

	// ConflictMalformed marks config where "mcp" exists but is not a JSON
	// object, or the document cannot be parsed unambiguously.
	ConflictMalformed ConflictKind = "malformed-config"

	// ConflictDrifted marks a custom entry whose live full postimage no
	// longer matches the recorded mcpv2 fingerprint: URL, env/header value,
	// enabled state, type, argv, or config path drifted after cortex-ia
	// accredited it. The entry mutates nothing and the user must inspect it
	// before any destructive operation is retried.
	ConflictDrifted ConflictKind = "postimage-drift"

	// ConflictLegacyOwnership marks destructive custom accreditation backed
	// only by legacy mcpv1 identity evidence (or by a record whose mcpv2
	// fingerprint cannot be verified because the local salt is missing or
	// corrupt). Legacy records are read as incompatible ownership evidence
	// and are never silently upgraded: the fail-closed remedy is to re-run
	// the add command — which verifies full-value equality first — and then
	// remove.
	ConflictLegacyOwnership ConflictKind = "legacy-ownership"
)

// ConflictError is the typed fail-closed result for ownership and structure
// violations. Callers must treat it as "mutate nothing and report"; the
// manager guarantees the config file is never written when this is returned.
type ConflictError struct {
	Name           string
	Kind           ConflictKind
	ExpectedDigest string
	ObservedDigest string
	Detail         string
}

func (e *ConflictError) Error() string {
	switch e.Kind {
	case ConflictUnmanaged:
		return fmt.Sprintf("mcpmanager: %q is not a cortex-ia managed MCP preset; refusing to touch unmanaged configuration", e.Name)
	case ConflictUnaccredited:
		return fmt.Sprintf(
			"mcpmanager: MCP entry %q equals the managed preset (digest %s) but no ownership record accredits it; refusing to appropriate or remove unregistered configuration",
			e.Name, e.ObservedDigest,
		)
	case ConflictMalformed:
		return fmt.Sprintf("mcpmanager: malformed OpenCode MCP config: %s", e.Detail)
	case ConflictDrifted:
		return fmt.Sprintf(
			"mcpmanager: MCP entry %q drifted from the accredited mcpv2 postimage (expected fingerprint %s, observed %s); URL, env/header values, enabled, type, argv, or the config file changed; refusing to remove user-modified configuration",
			e.Name, e.ExpectedDigest, e.ObservedDigest,
		)
	case ConflictLegacyOwnership:
		if e.Detail == "" {
			e.Detail = "ownership record predates mcpv2 full-postimage fingerprints"
		}
		return fmt.Sprintf(
			"mcpmanager: MCP entry %q is accredited only by legacy ownership evidence (%s); value drift cannot be excluded, so destructive removal fails closed — inspect the entry, re-run the add command to accredit the full postimage, then remove it",
			e.Name, e.Detail,
		)
	default:
		return fmt.Sprintf(
			"mcpmanager: MCP entry %q differs from the managed template (expected digest %s, observed %s); refusing to overwrite user-modified or unmanaged configuration",
			e.Name, e.ExpectedDigest, e.ObservedDigest,
		)
	}
}

// OwnershipRecord is the transactional-metadata evidence that cortex-ia
// previously wrote one MCP entry. Records are produced by Add and must be
// persisted by the installer pipeline (state v2) after a successful apply;
// Remove and managed classification consume them as the only ownership
// proof. Evidence is bound to one config path and one semantic digest: a
// record never accredits an entry observed in a different file or with
// different content. PostImageDigest is the parallel mcpv2 full-postimage
// fingerprint ("mcpv2:<hex64>", keyed by the home's local salt): when it is
// empty the record is legacy mcpv1-only evidence and can never accredit a
// destructive custom removal.
type OwnershipRecord struct {
	Name       string
	Digest     string
	ConfigPath string
	// PostImageDigest is the recorded mcpv2 fingerprint covering URL,
	// env/header values (keyed hashes), enabled, identity, and config path.
	PostImageDigest string
}

// InvalidPresetError reports a catalog preset that is incompatible with the
// existing qualification boundary (a local server with a non-empty string
// command vector). Operations fail closed before any write.
type InvalidPresetError struct {
	Name   string
	Detail string
}

func (e *InvalidPresetError) Error() string {
	return fmt.Sprintf("mcpmanager: managed preset %q is incompatible with command qualification: %s", e.Name, e.Detail)
}

// EntryReport is the per-preset projection of one List call. It is the
// sanitized, JSON-fit projection of one MCP entry: identity-level data only.
// Env and header VALUES, URLs (which may embed credentials), and local argv
// vectors are never exposed; only variable NAMES, the entry type, and the
// secret-free semantic digest travel to callers, receipts, and JSON. Detail
// carries the typed reason of a custom-entry conflict (postimage drift or
// legacy ownership) so drift is visible without exposing any value.
type EntryReport struct {
	Name   string      `json:"name"`
	Status EntryStatus `json:"status"`
	// Digest is the observed semantic digest for present entries.
	Digest string `json:"digest,omitempty"`
	// Type is the observed entry "type" (e.g. "local", "remote").
	Type string `json:"type,omitempty"`
	// EnvNames are the observed env variable names, sorted; never values.
	EnvNames []string `json:"env_names,omitempty"`
	// HeaderNames are the observed header names, sorted; never values.
	HeaderNames []string `json:"header_names,omitempty"`
	// Detail explains a conflict for custom entries; empty otherwise.
	Detail string `json:"detail,omitempty"`
}

// ListResult describes every managed preset, every accredited custom entry,
// and the unknown entries found in the configuration. Unknown entries are
// informational only; the manager never mutates them.
type ListResult struct {
	ConfigPath string
	Entries    []EntryReport
	Unknown    []string
}

// Result describes the outcome of one Add or Remove operation.
type Result struct {
	Name       string
	ConfigPath string
	// Action is "added", "removed", "already-present", or "already-absent".
	Action string
	// Changed reports whether the config file bytes changed.
	Changed bool
	// Created reports whether the config file did not exist before.
	Created bool
	// Configured reports that the desired managed entry is present in the
	// config with freshly written or accredited ownership.
	Configured bool
	// Qualified reports that explicit qualification evidence validated the
	// configured entry during this call.
	Qualified bool
	// Installed is the success verdict: Configured AND Qualified. It is
	// never true without valid probe evidence.
	Installed bool
	// Ownership is the record to persist in transactional metadata. It is
	// nil for removals.
	Ownership *OwnershipRecord
	// Qualification carries the evaluated probe outcome for Add calls; nil
	// when no probe ran.
	Qualification *ProbeEvidence
}

// Manager owns MCP add/list/remove for exactly one OpenCode configuration
// rooted at a supplied home directory. It never reads process-global home
// state and never resolves paths outside the OpenCode config root.
type Manager struct {
	homeDir string
	// fingerprintSalt enables mcpv2 full-postimage fingerprints. When nil
	// (the plain New constructor, used by pipeline planning/apply), custom
	// listings keep the legacy semantic classification and Add produces
	// legacy records; destructive custom removal still refuses legacy
	// evidence. Service callers always supply the home's salt.
	fingerprintSalt []byte
}

// New returns a Manager for the given home directory.
func New(homeDir string) *Manager {
	return &Manager{homeDir: homeDir}
}

// NewFingerprinting returns a Manager that records and verifies mcpv2
// full-postimage fingerprints keyed by salt. The salt stays in live memory:
// it is never written to the config, receipts, or logs, and it never leaves
// the process except through the caller's own state sidecar.
func NewFingerprinting(homeDir string, fingerprintSalt []byte) *Manager {
	return &Manager{homeDir: homeDir, fingerprintSalt: fingerprintSalt}
}

// ConfigPath follows OpenCode's global load precedence: JSONC is loaded after
// JSON and therefore owns conflicting keys when both files exist, so the
// manager mutates the JSONC file whenever one is present.
func (m *Manager) ConfigPath() string {
	dir := filepath.Join(m.homeDir, filepath.FromSlash(configRoot))
	jsonc := filepath.Join(dir, jsoncName)
	if _, err := os.Stat(jsonc); err == nil {
		return jsonc
	}
	jsonFile := filepath.Join(dir, jsonName)
	if _, err := os.Stat(jsonFile); err == nil {
		return jsonFile
	}
	return jsonc
}

// Add installs the managed preset entry for name and evaluates explicit
// qualification evidence. Configuration and qualification stay separated in
// the Result: Installed is true only when the entry is configured with
// accredited ownership AND every supplied probe returned valid evidence for
// this server. Supplying no probe fails closed (Qualified=false); probe
// errors, rejecting probes, and evidence naming another server never report
// success.
//
// Add is idempotent only for entries accredited by the supplied ownership
// records. An identical but unaccredited entry fails closed with
// ConflictUnaccredited because cortex-ia never appropriates unregistered
// configuration. Unrelated keys, unknown MCP entries, and JSONC comments
// are preserved by the merge boundary.
func (m *Manager) Add(name string, evidence []OwnershipRecord, probes ...ProbeFunc) (Result, error) {
	preset, ok := Lookup(name)
	if !ok {
		return Result{}, &ConflictError{Name: name, Kind: ConflictUnmanaged}
	}
	if _, ok := preset.Command(); !ok {
		return Result{}, &InvalidPresetError{Name: name, Detail: "entry must be a local server with a non-empty string command vector"}
	}

	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return Result{}, err
	}

	presetDigest, err := SemanticDigest(name, preset.Entry)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Name:       name,
		ConfigPath: path,
		Ownership:  &OwnershipRecord{Name: name, Digest: presetDigest, ConfigPath: path},
	}
	postImageDigest, err := m.postImageDigest(name, preset.Entry)
	if err != nil {
		return Result{}, err
	}
	result.Ownership.PostImageDigest = postImageDigest

	observed, present := entries[name]
	if present {
		observedMap, isMap := observed.(map[string]any)
		if !isMap {
			return Result{}, &ConflictError{
				Name:   name,
				Kind:   ConflictMalformed,
				Detail: fmt.Sprintf("entry %q in %q is not a JSON object", name, path),
			}
		}
		observedDigest, err := SemanticDigest(name, observedMap)
		if err != nil {
			return Result{}, err
		}
		equal, err := semanticEqual(observedMap, preset.Entry)
		if err != nil {
			return Result{}, err
		}
		if !equal {
			return Result{}, &ConflictError{
				Name:           name,
				Kind:           ConflictModified,
				ExpectedDigest: presetDigest,
				ObservedDigest: observedDigest,
			}
		}
		if _, accredited := accredit(evidence, name, observedDigest, path); !accredited {
			return Result{}, &ConflictError{
				Name:           name,
				Kind:           ConflictUnaccredited,
				ExpectedDigest: presetDigest,
				ObservedDigest: observedDigest,
			}
		}
		result.Action = "already-present"
		result.Configured = true
		if len(probes) == 0 {
			probes = []ProbeFunc{LocalCommandProbe}
		}
		qualified, outcome := qualify(preset, probes)
		result.Qualified = qualified
		result.Qualification = outcome
		result.Installed = qualified
		return result, nil
	}

	overlay, err := json.Marshal(map[string]any{
		mcpKey: map[string]any{name: preset.Entry},
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode MCP overlay for %q: %w", name, err)
	}

	mutation, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return Result{}, fmt.Errorf("add MCP entry %q: %w", name, err)
	}
	result.Action = "added"
	result.Changed = mutation.Changed
	result.Created = mutation.Created
	result.Configured = true
	if len(probes) == 0 {
		probes = []ProbeFunc{LocalCommandProbe}
	}
	qualified, outcome := qualify(preset, probes)
	result.Qualified = qualified
	result.Qualification = outcome
	result.Installed = qualified
	return result, nil
}

// AddDesired installs the typed desired MCP server: a catalog preset, a
// custom local server with an exact argv vector (never a shell string), or
// a custom remote http(s) endpoint with optional header assignments. The
// desired description is validated before any configuration access, so a
// malformed request fails closed as *InvalidDesiredError without touching
// the home.
//
// Ownership and qualification semantics mirror Add: identical entries are
// idempotent only when accredited by the supplied ownership records,
// user-owned entries fail closed with typed conflicts, and Installed is
// true only when the entry is configured with accredited ownership AND
// every probe returned valid evidence. When no probe is supplied the
// kind-compatible offline default is applied: LocalCommandProbe for preset
// and local kinds, RemoteURLProbe for the remote kind. Env and header
// values reach the configuration file only; they are not representable in
// the ownership record, the semantic digest, or probe evidence.
func (m *Manager) AddDesired(desired Desired, evidence []OwnershipRecord, probes ...ProbeFunc) (Result, error) {
	if err := desired.Validate(); err != nil {
		return Result{}, err
	}
	entry, err := desired.Entry()
	if err != nil {
		return Result{}, err
	}

	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return Result{}, err
	}

	desiredDigest, err := SemanticDigest(desired.Name, entry)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Name:       desired.Name,
		ConfigPath: path,
		Ownership:  &OwnershipRecord{Name: desired.Name, Digest: desiredDigest, ConfigPath: path},
	}
	postImageDigest, err := m.postImageDigest(desired.Name, entry)
	if err != nil {
		return Result{}, err
	}
	result.Ownership.PostImageDigest = postImageDigest

	observed, present := entries[desired.Name]
	if present {
		observedMap, isMap := observed.(map[string]any)
		if !isMap {
			return Result{}, &ConflictError{
				Name:   desired.Name,
				Kind:   ConflictMalformed,
				Detail: fmt.Sprintf("entry %q in %q is not a JSON object", desired.Name, path),
			}
		}
		observedDigest, err := SemanticDigest(desired.Name, observedMap)
		if err != nil {
			return Result{}, err
		}
		equal, err := semanticEqual(observedMap, entry)
		if err != nil {
			return Result{}, err
		}
		if !equal {
			return Result{}, &ConflictError{
				Name:           desired.Name,
				Kind:           ConflictModified,
				ExpectedDigest: desiredDigest,
				ObservedDigest: observedDigest,
			}
		}
		if _, accredited := accredit(evidence, desired.Name, observedDigest, path); !accredited {
			return Result{}, &ConflictError{
				Name:           desired.Name,
				Kind:           ConflictUnaccredited,
				ExpectedDigest: desiredDigest,
				ObservedDigest: observedDigest,
			}
		}
		result.Action = "already-present"
		result.Configured = true
		qualified, outcome := qualifyDesired(desired, Preset{Name: desired.Name, Entry: entry}, probes)
		result.Qualified = qualified
		result.Qualification = outcome
		result.Installed = qualified
		return result, nil
	}

	overlay, err := json.Marshal(map[string]any{
		mcpKey: map[string]any{desired.Name: entry},
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode MCP overlay for %q: %w", desired.Name, err)
	}

	mutation, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return Result{}, fmt.Errorf("add MCP entry %q: %w", desired.Name, err)
	}
	result.Action = "added"
	result.Changed = mutation.Changed
	result.Created = mutation.Created
	result.Configured = true
	qualified, outcome := qualifyDesired(desired, Preset{Name: desired.Name, Entry: entry}, probes)
	result.Qualified = qualified
	result.Qualification = outcome
	result.Installed = qualified
	return result, nil
}

// postImageDigest computes the mcpv2 full-postimage fingerprint of one entry
// against this manager's salt and resolved config path. With no salt the
// result is the empty string: the caller records legacy ownership evidence
// and destructive accreditation of that record will fail closed.
func (m *Manager) postImageDigest(name string, entry map[string]any) (string, error) {
	if len(m.fingerprintSalt) == 0 {
		return "", nil
	}
	postImage, err := installmeta.MCPServerPostImageFromEntry(name, entry)
	if err != nil {
		return "", err
	}
	postImage.ConfigPath = m.ConfigPath()
	return installmeta.MCPPostImageDigest(postImage, m.fingerprintSalt)
}

// accreditPostImage verifies that one semantically accredited record also
// carries a verifiable, matching mcpv2 full-postimage fingerprint for the
// observed entry. It returns the typed fail-closed conflict otherwise:
// legacy mcpv1-only records (or an unverifiable local salt) yield
// ConflictLegacyOwnership with the manual re-add remedy, and a fingerprint
// mismatch yields ConflictDrifted. Comparison of the decoded sums is
// constant-time.
func (m *Manager) accreditPostImage(record OwnershipRecord, name string, observedEntry map[string]any) error {
	if record.PostImageDigest == "" {
		return &ConflictError{
			Name:           name,
			Kind:           ConflictLegacyOwnership,
			ObservedDigest: record.Digest,
		}
	}
	if len(m.fingerprintSalt) == 0 {
		return &ConflictError{
			Name:           name,
			Kind:           ConflictLegacyOwnership,
			ExpectedDigest: record.PostImageDigest,
			Detail:         "the local fingerprint salt is missing or corrupt, so the recorded mcpv2 fingerprint cannot be verified",
		}
	}
	expectedVersion, expectedSum, err := installmeta.ParseMCPServerDigest(record.PostImageDigest)
	if err != nil || expectedVersion != installmeta.MCPPostImageDigestVersion {
		return &ConflictError{
			Name:           name,
			Kind:           ConflictLegacyOwnership,
			ExpectedDigest: record.PostImageDigest,
			Detail:         "the recorded postimage digest is not the canonical mcpv2 encoding",
		}
	}
	expected, decodeErr := hex.DecodeString(expectedSum)
	if decodeErr != nil {
		return &ConflictError{
			Name:           name,
			Kind:           ConflictLegacyOwnership,
			ExpectedDigest: record.PostImageDigest,
			Detail:         "the recorded postimage digest is malformed",
		}
	}
	observedDigest, err := m.postImageDigest(name, observedEntry)
	if err != nil {
		return err
	}
	_, observedSum, parseErr := installmeta.ParseMCPServerDigest(observedDigest)
	if parseErr != nil {
		return parseErr
	}
	observed, decodeErr := hex.DecodeString(observedSum)
	if decodeErr != nil {
		return decodeErr
	}
	if !hmac.Equal(expected, observed) {
		return &ConflictError{
			Name:           name,
			Kind:           ConflictDrifted,
			ExpectedDigest: record.PostImageDigest,
			ObservedDigest: observedDigest,
		}
	}
	return nil
}

// RemoveCustom removes a custom (non-catalog) MCP entry. Removal is
// accredited exclusively by transactional ownership evidence, in two stages:
// a record whose name, secret-free semantic identity digest, and config path
// all match the observed entry, AND a recorded mcpv2 full-postimage
// fingerprint that still matches the live entry's URL, env/header values
// (keyed hashes), enabled state, identity, and config path. Absent entries
// are a no-op; entries without any matching record fail closed with
// *ConflictError; records predating mcpv2 fingerprints fail closed with the
// manual re-add remedy and are never silently upgraded; any postimage drift
// fails closed as typed drift. Nothing is mutated on any refusal.
func (m *Manager) RemoveCustom(name string, evidence []OwnershipRecord) (Result, error) {
	if strings.TrimSpace(name) == "" || len(evidence) == 0 {
		return Result{}, &ConflictError{Name: name, Kind: ConflictUnmanaged}
	}

	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return Result{}, err
	}

	result := Result{Name: name, ConfigPath: path}

	observed, present := entries[name]
	if !present {
		result.Action = "already-absent"
		return result, nil
	}

	observedMap, isMap := observed.(map[string]any)
	if !isMap {
		return Result{}, &ConflictError{
			Name:   name,
			Kind:   ConflictMalformed,
			Detail: fmt.Sprintf("entry %q in %q is not a JSON object", name, path),
		}
	}
	observedDigest, err := SemanticDigest(name, observedMap)
	if err != nil {
		return Result{}, err
	}
	record, accredited := accredit(evidence, name, observedDigest, path)
	if !accredited {
		return Result{}, &ConflictError{
			Name:           name,
			Kind:           ConflictUnaccredited,
			ExpectedDigest: "",
			ObservedDigest: observedDigest,
		}
	}
	// Second stage: the record must also accredit the full live postimage
	// (mcpv2). Legacy records and drifted entries fail closed here.
	if err := m.accreditPostImage(record, name, observedMap); err != nil {
		return Result{}, err
	}

	mutation, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{
		RemovePaths: [][]string{{mcpKey, name}},
	})
	if err != nil {
		return Result{}, fmt.Errorf("remove MCP entry %q: %w", name, err)
	}
	result.Action = "removed"
	result.Changed = mutation.Changed
	return result, nil
}

// InspectDesired predicts, read-only, the AddDesired outcome for the desired
// server against the supplied ownership evidence. It returns "add" when the
// entry is absent, or "already-present" with configured=true when the exact
// desired entry is present and accredited. A present entry that differs from
// the desired entry (including remote URL drift, which digests deliberately
// cannot see) fails closed with *ConflictError exactly like a real add; no
// probe runs and nothing is written.
func (m *Manager) InspectDesired(desired Desired, evidence []OwnershipRecord) (string, bool, error) {
	if err := desired.Validate(); err != nil {
		return "", false, err
	}
	entry, err := desired.Entry()
	if err != nil {
		return "", false, err
	}

	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return "", false, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return "", false, err
	}

	observed, present := entries[desired.Name]
	if !present {
		return "add", false, nil
	}

	observedMap, isMap := observed.(map[string]any)
	if !isMap {
		return "", false, &ConflictError{
			Name:   desired.Name,
			Kind:   ConflictMalformed,
			Detail: fmt.Sprintf("entry %q in %q is not a JSON object", desired.Name, path),
		}
	}
	observedDigest, err := SemanticDigest(desired.Name, observedMap)
	if err != nil {
		return "", false, err
	}
	desiredDigest, err := SemanticDigest(desired.Name, entry)
	if err != nil {
		return "", false, err
	}
	equal, err := semanticEqual(observedMap, entry)
	if err != nil {
		return "", false, err
	}
	if !equal {
		return "", false, &ConflictError{
			Name:           desired.Name,
			Kind:           ConflictModified,
			ExpectedDigest: desiredDigest,
			ObservedDigest: observedDigest,
		}
	}
	if _, accredited := accredit(evidence, desired.Name, observedDigest, path); !accredited {
		return "", false, &ConflictError{
			Name:           desired.Name,
			Kind:           ConflictUnaccredited,
			ExpectedDigest: desiredDigest,
			ObservedDigest: observedDigest,
		}
	}
	return "already-present", true, nil
}

// qualifyDesired evaluates qualification for a desired server, defaulting to
// the kind-compatible offline probe when the caller supplied none. The
// default never relaxes the boundary: preset and local servers still require
// a resolvable command, remote servers still require a well-formed http(s)
// URL, and configured-but-unqualified remains a non-success verdict.
func qualifyDesired(desired Desired, preset Preset, probes []ProbeFunc) (bool, *ProbeEvidence) {
	if len(probes) == 0 {
		if desired.Kind == DesiredRemote {
			probes = []ProbeFunc{RemoteURLProbe}
		} else {
			probes = []ProbeFunc{LocalCommandProbe}
		}
	}
	return qualify(preset, probes)
}

// Remove deletes the managed preset entry for name. Removal is accredited
// exclusively by transactional metadata: an ownership record whose name,
// semantic digest, and config path all match the observed entry. Absent
// entries are a no-op; unmanaged names, modified entries, and equivalent
// entries without a record fail closed with *ConflictError and mutate
// nothing.
func (m *Manager) Remove(name string, evidence []OwnershipRecord) (Result, error) {
	preset, ok := Lookup(name)
	if !ok {
		preset, ok = lookupRetired(name)
		if !ok {
			return Result{}, &ConflictError{Name: name, Kind: ConflictUnmanaged}
		}
	}

	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return Result{}, err
	}

	result := Result{Name: name, ConfigPath: path}

	observed, present := entries[name]
	if !present {
		result.Action = "already-absent"
		return result, nil
	}

	observedMap, isMap := observed.(map[string]any)
	if !isMap {
		return Result{}, &ConflictError{
			Name:   name,
			Kind:   ConflictMalformed,
			Detail: fmt.Sprintf("entry %q in %q is not a JSON object", name, path),
		}
	}
	observedDigest, err := SemanticDigest(name, observedMap)
	if err != nil {
		return Result{}, err
	}
	equal, err := semanticEqual(observedMap, preset.Entry)
	if err != nil {
		return Result{}, err
	}
	presetDigest, err := SemanticDigest(name, preset.Entry)
	if err != nil {
		return Result{}, err
	}
	if !equal {
		return Result{}, &ConflictError{
			Name:           name,
			Kind:           ConflictModified,
			ExpectedDigest: presetDigest,
			ObservedDigest: observedDigest,
		}
	}
	if _, accredited := accredit(evidence, name, observedDigest, path); !accredited {
		return Result{}, &ConflictError{
			Name:           name,
			Kind:           ConflictUnaccredited,
			ExpectedDigest: presetDigest,
			ObservedDigest: observedDigest,
		}
	}

	mutation, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{
		RemovePaths: [][]string{{mcpKey, name}},
	})
	if err != nil {
		return Result{}, fmt.Errorf("remove MCP entry %q: %w", name, err)
	}
	result.Action = "removed"
	result.Changed = mutation.Changed
	return result, nil
}

// List reports the status of every managed preset, every accredited custom
// entry, and the names of unknown MCP entries found in the configuration.
// Managed status requires an accredited ownership record; an entry that
// equals a preset without a record is unmanaged-equivalent, and a custom
// entry without a matching record stays unknown (user-owned). Reports carry
// a sanitized identity projection only: variable names, entry type, and the
// secret-free digest. List never writes.
func (m *Manager) List(evidence []OwnershipRecord) (ListResult, error) {
	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return ListResult{}, err
	}

	entries, err := mcpEntries(config, path)
	if err != nil {
		return ListResult{}, err
	}

	listing := ListResult{ConfigPath: path}
	managedNames := make(map[string]struct{}, len(managedPresets))
	for _, preset := range Presets() {
		managedNames[preset.Name] = struct{}{}
		report := EntryReport{Name: preset.Name, Status: StatusAbsent}
		if observed, present := entries[preset.Name]; present {
			observedMap, isMap := observed.(map[string]any)
			if !isMap {
				return ListResult{}, &ConflictError{
					Name:   preset.Name,
					Kind:   ConflictMalformed,
					Detail: fmt.Sprintf("entry %q in %q is not a JSON object", preset.Name, path),
				}
			}
			digest, err := SemanticDigest(preset.Name, observedMap)
			if err != nil {
				return ListResult{}, err
			}
			report.Digest = digest
			fillSanitizedIdentity(&report, preset.Name, observedMap)
			equal, err := semanticEqual(observedMap, preset.Entry)
			if err != nil {
				return ListResult{}, err
			}
			switch {
			case !equal:
				report.Status = StatusConflict
			default:
				if _, accredited := accredit(evidence, preset.Name, digest, path); accredited {
					report.Status = StatusManaged
				} else {
					report.Status = StatusUnmanagedEquivalent
				}
			}
		}
		listing.Entries = append(listing.Entries, report)
	}
	for _, preset := range RetiredPresets() {
		observed, present := entries[preset.Name]
		if !present {
			continue
		}
		managedNames[preset.Name] = struct{}{}
		report := EntryReport{Name: preset.Name, Status: StatusConflict}
		observedMap, isMap := observed.(map[string]any)
		if !isMap {
			return ListResult{}, &ConflictError{Name: preset.Name, Kind: ConflictMalformed, Detail: "retired entry is not a JSON object"}
		}
		digest, err := SemanticDigest(preset.Name, observedMap)
		if err != nil {
			return ListResult{}, err
		}
		report.Digest = digest
		fillSanitizedIdentity(&report, preset.Name, observedMap)
		equal, err := semanticEqual(observedMap, preset.Entry)
		if err != nil {
			return ListResult{}, err
		}
		if equal {
			if _, accredited := accredit(evidence, preset.Name, digest, path); accredited {
				report.Status = StatusManaged
			} else {
				report.Status = StatusUnmanagedEquivalent
			}
		}
		listing.Entries = append(listing.Entries, report)
	}

	customNames := make([]string, 0)
	for name := range entries {
		if _, managed := managedNames[name]; !managed {
			customNames = append(customNames, name)
		}
	}
	sort.Strings(customNames)
	for _, name := range customNames {
		observedMap, isMap := entries[name].(map[string]any)
		if !isMap {
			return ListResult{}, &ConflictError{
				Name:   name,
				Kind:   ConflictMalformed,
				Detail: fmt.Sprintf("entry %q in %q is not a JSON object", name, path),
			}
		}
		digest, err := SemanticDigest(name, observedMap)
		if err != nil {
			// An entry whose identity cannot be parsed cannot be accredited
			// by any record; it stays user-owned and informational.
			listing.Unknown = append(listing.Unknown, name)
			continue
		}
		record, accredited := accredit(evidence, name, digest, path)
		if !accredited {
			listing.Unknown = append(listing.Unknown, name)
			continue
		}
		if len(m.fingerprintSalt) > 0 {
			// Fingerprinting callers (the install service) classify customs
			// by the full mcpv2 postimage: a matching fingerprint is the
			// only managed verdict, a drifted entry is reported as a
			// conflict with its typed reason, and legacy mcpv1-only records
			// are reported as legacy-ownership conflicts with the re-add
			// remedy. Nothing is mutated by List in any case.
			report := EntryReport{Name: name, Status: StatusManaged, Digest: digest}
			fillSanitizedIdentity(&report, name, observedMap)
			switch conflict := m.accreditPostImage(record, name, observedMap); {
			case conflict == nil:
			case isConflictKind(conflict, ConflictDrifted):
				report.Status = StatusConflict
				report.Detail = "postimage drift detected: URL, env/header values, enabled, type, argv, or config file changed after accreditation"
			default:
				report.Status = StatusConflict
				report.Detail = "legacy ownership record without an mcpv2 postimage fingerprint; re-run the add command to accredit the full postimage, then remove"
			}
			listing.Entries = append(listing.Entries, report)
			continue
		}
		report := EntryReport{Name: name, Status: StatusManaged, Digest: digest}
		fillSanitizedIdentity(&report, name, observedMap)
		listing.Entries = append(listing.Entries, report)
	}
	sort.Strings(listing.Unknown)
	return listing, nil
}

// isConflictKind reports whether err is a *ConflictError of the given kind.
func isConflictKind(err error, kind ConflictKind) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict) && conflict.Kind == kind
}

// fillSanitizedIdentity copies the secret-free identity projection of one
// observed entry onto its report: entry type plus env and header variable
// NAMES. Values, URLs, and argv vectors are never copied; a shape that
// cannot yield an identity leaves the report unenriched.
func fillSanitizedIdentity(report *EntryReport, name string, entry map[string]any) {
	identity, err := installmeta.MCPServerIdentityFromEntry(name, entry)
	if err != nil {
		return
	}
	report.Type = identity.Type
	report.EnvNames = identity.EnvNames
	report.HeaderNames = identity.HeaderNames
}

// accredit returns the ownership record that accredits the observed entry:
// record name, semantic digest, and config path must all match. Anything
// else is not ownership evidence.
func accredit(evidence []OwnershipRecord, name, observedDigest, path string) (OwnershipRecord, bool) {
	for _, record := range evidence {
		if record.Name == name && record.Digest == observedDigest && record.ConfigPath == path {
			return record, true
		}
	}
	return OwnershipRecord{}, false
}

// qualify evaluates every probe against the preset. All probes must return
// valid, error-free evidence naming this server (or leaving the name empty)
// for a qualified verdict; absence of probes fails closed. The returned
// outcome is nil only when no probe ran.
func qualify(preset Preset, probes []ProbeFunc) (bool, *ProbeEvidence) {
	if len(probes) == 0 {
		return false, nil
	}
	for _, probe := range probes {
		evidence, err := probe(preset)
		if err != nil {
			return false, &ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     fmt.Sprintf("probe error: %v", err),
			}
		}
		if evidence.ServerName == "" {
			evidence.ServerName = preset.Name
		}
		if evidence.ServerName != preset.Name {
			return false, &ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     fmt.Sprintf("probe evidence belongs to server %q, not %q", evidence.ServerName, preset.Name),
			}
		}
		if !evidence.Valid {
			if evidence.Detail == "" {
				evidence.Detail = "probe reported the server as not qualified"
			}
			return false, &evidence
		}
	}
	return true, &ProbeEvidence{
		ServerName: preset.Name,
		Valid:      true,
		Summary:    fmt.Sprintf("%d probe(s) returned valid evidence", len(probes)),
	}
}

// loadConfig reads and decodes the OpenCode global config. A missing file is
// an empty config; JSONC comments, duplicate-member rejection, and root type
// validation are delegated to the shared filemerge boundary.
func loadConfig(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read OpenCode config %q: %w", path, err)
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		return nil, &ConflictError{Kind: ConflictMalformed, Detail: err.Error()}
	}
	return config, nil
}

// mcpEntries extracts the "mcp" object. A "mcp" value of any non-object type
// is a malformed conflict, never an implicit replacement.
func mcpEntries(config map[string]any, path string) (map[string]any, error) {
	value, present := config[mcpKey]
	if !present || value == nil {
		return map[string]any{}, nil
	}
	entries, isMap := value.(map[string]any)
	if !isMap {
		return nil, &ConflictError{
			Kind:   ConflictMalformed,
			Detail: fmt.Sprintf("%q in %q must be a JSON object", mcpKey, path),
		}
	}
	return entries, nil
}
