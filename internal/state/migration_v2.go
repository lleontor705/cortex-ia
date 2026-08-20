package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// MigrationSourceNone means no legacy metadata existed.
const MigrationSourceNone = "none"

// MigrationSourceLegacyEmpty means legacy metadata existed but recorded no
// managed content, so a v2 install may proceed as a fresh install.
const MigrationSourceLegacyEmpty = "legacy-empty"

// MigrationSourceLegacyAdopt means legacy metadata recorded verifiable file
// evidence whose every path is provably and strictly inside the supplied
// OpenCode root, and whose state and lock declarations agree, so a v2
// install may adopt the targets.
const MigrationSourceLegacyAdopt = "legacy-adopt"

// LegacySnapshot is a tolerant, read-only view of v1 metadata. Reading never
// fails and never writes: parse problems are captured as booleans plus
// details so planning can fail closed while the files stay untouched.
type LegacySnapshot struct {
	StateExists    bool
	LockExists     bool
	State          *State
	Lock           *Lockfile
	StateMalformed bool
	LockMalformed  bool
	StateDigest    string
	LockDigest     string
	// ReadErrors records non-fatal IO problems, e.g. unreadable files.
	ReadErrors []string
}

// ReadLegacySnapshot reads the legacy state and lock files beneath homeDir
// read-only. It never mutates, creates, or deletes anything and always
// returns a usable snapshot; malformed or unreadable documents are marked
// instead of failing.
func ReadLegacySnapshot(homeDir string) LegacySnapshot {
	snap := LegacySnapshot{}
	raw, err := os.ReadFile(StatePath(homeDir))
	if err != nil {
		if !os.IsNotExist(err) {
			snap.StateExists = true
			snap.StateMalformed = true
			snap.ReadErrors = append(snap.ReadErrors,
				fmt.Sprintf("read legacy state: %v", err))
		}
	} else {
		snap.StateExists = true
		snap.StateDigest = rawDigest(raw)
		var legacy State
		if jsonErr := json.Unmarshal(raw, &legacy); jsonErr != nil {
			snap.StateMalformed = true
		} else {
			snap.State = &legacy
		}
	}
	raw, err = os.ReadFile(LockPath(homeDir))
	if err != nil {
		if !os.IsNotExist(err) {
			snap.LockExists = true
			snap.LockMalformed = true
			snap.ReadErrors = append(snap.ReadErrors,
				fmt.Sprintf("read legacy lock: %v", err))
		}
	} else {
		snap.LockExists = true
		snap.LockDigest = rawDigest(raw)
		var legacy Lockfile
		if jsonErr := json.Unmarshal(raw, &legacy); jsonErr != nil {
			snap.LockMalformed = true
		} else {
			snap.Lock = &legacy
		}
	}
	return snap
}

// MigrationCandidate is an in-memory migration decision that permits a v2
// install. The candidate is never persisted on its own; it is embedded as
// MigrationProvenance when v2 metadata is committed.
type MigrationCandidate struct {
	Source     string
	Provenance MigrationProvenance
}

// MigrationBlocker is a fail-closed migration outcome. It carries the reason
// and a remediation hint, plus the read-only legacy evidence paths.
type MigrationBlocker struct {
	Reason      string
	Remediation string
	Evidence    []string
}

// MigrationDecision is either a candidate or a blocker, never both and
// never neither.
type MigrationDecision struct {
	Candidate *MigrationCandidate
	Blocker   *MigrationBlocker
}

// AssessMigration reads legacy metadata beneath homeDir, resolves the
// OpenCode root to an absolute cleaned path, and derives a bounded, purely
// in-memory migration decision. It never mutates legacy files:
//
//   - no legacy files at all                     -> fresh candidate ("none")
//   - legacy files recording no managed content  -> fresh candidate ("legacy-empty")
//   - declared content without verifiable files  -> blocker (fail closed)
//   - state and lock declarations disagree       -> blocker (fail closed)
//   - any recorded path not strictly inside the  -> blocker (fail closed)
//     resolved OpenCode root
//   - duplicate or case-colliding lock paths     -> blocker (fail closed)
//   - everything provable                        -> adopt candidate ("legacy-adopt")
//
// An empty or unresolvable root proves nothing and fails closed.
func AssessMigration(homeDir, opencodeRoot string, now time.Time) MigrationDecision {
	snap := ReadLegacySnapshot(homeDir)
	root, err := resolveRoot(opencodeRoot)
	if err != nil {
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason:      fmt.Sprintf("OpenCode root cannot be resolved: %v", err),
			Remediation: "supply a non-empty, resolvable OpenCode configuration root",
			Evidence:    legacyEvidencePaths(snap),
		}}
	}
	return assessSnapshot(snap, root, now)
}

