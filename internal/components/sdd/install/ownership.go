// Package install defines persistent installation ownership contracts without
// performing workflow execution or owning runtime state.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

const (
	OwnerCortexIA = "cortex-ia"
	MarkerPrefix  = "cortex-ia:ownership "

	ownershipSchemaVersion = "1.0.0"
	sidecarSuffix          = ".cortex-ia.json"
	baseSuffix             = ".cortex-ia.base"
)

var (
	ErrOwnershipNotFound    = errors.New("ownership metadata not found")
	ErrUnknownOwnership     = errors.New("destructive replacement blocked: ownership is unknown")
	ErrTakeoverNotDisclosed = errors.New("takeover blocked: destructive effects were not disclosed")
	ErrCustomizedContent    = errors.New("replacement blocked: managed content was customized")
	ErrCorruptOwnership     = errors.New("replacement blocked: ownership evidence is corrupt")
)

// Ownership is the stable identity and hash record shared by inline markers
// and sidecars. BaseSHA256 identifies the preserved install-state base used by
// later three-way merge; ContentSHA256 identifies the installed managed bytes.
type Ownership struct {
	SchemaVersion    string         `json:"schema_version"`
	Owner            string         `json:"owner"`
	Scope            OwnershipScope `json:"scope"`
	GeneratorVersion string         `json:"generator_version"`
	SemanticID       ir.SemanticID  `json:"semantic_id"`
	AssetPath        string         `json:"asset_path"`
	BaseSHA256       string         `json:"base_sha256"`
	ContentSHA256    string         `json:"content_sha256"`
}

type OwnershipScope string

const (
	OwnershipScopeAsset  OwnershipScope = "asset"
	OwnershipScopeRegion OwnershipScope = "region"
)

// NewOwnership validates stable identity fields and hashes owned copies of the
// generated base and installed content.
func NewOwnership(assetPath, generatorVersion string, semanticID ir.SemanticID, base, content []byte) (Ownership, error) {
	return newOwnership(OwnershipScopeAsset, assetPath, generatorVersion, semanticID, base, content)
}

// NewRegionOwnership creates marker metadata for one managed semantic region
// inside an otherwise user-owned asset.
func NewRegionOwnership(assetPath, generatorVersion string, semanticID ir.SemanticID, base, content []byte) (Ownership, error) {
	return newOwnership(OwnershipScopeRegion, assetPath, generatorVersion, semanticID, base, content)
}

func newOwnership(scope OwnershipScope, assetPath, generatorVersion string, semanticID ir.SemanticID, base, content []byte) (Ownership, error) {
	metadata := Ownership{
		SchemaVersion:    ownershipSchemaVersion,
		Owner:            OwnerCortexIA,
		Scope:            scope,
		GeneratorVersion: generatorVersion,
		SemanticID:       semanticID,
		AssetPath:        assetPath,
		BaseSHA256:       SHA256(base),
		ContentSHA256:    SHA256(content),
	}
	if err := metadata.Validate(); err != nil {
		return Ownership{}, err
	}
	return metadata, nil
}

// Validate rejects metadata that cannot be trusted as cortex-ia ownership
// evidence. Unknown schema or generator major versions are handled by callers
// as corruption rather than silently granting replacement authority.
func (o Ownership) Validate() error {
	if o.SchemaVersion != ownershipSchemaVersion {
		return fmt.Errorf("unsupported ownership schema %q", o.SchemaVersion)
	}
	if o.Owner != OwnerCortexIA {
		return fmt.Errorf("unexpected owner %q", o.Owner)
	}
	if o.Scope != OwnershipScopeAsset && o.Scope != OwnershipScopeRegion {
		return fmt.Errorf("invalid ownership scope %q", o.Scope)
	}
	if _, err := ir.ParseVersion(o.GeneratorVersion); err != nil {
		return fmt.Errorf("generator version: %w", err)
	}
	if err := ir.ValidateSemanticID(o.SemanticID); err != nil {
		return fmt.Errorf("semantic ID: %w", err)
	}
	if err := validateAssetPath(o.AssetPath); err != nil {
		return err
	}
	if !validSHA256(o.BaseSHA256) || !validSHA256(o.ContentSHA256) {
		return errors.New("ownership hashes must be lower-case SHA-256 values")
	}
	return nil
}

