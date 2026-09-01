package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

// MetadataSchemaV2 is the schema version written by the OpenCode installer
// metadata boundary. Documents without "schema_version" are legacy (v1);
// documents with any other version are rejected, never degraded to legacy.
const MetadataSchemaV2 = 2

// Ownership classifies who owns a managed path or entry.
type Ownership string

const (
	// OwnershipManaged marks entries cortex-ia is allowed to mutate.
	OwnershipManaged Ownership = "managed"
	// OwnershipUser marks entries that belong to the user and must never be
	// overwritten without explicit confirmation and a verified backup.
	OwnershipUser Ownership = "user"
)

// validOwnership is the closed set of ownership values accepted in v2
// documents.
var validOwnership = map[Ownership]bool{
	OwnershipManaged: true,
	OwnershipUser:    true,
}

// ArtifactKind is the semantic classification of a managed asset.
type ArtifactKind string

const (
	KindSkill     ArtifactKind = "skill"
	KindShared    ArtifactKind = "shared"
	KindAgent     ArtifactKind = "agent"
	KindCommand   ArtifactKind = "command"
	KindPrompt    ArtifactKind = "prompt"
	KindMCPConfig ArtifactKind = "mcp-config"
	KindRule      ArtifactKind = "rule"
	KindOther     ArtifactKind = "other"
	KindTUI       ArtifactKind = "tui"
)

// validArtifactKind is the closed set of artifact kinds accepted in v2
// documents.
var validArtifactKind = map[ArtifactKind]bool{
	KindSkill:     true,
	KindShared:    true,
	KindAgent:     true,
	KindCommand:   true,
	KindPrompt:    true,
	KindMCPConfig: true,
	KindRule:      true,
	KindOther:     true,
	KindTUI:       true,
}

// ArtifactPrior records pre-install evidence for drift detection.
type ArtifactPrior struct {
	Existed bool   `json:"existed"`
	Digest  string `json:"digest,omitempty"`
}

// ArtifactV2 describes one managed asset beneath the OpenCode root.
type ArtifactV2 struct {
	// Path is relative to the OpenCode root, slash-separated, cleaned, and
	// strictly beneath the root (never absolute, traversal, or ".").
	Path   string       `json:"path"`
	Kind   ArtifactKind `json:"kind"`
	Origin string       `json:"origin"`
	Digest string       `json:"digest"`
	// SourceDigest is the sha256 of the source bytes when they differ.
	SourceDigest string         `json:"source_digest,omitempty"`
	Ownership    Ownership      `json:"ownership"`
	Prior        *ArtifactPrior `json:"prior,omitempty"`
	// BackupRef locates the artifact inside the verified backup set.
	BackupRef string `json:"backup_ref,omitempty"`
}

// MCPV2 records one managed OpenCode MCP server. It intentionally carries no
// configuration values: no URLs, headers, or environment variable values.
// Only non-secret semantic identity and a sanitized digest are persisted, so
// no secret or token can reach the state or lock file.
type MCPV2 struct {
	Name string `json:"name"`
	// ConfigPath is the MCP config file relative to the OpenCode root.
	ConfigPath string `json:"config_path"`
	// SemanticDigest is the versioned, secret-free canonical MCP digest
	// defined by internal/installmeta ("mcpv<version>:<hex64>") over the
	// server identity only. It is exactly the encoding the OpenCode MCP
	// manager records, so state, lock, and ownership evidence can never
	// disagree on one server's identity.
	SemanticDigest string    `json:"semantic_digest"`
	Ownership      Ownership `json:"ownership"`
}

