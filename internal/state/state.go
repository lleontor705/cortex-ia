package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

const stateDir = ".cortex-ia"
const stateFile = "state.json"
const lockFile = "cortex-ia.lock"

// writeFileAtomic is replaceable by package tests to exercise metadata commit
// failures without risking a partially written state or lock file.
var writeFileAtomic = func(path string, data []byte, perm os.FileMode) error {
	_, err := filemerge.WriteFileAtomic(path, data, perm)
	return err
}

// State is the read-only legacy (v1) state document. The OpenCode-only
// product never writes it; it is parsed only to assess a legacy home during
// migration planning and to resolve the legacy last-backup fallback, so it
// keeps exactly the fields those fail-closed reads depend on. Unknown v1
// fields are ignored by design.
type State struct {
	InstalledAgents   []string        `json:"installed_agents"`
	Preset            string          `json:"preset,omitempty"`
	Components        []string        `json:"components,omitempty"`
	RegistrySelection json.RawMessage `json:"registry_selection,omitempty"`
	LastBackupID      string          `json:"last_backup_id,omitempty"`
}

// Lockfile is the read-only legacy (v1) lock document, mirroring State.
type Lockfile struct {
	InstalledAgents   []string        `json:"installed_agents"`
	Preset            string          `json:"preset,omitempty"`
	Components        []string        `json:"components,omitempty"`
	Files             []string        `json:"files,omitempty"`
	RegistrySelection json.RawMessage `json:"registry_selection,omitempty"`
	LastBackupID      string          `json:"last_backup_id,omitempty"`
}

// StatePath returns the path to the state file.
func StatePath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, stateFile)
}

// LockPath returns the path to the lock file.
func LockPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, lockFile)
}

// Load reads the legacy state file. Returns empty state if not found.
func Load(homeDir string) (State, error) {
	path := StatePath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

// LoadLock reads the legacy lock file. Returns empty lock if not found.
func LoadLock(homeDir string) (Lockfile, error) {
	path := LockPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Lockfile{}, nil
		}
		return Lockfile{}, fmt.Errorf("read lock: %w", err)
	}

	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return Lockfile{}, fmt.Errorf("parse lock: %w", err)
	}
	return lock, nil
}

// FingerprintSchemaV1 is the schema version of the MCP postimage fingerprint
// document, the local ownership sidecar of the v2 metadata.
const FingerprintSchemaV1 = 1

// fingerprintFile stores the per-home MCP postimage fingerprint sidecar
// beside the v2 state document. Compatibility contract: the salt is local to
// one home, is generated exactly once, is persisted ONLY here (never in the
// state.json/lock bodies, receipts, or logs), and is required to verify any
// mcpv2 fingerprint. A missing document means no mcpv2 evidence exists yet
// (legacy homes); a corrupt document blocks destructive MCP accreditation
// fail-closed instead of degrading.
const fingerprintFile = "mcp_fingerprint.json"

// fingerprintSaltBytes is the generated salt size (64 hex characters).
const fingerprintSaltBytes = 32

// FingerprintRecord binds one managed MCP name to its recorded mcpv2
// full-postimage fingerprint and the config file it was accredited in. It
// carries no URL, env, or header values: only the non-reversible keyed
// digest produced by internal/installmeta.
type FingerprintRecord struct {
	Name            string `json:"name"`
	ConfigPath      string `json:"config_path"`
	PostImageDigest string `json:"postimage_digest"`
}

// FingerprintDocument is the local MCP postimage fingerprint store. It is
// intentionally separate from the MetadataV2 body: pipeline commits rewrite
// state.json from the metadata struct, so a salt embedded there would be
// silently dropped, while this sidecar survives every state rewrite. It is
// never copied into the lock file or any receipt.
type FingerprintDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Salt          string              `json:"salt"`
	Records       []FingerprintRecord `json:"records"`
}

// FingerprintPath returns the path to the MCP fingerprint sidecar file.
func FingerprintPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, fingerprintFile)
}