// SHA256 returns the canonical lower-case digest used by ownership records.
func SHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

// FormatOwnershipMarker produces target-compatible inline ownership metadata.
// Comment delimiters are caller-supplied because target syntaxes differ.
func FormatOwnershipMarker(commentStart, commentEnd string, metadata Ownership) (string, error) {
	if err := metadata.Validate(); err != nil {
		return "", err
	}
	if strings.ContainsAny(commentStart+commentEnd, "\r\n") {
		return "", errors.New("ownership marker delimiters must be single-line")
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal ownership marker: %w", err)
	}
	return commentStart + MarkerPrefix + string(encoded) + commentEnd, nil
}

// ParseOwnershipMarker extracts and validates inline metadata independently of
// the target's comment delimiters.
func ParseOwnershipMarker(marker string) (Ownership, error) {
	start := strings.Index(marker, MarkerPrefix)
	if start < 0 {
		return Ownership{}, ErrOwnershipNotFound
	}
	payload := marker[start+len(MarkerPrefix):]
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	var metadata Ownership
	if err := decoder.Decode(&metadata); err != nil {
		return Ownership{}, fmt.Errorf("decode ownership marker: %w", err)
	}
	if err := metadata.Validate(); err != nil {
		return Ownership{}, err
	}
	return metadata, nil
}

// OwnershipStore persists sidecar metadata and the exact generated base beside
// formats that cannot safely carry an inline marker.
type OwnershipStore struct{ root string }

func NewOwnershipStore(root string) OwnershipStore { return OwnershipStore{root: root} }

func (s OwnershipStore) Write(metadata Ownership, base []byte) error {
	if err := metadata.Validate(); err != nil {
		return err
	}
	if SHA256(base) != metadata.BaseSHA256 {
		return errors.New("install-state base does not match ownership base hash")
	}
	sidecar, basePath, err := s.paths(metadata.AssetPath, metadata.Scope, metadata.SemanticID)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ownership sidecar: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := writePrivateFile(basePath, base); err != nil {
		return fmt.Errorf("write install-state base: %w", err)
	}
	if err := writePrivateFile(sidecar, encoded); err != nil {
		return fmt.Errorf("write ownership sidecar: %w", err)
	}
	return nil
}

func (s OwnershipStore) Read(assetPath string) (Ownership, []byte, error) {
	return s.read(assetPath, OwnershipScopeAsset, "")
}

// ReadRegion retrieves sidecar metadata for one region when the target format
// cannot carry trustworthy inline markers.
func (s OwnershipStore) ReadRegion(assetPath string, semanticID ir.SemanticID) (Ownership, []byte, error) {
	if err := ir.ValidateSemanticID(semanticID); err != nil {
		return Ownership{}, nil, fmt.Errorf("semantic ID: %w", err)
	}
	return s.read(assetPath, OwnershipScopeRegion, semanticID)
}