// NewMCPV2 builds a sanitized MCP record from the canonical secret-free
// identity defined by internal/installmeta. The semantic digest is computed
// by the shared leaf, never locally: state and the OpenCode MCP manager use
// exactly one versioned encoding. Identity carries variable NAMES only;
// values and URLs are not representable in it and can never reach state or
// lock files.
func NewMCPV2(identity installmeta.MCPServerIdentity, configPath string, ownership Ownership) (MCPV2, error) {
	if strings.TrimSpace(identity.Name) == "" {
		return MCPV2{}, &ValidationError{Field: "mcps.name", Reason: "empty"}
	}
	digest, err := installmeta.MCPIdentityDigest(identity)
	if err != nil {
		return MCPV2{}, fmt.Errorf("state: digest MCP identity %q: %w", identity.Name, err)
	}
	return MCPV2{
		Name:           identity.Name,
		ConfigPath:     normalizeRelPath(configPath),
		SemanticDigest: digest,
		Ownership:      ownership,
	}, nil
}

// SelectionV2 records the intent of the last successful install.
type SelectionV2 struct {
	AssetGroups []string `json:"asset_groups,omitempty"`
	Cortex      bool     `json:"cortex"`
	Context7    bool     `json:"context7"`
}

// MigrationProvenance records how legacy (v1) metadata was assessed. It is
// derived in memory and never causes legacy files to be rewritten removed.
type MigrationProvenance struct {
	// Source is one of "none", "legacy-empty", or "legacy-adopt".
	Source string `json:"source"`
	// LegacyDigests are raw sha256 digests of the legacy files, kept as
	// read-only evidence.
	LegacyDigests []string  `json:"legacy_digests,omitempty"`
	AssessedAt    time.Time `json:"assessed_at"`
	Note          string    `json:"note,omitempty"`
}