// RandomFingerprintSalt returns one new local salt as 64 hex characters. The
// salt is generated with crypto/rand and is displayed nowhere; callers
// persist it only through SaveFingerprintDocument.
func RandomFingerprintSalt() (string, error) {
	raw := make([]byte, fingerprintSaltBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate MCP fingerprint salt: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// SaltBytes decodes the document salt for HMAC use. The document must have
// been loaded or saved through this package, which validates the hex form.
func (d FingerprintDocument) SaltBytes() ([]byte, error) {
	salt, err := hex.DecodeString(d.Salt)
	if err != nil {
		return nil, fmt.Errorf("state: MCP fingerprint salt is not valid hex")
	}
	if len(salt) < 16 {
		return nil, fmt.Errorf("state: MCP fingerprint salt is too short")
	}
	return salt, nil
}

// LoadFingerprintDocument reads the MCP fingerprint sidecar. A missing file
// is (zero, false, nil): the home simply has no mcpv2 evidence yet. A
// present but invalid document is (zero, true, err) so destructive callers
// fail closed on corrupt local evidence instead of treating it as absent.
func LoadFingerprintDocument(homeDir string) (FingerprintDocument, bool, error) {
	path := FingerprintPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FingerprintDocument{}, false, nil
		}
		return FingerprintDocument{}, true, fmt.Errorf("read MCP fingerprint document: %w", err)
	}
	var doc FingerprintDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return FingerprintDocument{}, true, fmt.Errorf("parse MCP fingerprint document: %w", err)
	}
	if err := validateFingerprintDocument(doc); err != nil {
		return FingerprintDocument{}, true, err
	}
	return doc, true, nil
}

// SaveFingerprintDocument validates and atomically writes the sidecar. The
// caller must ensure the .cortex-ia directory exists (any agreed v2 home
// does). The file is written with owner-only permissions and never
// synchronously through any receipt or log channel.
func SaveFingerprintDocument(homeDir string, doc FingerprintDocument) error {
	doc.SchemaVersion = FingerprintSchemaV1
	doc.Records = sortedFingerprintRecords(doc.Records)
	if err := validateFingerprintDocument(doc); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode MCP fingerprint document: %w", err)
	}
	return writeFileAtomic(FingerprintPath(homeDir), data, 0o600)
}

// UpsertFingerprintRecord replaces the record for one server name, or
// appends it, returning a name-sorted set.
func UpsertFingerprintRecord(doc FingerprintDocument, record FingerprintRecord) FingerprintDocument {
	records := make([]FingerprintRecord, 0, len(doc.Records)+1)
	replaced := false
	for _, existing := range doc.Records {
		if existing.Name == record.Name {
			records = append(records, record)
			replaced = true
			continue
		}
		records = append(records, existing)
	}
	if !replaced {
		records = append(records, record)
	}
	return FingerprintDocument{SchemaVersion: doc.SchemaVersion, Salt: doc.Salt, Records: sortedFingerprintRecords(records)}
}

// DropFingerprintRecord removes the record for one server name.
func DropFingerprintRecord(doc FingerprintDocument, name string) FingerprintDocument {
	records := make([]FingerprintRecord, 0, len(doc.Records))
	for _, existing := range doc.Records {
		if existing.Name != name {
			records = append(records, existing)
		}
	}
	return FingerprintDocument{SchemaVersion: doc.SchemaVersion, Salt: doc.Salt, Records: sortedFingerprintRecords(records)}
}

// HasFingerprintRecord reports whether a record exists for the server name.
func (d FingerprintDocument) HasFingerprintRecord(name string) bool {
	for _, existing := range d.Records {
		if existing.Name == name {
			return true
		}
	}
	return false
}

// sortedFingerprintRecords returns the records sorted by name.
func sortedFingerprintRecords(records []FingerprintRecord) []FingerprintRecord {
	sorted := make([]FingerprintRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

// validateFingerprintDocument enforces the closed sidecar schema: one known
// schema version, one sufficiently strong hex salt, and unique records whose
// digests are exactly the canonical mcpv2 encoding and whose config paths
// share the MetadataV2 relative-path form. No secret value is representable
// here by construction.
func validateFingerprintDocument(doc FingerprintDocument) error {
	if doc.SchemaVersion != FingerprintSchemaV1 {
		return fmt.Errorf("state: MCP fingerprint document schema_version %d is not supported", doc.SchemaVersion)
	}
	if _, err := doc.SaltBytes(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(doc.Records))
	for i, record := range doc.Records {
		field := fmt.Sprintf("records[%d]", i)
		if record.Name == "" {
			return fmt.Errorf("state: MCP fingerprint %s.name is empty", field)
		}
		if seen[record.Name] {
			return fmt.Errorf("state: MCP fingerprint %s.name duplicates %s", field, record.Name)
		}
		seen[record.Name] = true
		if err := validateRelPath(field+".config_path", record.ConfigPath); err != nil {
			return err
		}
		if !installmeta.ValidMCPServerPostImageDigest(record.PostImageDigest) {
			return fmt.Errorf("state: MCP fingerprint %s.postimage_digest is not the canonical mcpv%d encoding", field, installmeta.MCPPostImageDigestVersion)
		}
	}
	return nil
}