// resolveRoot returns the absolute, cleaned OpenCode root used for every
// ownership proof. Relative roots resolve against the process working
// directory; resolution failures fail closed.
func resolveRoot(opencodeRoot string) (string, error) {
	if strings.TrimSpace(opencodeRoot) == "" {
		return "", fmt.Errorf("empty root")
	}
	return filepath.Abs(filepath.Clean(opencodeRoot))
}

func assessSnapshot(snap LegacySnapshot, root string, now time.Time) MigrationDecision {
	evidence := legacyEvidencePaths(snap)
	switch {
	case !snap.StateExists && !snap.LockExists:
		return MigrationDecision{Candidate: &MigrationCandidate{
			Source: MigrationSourceNone,
			Provenance: MigrationProvenance{
				Source:     MigrationSourceNone,
				AssessedAt: now,
			},
		}}
	case snap.StateMalformed || snap.LockMalformed:
		return malformedBlocker(snap, evidence)
	case legacyIsEmpty(snap):
		return MigrationDecision{Candidate: &MigrationCandidate{
			Source: MigrationSourceLegacyEmpty,
			Provenance: MigrationProvenance{
				Source:        MigrationSourceLegacyEmpty,
				LegacyDigests: legacyDigests(snap),
				AssessedAt:    now,
				Note:          "legacy metadata records no managed content; install proceeds as fresh",
			},
		}}
	case snap.Lock == nil || len(snap.Lock.Files) == 0:
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason: "legacy metadata declares managed content without verifiable file evidence",
			Remediation: "legacy lock must record the managed files so ownership can be proven; " +
				"review the legacy files listed in evidence, then rerun the install; " +
				"cortex-ia never modifies legacy metadata",
			Evidence: evidence,
		}}
	}
	if reason := legacyDeclarationsAgree(snap); reason != "" {
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason: "legacy state and lock disagree on " + reason,
			Remediation: "reconcile the legacy state and lock declarations manually, then rerun " +
				"the install; cortex-ia never modifies legacy metadata",
			Evidence: evidence,
		}}
	}
	if !legacyIntentDeclared(snap) {
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason: "legacy lock records file evidence without declared install intent",
			Remediation: "reconcile the legacy state and lock declarations manually, then rerun " +
				"the install; cortex-ia never modifies legacy metadata",
			Evidence: evidence,
		}}
	}
	if escaping := pathsNotStrictlyWithin(root, snap.Lock.Files); len(escaping) > 0 {
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason: fmt.Sprintf("legacy lock records %d path(s) not strictly inside the OpenCode root (first: %s)",
				len(escaping), escaping[0]),
			Remediation: "review the legacy files listed in evidence, remove or relocate them " +
				"manually, then rerun the install; cortex-ia never modifies legacy metadata",
			Evidence: evidence,
		}}
	}
	if dups := duplicateLegacyPaths(snap.Lock.Files); len(dups) > 0 {
		return MigrationDecision{Blocker: &MigrationBlocker{
			Reason: fmt.Sprintf("legacy lock records %d duplicate or case-colliding path(s) (first: %s)",
				len(dups), dups[0]),
			Remediation: "reconcile the duplicated entries in the legacy lock manually so every " +
				"managed path is recorded exactly once, then rerun the install; " +
				"cortex-ia never modifies legacy metadata",
			Evidence: evidence,
		}}
	}
	return MigrationDecision{Candidate: &MigrationCandidate{
		Source: MigrationSourceLegacyAdopt,
		Provenance: MigrationProvenance{
			Source:        MigrationSourceLegacyAdopt,
			LegacyDigests: legacyDigests(snap),
			AssessedAt:    now,
			Note: fmt.Sprintf("legacy lock records %d path(s), all strictly inside the OpenCode root, "+
				"with agreeing state and lock declarations", len(snap.Lock.Files)),
		},
	}}
}

func malformedBlocker(snap LegacySnapshot, evidence []string) MigrationDecision {
	problems := make([]string, 0, 2)
	if snap.StateMalformed {
		problems = append(problems, "state.json")
	}
	if snap.LockMalformed {
		problems = append(problems, "cortex-ia.lock")
	}
	if len(snap.ReadErrors) > 0 {
		evidence = append([]string(nil), evidence...)
		evidence = append(evidence, snap.ReadErrors...)
	}
	return MigrationDecision{Blocker: &MigrationBlocker{
		Reason: fmt.Sprintf("legacy metadata malformed or unreadable: %s",
			strings.Join(problems, ", ")),
		Remediation: "fix or remove the malformed legacy files manually, then rerun the " +
			"install; cortex-ia never modifies legacy metadata",
		Evidence: evidence,
	}}
}

func legacyEvidencePaths(snap LegacySnapshot) []string {
	evidence := make([]string, 0, 2)
	if snap.StateExists {
		evidence = append(evidence, "state.json (legacy, read-only)")
	}
	if snap.LockExists {
		evidence = append(evidence, "cortex-ia.lock (legacy, read-only)")
	}
	return evidence
}