func (s OwnershipStore) read(assetPath string, scope OwnershipScope, semanticID ir.SemanticID) (Ownership, []byte, error) {
	sidecar, basePath, err := s.paths(assetPath, scope, semanticID)
	if err != nil {
		return Ownership{}, nil, err
	}
	encoded, err := os.ReadFile(sidecar)
	if errors.Is(err, fs.ErrNotExist) {
		return Ownership{}, nil, ErrOwnershipNotFound
	}
	if err != nil {
		return Ownership{}, nil, fmt.Errorf("read ownership sidecar: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var metadata Ownership
	if err := decoder.Decode(&metadata); err != nil {
		return Ownership{}, nil, fmt.Errorf("decode ownership sidecar: %w", err)
	}
	if err := metadata.Validate(); err != nil {
		return Ownership{}, nil, err
	}
	if metadata.AssetPath != assetPath {
		return Ownership{}, nil, errors.New("ownership sidecar asset path does not match requested asset")
	}
	if metadata.Scope != scope || scope == OwnershipScopeRegion && metadata.SemanticID != semanticID {
		return Ownership{}, nil, errors.New("ownership sidecar identity does not match requested scope")
	}
	base, err := os.ReadFile(basePath)
	if err != nil {
		return Ownership{}, nil, fmt.Errorf("read install-state base: %w", err)
	}
	return metadata, base, nil
}

func (s OwnershipStore) paths(assetPath string, scope OwnershipScope, semanticID ir.SemanticID) (string, string, error) {
	if err := validateAssetPath(assetPath); err != nil {
		return "", "", err
	}
	local := filepath.FromSlash(assetPath)
	if scope == OwnershipScopeAsset {
		return filepath.Join(s.root, local+sidecarSuffix), filepath.Join(s.root, local+baseSuffix), nil
	}
	if scope != OwnershipScopeRegion {
		return "", "", fmt.Errorf("invalid ownership scope %q", scope)
	}
	regionSuffix := ".cortex-ia.region." + SHA256([]byte(semanticID))
	return filepath.Join(s.root, local+regionSuffix+".json"), filepath.Join(s.root, local+regionSuffix+".base"), nil
}

type OwnershipState string

const (
	OwnershipUnknown    OwnershipState = "unknown"
	OwnershipClean      OwnershipState = "clean"
	OwnershipCustomized OwnershipState = "customized"
	OwnershipCorrupt    OwnershipState = "corrupt"
)

type Inspection struct {
	State  OwnershipState
	Reason string
}

// InspectOwnership classifies current bytes without guessing. A valid base and
// metadata establish customization; invalid ownership evidence is corruption;
// absent evidence remains unknown.
func InspectOwnership(current []byte, metadata *Ownership, base []byte) Inspection {
	if metadata == nil {
		return Inspection{State: OwnershipUnknown, Reason: "ownership metadata is absent"}
	}
	if err := metadata.Validate(); err != nil {
		return Inspection{State: OwnershipCorrupt, Reason: err.Error()}
	}
	if SHA256(base) != metadata.BaseSHA256 {
		return Inspection{State: OwnershipCorrupt, Reason: "install-state base hash does not match ownership metadata"}
	}
	if SHA256(current) == metadata.ContentSHA256 {
		return Inspection{State: OwnershipClean, Reason: "managed content matches recorded content hash"}
	}
	return Inspection{State: OwnershipCustomized, Reason: "current content differs from the recorded managed content"}
}

type ReplacementRequest struct {
	Destructive       bool
	Takeover          bool
	TakeoverDisclosed bool
}

// AuthorizeReplacement is the destructive safety gate used by installation
// planning. Takeover is valid only when explicitly selected after disclosure.
func AuthorizeReplacement(inspection Inspection, request ReplacementRequest) error {
	if !request.Destructive {
		return nil
	}
	switch inspection.State {
	case OwnershipClean:
		return nil
	case OwnershipUnknown:
		if !request.Takeover {
			return ErrUnknownOwnership
		}
		if !request.TakeoverDisclosed {
			return ErrTakeoverNotDisclosed
		}
		return nil
	case OwnershipCustomized:
		return ErrCustomizedContent
	default:
		return ErrCorruptOwnership
	}
}

func validateAssetPath(assetPath string) error {
	if assetPath == "" || !fs.ValidPath(assetPath) || strings.ContainsAny(assetPath, `\:`) {
		return fmt.Errorf("invalid relative asset path %q", assetPath)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func writePrivateFile(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	return os.WriteFile(name, content, 0o600)
}