// MetadataV2 is the v2 state document committed only after target mutation
// has succeeded.
type MetadataV2 struct {
	SchemaVersion int `json:"schema_version"`
	// OpencodeRoot is the canonical OpenCode configuration root the
	// artifacts and MCP entries are relative to.
	OpencodeRoot  string               `json:"opencode_root"`
	Selection     SelectionV2          `json:"selection"`
	Artifacts     []ArtifactV2         `json:"artifacts"`
	MCPs          []MCPV2              `json:"mcps"`
	TransactionID string               `json:"transaction_id"`
	BackupID      string               `json:"backup_id,omitempty"`
	Migration     *MigrationProvenance `json:"migration,omitempty"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// LockV2 is the v2 lock document. It mirrors MetadataV2 for verification and
// repair; disagreement between the two fails closed via CheckAgreementV2.
type LockV2 struct {
	SchemaVersion int          `json:"schema_version"`
	OpencodeRoot  string       `json:"opencode_root"`
	Artifacts     []ArtifactV2 `json:"artifacts"`
	MCPs          []MCPV2      `json:"mcps"`
	TransactionID string       `json:"transaction_id"`
	BackupID      string       `json:"backup_id,omitempty"`
	GeneratedAt   time.Time    `json:"generated_at"`
}

// NewLockFromMetadataV2 derives the lock view of a metadata document so both
// files agree on transaction and managed artifacts by construction.
func NewLockFromMetadataV2(meta MetadataV2) LockV2 {
	meta.Normalize()
	return LockV2{
		SchemaVersion: MetadataSchemaV2,
		OpencodeRoot:  meta.OpencodeRoot,
		Artifacts:     meta.Artifacts,
		MCPs:          meta.MCPs,
		TransactionID: meta.TransactionID,
		BackupID:      meta.BackupID,
		GeneratedAt:   meta.UpdatedAt,
	}
}

// Normalize sorts artifacts by path and MCPs by name so serialized documents
// and agreement checks are deterministic. Nil slices become empty slices.
func (m *MetadataV2) Normalize() {
	if m == nil {
		return
	}
	if m.Artifacts == nil {
		m.Artifacts = []ArtifactV2{}
	}
	if m.MCPs == nil {
		m.MCPs = []MCPV2{}
	}
	sort.SliceStable(m.Artifacts, func(i, j int) bool {
		return m.Artifacts[i].Path < m.Artifacts[j].Path
	})
	sort.SliceStable(m.MCPs, func(i, j int) bool {
		return m.MCPs[i].Name < m.MCPs[j].Name
	})
}

// Normalize sorts the lock document exactly like MetadataV2.
func (l *LockV2) Normalize() {
	if l == nil {
		return
	}
	if l.Artifacts == nil {
		l.Artifacts = []ArtifactV2{}
	}
	if l.MCPs == nil {
		l.MCPs = []MCPV2{}
	}
	sort.SliceStable(l.Artifacts, func(i, j int) bool {
		return l.Artifacts[i].Path < l.Artifacts[j].Path
	})
	sort.SliceStable(l.MCPs, func(i, j int) bool {
		return l.MCPs[i].Name < l.MCPs[j].Name
	})
}

// ValidationError reports the first canonicality violation found by
// ValidateV2 or ValidateLockV2.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid v2 metadata: %s: %s", e.Field, e.Reason)
}

// ValidateV2 enforces the canonical form of the v2 state document: required
// identifiers, a canonical absolute and cleaned OpenCode root, closed enums,
// unique artifact paths and MCP names, canonical artifact sha256 and
// versioned MCP digests, and relative paths without absolute, traversal,
// ambiguous Win32, or alias forms. Uniqueness follows the platform's path
// case policy: case-fold-equivalent artifact paths, MCP config paths, and
// MCP names collide on case-insensitive platforms (Windows, macOS) while
// remaining distinct on case-sensitive ones. MCP digests must be exactly the
// encoding of the supported installmeta version: legacy raw-hex or
// unknown-version values are rejected so consumers fail closed instead of
// degrading. Invalid documents are rejected before any write; Save and Load
// both pass through this single validation.
func ValidateV2(meta MetadataV2) error {
	meta.Normalize()
	if err := validateCore(meta.SchemaVersion, meta.OpencodeRoot, meta.TransactionID,
		meta.Artifacts, meta.MCPs); err != nil {
		return err
	}
	if meta.Migration != nil && meta.Migration.Source == "" {
		return &ValidationError{Field: "migration.source", Reason: "empty"}
	}
	return nil
}

// ValidateLockV2 enforces the same canonical form for the v2 lock document.
func ValidateLockV2(lock LockV2) error {
	lock.Normalize()
	return validateCore(lock.SchemaVersion, lock.OpencodeRoot, lock.TransactionID,
		lock.Artifacts, lock.MCPs)
}

func validateCore(schema int, root, txnID string, artifacts []ArtifactV2, mcps []MCPV2) error {
	if schema != MetadataSchemaV2 {
		return &ValidationError{Field: "schema_version",
			Reason: fmt.Sprintf("expected %d, got %d", MetadataSchemaV2, schema)}
	}
	if strings.TrimSpace(root) == "" {
		return &ValidationError{Field: "opencode_root", Reason: "empty"}
	}
	// The root must be stored in canonical absolute form: absolute in the
	// host's path semantics, cleaned (no redundant separators, "." or ".."
	// elements, no trailing separator), and therefore identical to its own
	// Clean() so every consumer resolves the same root on every platform.
	if !filepath.IsAbs(root) {
		return &ValidationError{Field: "opencode_root", Reason: "not an absolute path"}
	}
	if root != filepath.Clean(root) {
		return &ValidationError{Field: "opencode_root", Reason: "not in clean canonical form"}
	}
	if strings.TrimSpace(txnID) == "" {
		return &ValidationError{Field: "transaction_id", Reason: "empty"}
	}
	fold := caseInsensitivePaths()
	seenPaths := make(map[string]bool, len(artifacts))
	seenPathsFolded := make(map[string]bool, len(artifacts))
	for i, a := range artifacts {
		field := fmt.Sprintf("artifacts[%d]", i)
		if err := validateRelPath(field+".path", a.Path); err != nil {
			return err
		}
		if seenPaths[a.Path] {
			return &ValidationError{Field: field + ".path", Reason: "duplicate " + a.Path}
		}
		seenPaths[a.Path] = true
		// On case-insensitive platforms (Windows, macOS) two paths differing
		// only by case resolve to the same file and fail closed; on
		// case-sensitive platforms they stay distinct valid records.
		if fold {
			key := strings.ToLower(a.Path)
			if seenPathsFolded[key] {
				return &ValidationError{Field: field + ".path",
					Reason: "case-colliding duplicate " + a.Path}
			}
			seenPathsFolded[key] = true
		}
		if !validArtifactKind[a.Kind] {
			return &ValidationError{Field: field + ".kind", Reason: "unknown kind " + string(a.Kind)}
		}
		if strings.TrimSpace(a.Origin) == "" {
			return &ValidationError{Field: field + ".origin", Reason: "empty"}
		}
		if !isHex64(a.Digest) {
			return &ValidationError{Field: field + ".digest", Reason: "not sha256 hex"}
		}
		if a.SourceDigest != "" && !isHex64(a.SourceDigest) {
			return &ValidationError{Field: field + ".source_digest", Reason: "not sha256 hex"}
		}
		if !validOwnership[a.Ownership] {
			return &ValidationError{Field: field + ".ownership", Reason: "unknown ownership " + string(a.Ownership)}
		}
		if a.Prior != nil && a.Prior.Digest != "" && !isHex64(a.Prior.Digest) {
			return &ValidationError{Field: field + ".prior.digest", Reason: "not sha256 hex"}
		}
		if a.BackupRef != "" {
			if err := validateRelPath(field+".backup_ref", a.BackupRef); err != nil {
				return err
			}
		}
	}
	seenNames := make(map[string]bool, len(mcps))
	seenNamesFolded := make(map[string]bool, len(mcps))
	seenConfigFolded := make(map[string]string, len(mcps))
	for i, m := range mcps {
		field := fmt.Sprintf("mcps[%d]", i)
		if strings.TrimSpace(m.Name) == "" {
			return &ValidationError{Field: field + ".name", Reason: "empty"}
		}
		if seenNames[m.Name] {
			return &ValidationError{Field: field + ".name", Reason: "duplicate " + m.Name}
		}
		seenNames[m.Name] = true
		if fold {
			key := strings.ToLower(m.Name)
			if seenNamesFolded[key] {
				return &ValidationError{Field: field + ".name",
					Reason: "case-colliding duplicate " + m.Name}
			}
			seenNamesFolded[key] = true
		}
		if err := validateRelPath(field+".config_path", m.ConfigPath); err != nil {
			return err
		}
		// Several managed MCP servers legitimately share one config file, so
		// an exactly repeated config path is canonical. On case-insensitive
		// platforms two spellings folding to the same file are inconsistent
		// evidence about one file and fail closed.
		if fold {
			key := strings.ToLower(m.ConfigPath)
			if first, seen := seenConfigFolded[key]; seen {
				if first != m.ConfigPath {
					return &ValidationError{Field: field + ".config_path",
						Reason: "case-colliding duplicate " + m.ConfigPath}
				}
			} else {
				seenConfigFolded[key] = m.ConfigPath
			}
		}
		if !installmeta.ValidMCPServerDigest(m.SemanticDigest) {
			return &ValidationError{Field: field + ".semantic_digest",
				Reason: fmt.Sprintf("not the canonical mcpv%d digest of the current encoding", installmeta.MCPDigestVersion)}
		}
		if !validOwnership[m.Ownership] {
			return &ValidationError{Field: field + ".ownership", Reason: "unknown ownership " + string(m.Ownership)}
		}
	}
	return nil
}

// validateRelPath enforces the canonical relative path form used by every
// path field in v2 documents: non-empty, slash-separated, cleaned, relative,
// and strictly beneath the root (no absolute paths, no traversal, no ".",
// no backslashes, no NUL bytes). On Windows it additionally rejects forms
// whose filesystem meaning is ambiguous: alternate data streams, reserved
// DOS device names, trailing dot or space elements, and 8.3 short-name
// aliases. Device and UNC forms cannot reach this check: they are absolute
// or backslash-separated and rejected structurally on every platform.
func validateRelPath(field, p string) error {
	if p == "" {
		return &ValidationError{Field: field, Reason: "empty"}
	}
	if strings.Contains(p, "\\") {
		return &ValidationError{Field: field, Reason: "backslash separator"}
	}
	if path.IsAbs(p) {
		return &ValidationError{Field: field, Reason: "absolute path"}
	}
	// Windows drive-letter forms ("C:/x", "C:x") are absolute or
	// volume-relative on Windows even with slash separators; the canonical
	// slash-separated form must never carry them.
	if len(p) >= 2 && isASCIILetter(p[0]) && p[1] == ':' {
		return &ValidationError{Field: field, Reason: "absolute path"}
	}
	if c := path.Clean(p); c != p {
		return &ValidationError{Field: field, Reason: "not clean path form"}
	}
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return &ValidationError{Field: field, Reason: "traversal or root reference"}
	}
	if strings.ContainsRune(p, 0) {
		return &ValidationError{Field: field, Reason: "NUL byte"}
	}
	if err := validateWin32Ambiguity(field, p); err != nil {
		return err
	}
	return nil
}

// validateWin32Ambiguity rejects relative path forms whose meaning is
// ambiguous only on Windows filesystems; on every other platform such names
// are ordinary valid files and stay accepted. It covers alternate data
// streams (any remaining colon: the drive-letter form was already rejected
// above), reserved DOS device names in any element, elements with a trailing
// dot or space that Win32 strips on lookup, and generated 8.3 short-name
// aliases that can resolve to a different long name.
func validateWin32Ambiguity(field, p string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	for _, elem := range strings.Split(p, "/") {
		if strings.Contains(elem, ":") {
			return &ValidationError{Field: field,
				Reason: "alternate data stream or volume separator in " + p}
		}
		if windowsReservedName(elem) {
			return &ValidationError{Field: field,
				Reason: "reserved DOS device name in " + p}
		}
		if elem != "" && (elem[len(elem)-1] == '.' || elem[len(elem)-1] == ' ') {
			return &ValidationError{Field: field,
				Reason: "trailing dot or space in " + p}
		}
		if shortNameAlias(elem) {
			return &ValidationError{Field: field,
				Reason: "Win32 8.3 short-name alias in " + p}
		}
	}
	return nil
}

// shortNameAlias reports whether one path element has the generated Windows
// 8.3 short-name form: a base of at most eight characters whose tail after
// the last tilde is one to four digits, and either no extension or exactly
// one extension of at most three characters from the 8.3 character set. An
// element whose extension is longer than three characters, carries a second
// dot, or uses characters Windows never emits in aliases ("foo~1.markdown",
// "foo~1.TXT.BAK", "foo~1.a b") cannot be a generated short name: it is an
// ordinary long filename and stays accepted. Such an element can alias a
// different long name on Windows and is rejected there; elsewhere it is an
// ordinary filename.
func shortNameAlias(elem string) bool {
	base, ext, hasExt := strings.Cut(elem, ".")
	if len(base) < 2 || len(base) > 8 {
		return false
	}
	tilde := strings.LastIndexByte(base, '~')
	if tilde <= 0 || strings.Contains(base[:tilde], "~") {
		return false
	}
	suffix := base[tilde+1:]
	if len(suffix) == 0 || len(suffix) > 4 {
		return false
	}
	for i := 0; i < len(suffix); i++ {
		if suffix[i] < '0' || suffix[i] > '9' {
			return false
		}
	}
	if !hasExt {
		// "PROGRA~1": generated aliases exist without any extension.
		return true
	}
	if len(ext) == 0 || len(ext) > 3 {
		return false
	}
	for i := 0; i < len(ext); i++ {
		if !validShortNameChar(ext[i]) {
			return false
		}
	}
	return true
}

// validShortNameChar reports whether b may appear in a generated 8.3 short
// name: ASCII letters, digits, and the symbol set Windows preserves when it
// synthesizes aliases. Anything else — spaces, separators, control or
// non-ASCII bytes — is evidence of a long name, not an alias.
func validShortNameChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '$', '%', '\'', '-', '_', '@', '~', '`', '!',
		'(', ')', '{', '}', '^', '#', '&':
		return true
	}
	return false
}