func legacyDigests(snap LegacySnapshot) []string {
	digests := make([]string, 0, 2)
	if snap.StateDigest != "" {
		digests = append(digests, snap.StateDigest)
	}
	if snap.LockDigest != "" {
		digests = append(digests, snap.LockDigest)
	}
	return digests
}

// legacyIsEmpty reports whether the legacy snapshot records no managed
// content: no agents, components, preset, registry selection, or lock files.
func legacyIsEmpty(snap LegacySnapshot) bool {
	if snap.State == nil {
		return !snap.LockHasContent()
	}
	stateEmpty := len(snap.State.InstalledAgents) == 0 &&
		len(snap.State.Components) == 0 &&
		snap.State.Preset == "" &&
		!legacyHasRegistrySelection(snap.State.RegistrySelection)
	return stateEmpty && !snap.LockHasContent()
}

// LockHasContent reports whether the legacy lock records any managed
// surface. A missing lock has no content by definition.
func (s LegacySnapshot) LockHasContent() bool {
	if s.Lock == nil {
		return false
	}
	return len(s.Lock.InstalledAgents) > 0 ||
		len(s.Lock.Components) > 0 ||
		len(s.Lock.Files) > 0 ||
		s.Lock.Preset != "" ||
		legacyHasRegistrySelection(s.Lock.RegistrySelection)
}

// legacyDeclarationsAgree verifies that the legacy state and lock declare
// the same managed surface before adoption: installed agents, components,
// preset, and registry selection. A missing side counts as empty and
// therefore disagrees with any declared content on the other side. It
// returns "" when both sides agree, or a human-readable field description.
func legacyDeclarationsAgree(snap LegacySnapshot) string {
	stateAgents := agentIDs(snap.State)
	lockAgents := agentIDs(snap.Lock)
	if !sameStringSet(stateAgents, lockAgents) {
		return "installed_agents"
	}
	if !sameStringSet(componentIDs(snap.State), componentIDs(snap.Lock)) {
		return "components"
	}
	var statePreset, lockPreset string
	if snap.State != nil {
		statePreset = string(snap.State.Preset)
	}
	if snap.Lock != nil {
		lockPreset = string(snap.Lock.Preset)
	}
	if statePreset != lockPreset {
		return "preset"
	}
	if !registrySelectionsAgree(snap.State, snap.Lock) {
		return "registry_selection"
	}
	return ""
}

// legacyIntentDeclared reports whether either legacy side declares any
// install intent at all (agents, components, preset, or registry
// selection). Adoption must ground the lock's file evidence in declared
// intent; file evidence with empty declarations on both sides is ambiguous
// ownership and fails closed.
func legacyIntentDeclared(snap LegacySnapshot) bool {
	if snap.Lock != nil &&
		(len(snap.Lock.InstalledAgents) > 0 ||
			len(snap.Lock.Components) > 0 ||
			snap.Lock.Preset != "" ||
			legacyHasRegistrySelection(snap.Lock.RegistrySelection)) {
		return true
	}
	if snap.State == nil {
		return false
	}
	return len(snap.State.InstalledAgents) > 0 ||
		len(snap.State.Components) > 0 ||
		snap.State.Preset != "" ||
		legacyHasRegistrySelection(snap.State.RegistrySelection)
}

func agentIDs(v any) []string {
	switch t := v.(type) {
	case *State:
		if t == nil {
			return nil
		}
		out := make([]string, len(t.InstalledAgents))
		for i, a := range t.InstalledAgents {
			out[i] = string(a)
		}
		return out
	case *Lockfile:
		if t == nil {
			return nil
		}
		out := make([]string, len(t.InstalledAgents))
		for i, a := range t.InstalledAgents {
			out[i] = string(a)
		}
		return out
	}
	return nil
}

func componentIDs(v any) []string {
	switch t := v.(type) {
	case *State:
		if t == nil {
			return nil
		}
		out := make([]string, len(t.Components))
		for i, c := range t.Components {
			out[i] = string(c)
		}
		return out
	case *Lockfile:
		if t == nil {
			return nil
		}
		out := make([]string, len(t.Components))
		for i, c := range t.Components {
			out[i] = string(c)
		}
		return out
	}
	return nil
}

// registrySelectionsAgree compares the registry selection evidence of the
// legacy state and lock: both absent, or both present and deeply equal.
func registrySelectionsAgree(s *State, l *Lockfile) bool {
	sJSON, sOK := registrySelectionJSON(s)
	lJSON, lOK := lockRegistrySelectionJSON(l)
	if sOK != lOK {
		return false
	}
	if !sOK {
		return true
	}
	return sJSON == lJSON
}

// legacyHasRegistrySelection reports whether the legacy registry_selection
// evidence is present. A JSON null parses to the literal bytes "null" and
// counts as absent, matching the v1 pointer semantics this assessment was
// specified against.
func legacyHasRegistrySelection(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

// registrySelectionJSON normalizes the legacy registry selection evidence
// through a decode/encode round trip so deep equality stays formatting- and
// key-order-insensitive, exactly as the v1 struct comparison was.
func registrySelectionJSON(s *State) (string, bool) {
	if s == nil || !legacyHasRegistrySelection(s.RegistrySelection) {
		return "", false
	}
	var normalized any
	if err := json.Unmarshal(s.RegistrySelection, &normalized); err != nil {
		return "", true
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", true
	}
	return string(raw), true
}

func lockRegistrySelectionJSON(l *Lockfile) (string, bool) {
	if l == nil || !legacyHasRegistrySelection(l.RegistrySelection) {
		return "", false
	}
	var normalized any
	if err := json.Unmarshal(l.RegistrySelection, &normalized); err != nil {
		return "", true
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", true
	}
	return string(raw), true
}

// sameStringSet compares two lists as sets of non-empty strings.
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// pathsNotStrictlyWithin returns every path that cannot be proven strictly
// beneath root. Only canonical relative records are acceptable: empty paths,
// absolute paths (even when they happen to sit inside the root), and
// volume-qualified forms all fail closed, as do relative records that escape
// the root after resolution. The root itself is not a manageable artifact
// and is rejected.
func pathsNotStrictlyWithin(root string, paths []string) []string {
	var escaping []string
	for _, p := range paths {
		if !pathStrictlyWithin(root, p) {
			escaping = append(escaping, p)
		}
	}
	return escaping
}

// pathStrictlyWithin reports whether a legacy lock path is provably strictly
// beneath root. Empty paths, ".", absolute paths, volume-qualified forms
// (drive-relative "C:foo", drive-absolute "C:\x", UNC "\\server\share",
// device "\\.\Com1", and extended-length "\\?\C:\x" all carry a volume name),
// and traversal sequences fail closed. On Windows, path elements that would
// resolve to reserved DOS device names (NUL, CON, COM1-9, ...) are rejected
// as well. Surviving relative records are resolved against the root and must
// stay strictly beneath it.
func pathStrictlyWithin(root, p string) bool {
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return false
	}
	if runtime.GOOS == "windows" && containsReservedElement(p) {
		return false
	}
	resolved := filepath.Join(root, p)
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	// The root itself is never a manageable artifact.
	if rel == "." {
		return false
	}
	return true
}

// containsReservedElement reports whether any element of p resolves to a
// reserved DOS device name on Windows: CON, PRN, AUX, NUL, COM1-9, or
// LPT1-9, case-insensitively and with or without an extension ("NUL.txt"
// still opens the device). It is only consulted on Windows, where such
// names are not real files beneath any directory.
func containsReservedElement(p string) bool {
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' || p[i] == '\\' {
			if windowsReservedName(p[start:i]) {
				return true
			}
			start = i + 1
		}
	}
	return false
}

// windowsReservedName reports whether one path element is a reserved DOS
// device name.
func windowsReservedName(elem string) bool {
	base, _, _ := strings.Cut(elem, ".")
	switch strings.ToUpper(base) {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 {
		prefix := strings.ToUpper(base[:3])
		last := base[3]
		if (prefix == "COM" || prefix == "LPT") && last >= '1' && last <= '9' {
			return true
		}
	}
	return false
}

// caseInsensitivePaths reports whether the platform's prevailing filesystems
// resolve paths case-insensitively (Windows and macOS defaults). On such
// platforms two lock paths differing only by case collide and must fail
// closed; on case-sensitive platforms they are distinct files.
func caseInsensitivePaths() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

// duplicateLegacyPaths returns every lock path recorded more than once:
// exact duplicates are non-canonical on every platform, and on
// case-insensitive platforms case-folded duplicates collide as the same file.
// On Windows, slash- and backslash-separated records that resolve to the same
// file are also duplicates, so separator forms are normalized before
// comparison.
func duplicateLegacyPaths(paths []string) []string {
	seenExact := make(map[string]bool, len(paths))
	seenFolded := make(map[string]bool, len(paths))
	dupKey := func(p string) string {
		key := p
		if runtime.GOOS == "windows" {
			key = strings.ReplaceAll(key, "\\", "/")
		}
		if caseInsensitivePaths() {
			key = strings.ToLower(key)
		}
		return key
	}
	var dups []string
	for _, p := range paths {
		if seenExact[p] {
			dups = append(dups, p)
			continue
		}
		seenExact[p] = true
		key := dupKey(p)
		if seenFolded[key] {
			dups = append(dups, p)
			continue
		}
		seenFolded[key] = true
	}
	return dups
}

func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