func isHex64(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// SaveMetadataV2 validates and atomically writes the v2 state file. It
// performs no directory creation and no legacy mutation; callers commit
// metadata only after target mutation succeeded. Invalid documents are
// rejected before any write.
func SaveMetadataV2(homeDir string, meta MetadataV2) error {
	meta.Normalize()
	meta.SchemaVersion = MetadataSchemaV2
	if err := ValidateV2(meta); err != nil {
		return err
	}
	data, err := marshalV2(meta)
	if err != nil {
		return fmt.Errorf("marshal state v2: %w", err)
	}
	if err := writeFileAtomic(StatePath(homeDir), data, 0o644); err != nil {
		return fmt.Errorf("write state v2: %w", err)
	}
	return nil
}

// SaveLockV2 validates and atomically writes the v2 lock file.
func SaveLockV2(homeDir string, lock LockV2) error {
	lock.Normalize()
	lock.SchemaVersion = MetadataSchemaV2
	if err := ValidateLockV2(lock); err != nil {
		return err
	}
	data, err := marshalV2(lock)
	if err != nil {
		return fmt.Errorf("marshal lock v2: %w", err)
	}
	if err := writeFileAtomic(LockPath(homeDir), data, 0o644); err != nil {
		return fmt.Errorf("write lock v2: %w", err)
	}
	return nil
}

// MetadataPresence classifies what was found on disk for a metadata file.
type MetadataPresence string

const (
	// PresenceAbsent means the file does not exist.
	PresenceAbsent MetadataPresence = "absent"
	// PresenceLegacy means a v1 document without "schema_version" was found.
	PresenceLegacy MetadataPresence = "legacy"
	// PresenceV2 means a v2 document was found and parsed.
	PresenceV2 MetadataPresence = "v2"
	// PresenceMalformed means the file exists but cannot be accepted: it is
	// unparseable, unreadable, or declares an unknown schema_version. It is
	// never treated as success and never rewritten.
	PresenceMalformed MetadataPresence = "malformed"
)

// MetadataLoad is the tolerant result of reading the state file. Reading
// legacy or malformed content never returns an error; callers inspect
// Presence and Detail to fail closed.
type MetadataLoad struct {
	Presence MetadataPresence
	Metadata MetadataV2
	// Legacy holds the parsed v1 state when Presence is legacy and the
	// document parsed successfully.
	Legacy *State
	// Detail describes why a document is malformed.
	Detail string
}

// LoadMetadataV2 reads the state file tolerantly. A truly absent
// "schema_version" field classifies as legacy; null, mistyped, or unknown
// values are rejected as malformed, and a v2 declaration is only accepted
// when the full document passes ValidateV2 after unmarshalling. Malformed
// documents are explicit outcomes, not errors, so planning can fail closed
// without touching the file.
func LoadMetadataV2(homeDir string) MetadataLoad {
	raw, presence, detail := readMetadataRaw(StatePath(homeDir))
	switch presence {
	case PresenceAbsent:
		return MetadataLoad{Presence: PresenceAbsent}
	case PresenceMalformed:
		return MetadataLoad{Presence: PresenceMalformed, Detail: detail}
	}
	version, known, detail := probeSchemaVersion(raw)
	switch {
	case known && version == MetadataSchemaV2:
		var meta MetadataV2
		if err := json.Unmarshal(raw, &meta); err != nil {
			return MetadataLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		if err := ValidateV2(meta); err != nil {
			return MetadataLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		return MetadataLoad{Presence: PresenceV2, Metadata: meta}
	case known:
		return MetadataLoad{Presence: PresenceMalformed, Detail: detail}
	default:
		var legacy State
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return MetadataLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		return MetadataLoad{Presence: PresenceLegacy, Legacy: &legacy}
	}
}

// LockLoad is the tolerant result of reading the lock file.
type LockLoad struct {
	Presence MetadataPresence
	Lock     LockV2
	// Legacy holds the parsed v1 lock when Presence is legacy and the
	// document parsed successfully.
	Legacy *Lockfile
	Detail string
}

// LoadLockV2 reads the lock file tolerantly, mirroring LoadMetadataV2: null,
// mistyped, or unknown schema versions are malformed, and a v2 declaration is
// accepted only when the full document passes ValidateLockV2.
func LoadLockV2(homeDir string) LockLoad {
	raw, presence, detail := readMetadataRaw(LockPath(homeDir))
	switch presence {
	case PresenceAbsent:
		return LockLoad{Presence: PresenceAbsent}
	case PresenceMalformed:
		return LockLoad{Presence: PresenceMalformed, Detail: detail}
	}
	version, known, detail := probeSchemaVersion(raw)
	switch {
	case known && version == MetadataSchemaV2:
		var lock LockV2
		if err := json.Unmarshal(raw, &lock); err != nil {
			return LockLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		if err := ValidateLockV2(lock); err != nil {
			return LockLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		return LockLoad{Presence: PresenceV2, Lock: lock}
	case known:
		return LockLoad{Presence: PresenceMalformed, Detail: detail}
	default:
		var legacy Lockfile
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return LockLoad{Presence: PresenceMalformed, Detail: err.Error()}
		}
		return LockLoad{Presence: PresenceLegacy, Legacy: &legacy}
	}
}

// readMetadataRaw reads one metadata file and returns a coarse presence:
// absent, malformed (with detail), or present for further classification.
func readMetadataRaw(path string) ([]byte, MetadataPresence, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, PresenceAbsent, ""
		}
		return nil, PresenceMalformed, err.Error()
	}
	return raw, PresenceLegacy, ""
}

// probeSchemaVersion reports the declared "schema_version" and whether the
// declaration is usable. known=false with an empty detail means the field is
// genuinely absent (legacy v1); known=false with a detail means the document
// is not valid JSON; known=true with a detail means the field is present but
// unacceptable — null, a non-integer type, or an unsupported version — and
// the document must be rejected as malformed, never degraded to legacy.
// Only a truly absent field classifies as legacy.
func probeSchemaVersion(raw []byte) (version int, known bool, detail string) {
	var probe struct {
		SchemaVersion json.RawMessage `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return 0, false, err.Error()
	}
	if len(probe.SchemaVersion) == 0 {
		return 0, false, ""
	}
	if strings.TrimSpace(string(probe.SchemaVersion)) == "null" {
		return 0, true, "schema_version is null; it must be the integer " +
			fmt.Sprint(MetadataSchemaV2)
	}
	var v int
	if err := json.Unmarshal(probe.SchemaVersion, &v); err != nil {
		return 0, true, "schema_version must be an integer: " + err.Error()
	}
	if v != MetadataSchemaV2 {
		return v, true, fmt.Sprintf("unsupported schema_version %d (expected %d)", v, MetadataSchemaV2)
	}
	return v, true, ""
}

// AgreementError reports the first field where state and lock disagree.
type AgreementError struct {
	Field         string
	MetadataValue string
	LockValue     string
}

func (e *AgreementError) Error() string {
	return fmt.Sprintf("state/lock disagreement on %q: state=%q lock=%q", e.Field, e.MetadataValue, e.LockValue)
}

// CheckAgreementV2 verifies that state and lock agree on the transaction and
// the complete managed artifact and MCP evidence: identity (path/name), kind,
// origin, installed and source digests, prior presence evidence, backup
// references, ownership, and config paths. Any disagreement fails closed
// with an *AgreementError; it is never reported as success.
func CheckAgreementV2(meta MetadataV2, lock LockV2) error {
	meta.Normalize()
	lock.Normalize()
	if meta.SchemaVersion != MetadataSchemaV2 || lock.SchemaVersion != MetadataSchemaV2 {
		return &AgreementError{Field: "schema_version",
			MetadataValue: fmt.Sprint(meta.SchemaVersion), LockValue: fmt.Sprint(lock.SchemaVersion)}
	}
	for _, c := range []struct{ field, m, l string }{
		{"opencode_root", meta.OpencodeRoot, lock.OpencodeRoot},
		{"transaction_id", meta.TransactionID, lock.TransactionID},
		{"backup_id", meta.BackupID, lock.BackupID},
	} {
		if c.m != c.l {
			return &AgreementError{Field: c.field, MetadataValue: c.m, LockValue: c.l}
		}
	}
	if err := artifactsAgree(meta.Artifacts, lock.Artifacts); err != nil {
		return err
	}
	if err := mcpsAgree(meta.MCPs, lock.MCPs); err != nil {
		return err
	}
	return nil
}

func artifactsAgree(meta, lock []ArtifactV2) error {
	if len(meta) != len(lock) {
		return &AgreementError{Field: "artifacts.length",
			MetadataValue: fmt.Sprint(len(meta)), LockValue: fmt.Sprint(len(lock))}
	}
	for i := range meta {
		m, l := meta[i], lock[i]
		for _, p := range []struct{ field, m, l string }{
			{"path", m.Path, l.Path},
			{"kind", string(m.Kind), string(l.Kind)},
			{"origin", m.Origin, l.Origin},
			{"digest", m.Digest, l.Digest},
			{"source_digest", m.SourceDigest, l.SourceDigest},
			{"ownership", string(m.Ownership), string(l.Ownership)},
			{"backup_ref", m.BackupRef, l.BackupRef},
		} {
			if p.m != p.l {
				return &AgreementError{Field: "artifacts." + p.field + "[" + fmt.Sprint(i) + "]",
					MetadataValue: p.m, LockValue: p.l}
			}
		}
		pm, pl := priorString(m.Prior), priorString(l.Prior)
		if pm != pl {
			return &AgreementError{Field: "artifacts.prior[" + fmt.Sprint(i) + "]",
				MetadataValue: pm, LockValue: pl}
		}
	}
	return nil
}

// priorString renders prior evidence for exact comparison: nil, absent, or
// "existed[:digest]". nil and a zero-value prior are distinguished so
// missing evidence can never pass for explicit no-prior evidence.
func priorString(p *ArtifactPrior) string {
	if p == nil {
		return "<nil>"
	}
	if !p.Existed && p.Digest == "" {
		return "<zero>"
	}
	if p.Digest == "" {
		return "existed"
	}
	return "existed:" + p.Digest
}

func mcpsAgree(meta, lock []MCPV2) error {
	if len(meta) != len(lock) {
		return &AgreementError{Field: "mcps.length",
			MetadataValue: fmt.Sprint(len(meta)), LockValue: fmt.Sprint(len(lock))}
	}
	for i := range meta {
		m, l := meta[i], lock[i]
		for _, p := range []struct{ field, m, l string }{
			{"name", m.Name, l.Name},
			{"config_path", m.ConfigPath, l.ConfigPath},
			{"semantic_digest", m.SemanticDigest, l.SemanticDigest},
			{"ownership", string(m.Ownership), string(l.Ownership)},
		} {
			if p.m != p.l {
				return &AgreementError{Field: "mcps." + p.field + "[" + fmt.Sprint(i) + "]",
					MetadataValue: p.m, LockValue: p.l}
			}
		}
	}
	return nil
}

func marshalV2(doc any) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// normalizeRelPath cleans a relative path and converts it to slash form so
// documents stay identical across platforms.
func normalizeRelPath(p string) string {
	if p == "" {
		return ""
	}
	return path.Clean(filepath.ToSlash(p))
}
