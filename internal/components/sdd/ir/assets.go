package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
)

// AssetScope identifies the authority that owns an installed asset.
type AssetScope string

const (
	ScopeWorkflowRoot   AssetScope = "workflow-root"
	ScopeExternalConfig AssetScope = "external-config"
)

// AssetPath is a typed destination. Relative is always interpreted beneath
// the declared root; Absolute is reserved for explicitly protected external
// configuration roots and is never used for portable workflow bundles.
type AssetPath struct {
	Scope    AssetScope `json:"scope"`
	RootID   SemanticID `json:"root_id"`
	Relative string     `json:"relative"`
	Absolute string     `json:"absolute,omitempty"`
}

// AssetPathRoot describes one install root without collapsing workflow and
// externally-owned configuration into a single string path.
type AssetPathRoot struct {
	Scope    AssetScope `json:"scope"`
	RootID   SemanticID `json:"root_id"`
	Relative string     `json:"relative,omitempty"`
	Absolute string     `json:"absolute,omitempty"`
}

// AdapterInstallRoots is the complete typed root model consumed by planning.
type AdapterInstallRoots struct {
	Workflow       AssetPathRoot  `json:"workflow"`
	Prompt         AssetPathRoot  `json:"prompt"`
	Skills         AssetPathRoot  `json:"skills"`
	Agents         AssetPathRoot  `json:"agents"`
	Commands       *AssetPathRoot `json:"commands,omitempty"`
	ExternalConfig *AssetPathRoot `json:"external_config,omitempty"`
}

// AssetPlanItem couples validated content with its typed destination.
type AssetPlanItem struct {
	Spec        AssetSpec   `json:"spec"`
	Destination AssetPath   `json:"destination"`
	Content     []byte      `json:"content"`
	Mode        fs.FileMode `json:"mode"`
	Permissions []string    `json:"permissions,omitempty"`
}

// AssetClass enumerates every retained operational asset type that the typed
// installation path compiles and installs. Unknown classes are rejected so that
// an asset can never reach installation without a recognized lowering target.
type AssetClass string

const (
	AssetRootIndex       AssetClass = "root-index"
	AssetRootModule      AssetClass = "root-module"
	AssetSharedContract  AssetClass = "shared-contract"
	AssetSkill           AssetClass = "skill"
	AssetCommand         AssetClass = "command"
	AssetRoleStub        AssetClass = "role-stub"
	AssetProfileOverlay  AssetClass = "profile-overlay"
	AssetQualityTemplate AssetClass = "quality-plan-template"
	AssetContractSchema  AssetClass = "contract-schema"
	AssetManifest        AssetClass = "manifest"
)

var retainedAssetClasses = map[AssetClass]struct{}{
	AssetRootIndex:       {},
	AssetRootModule:      {},
	AssetSharedContract:  {},
	AssetSkill:           {},
	AssetCommand:         {},
	AssetRoleStub:        {},
	AssetProfileOverlay:  {},
	AssetQualityTemplate: {},
	AssetContractSchema:  {},
	AssetManifest:        {},
}

// ValidateAssetClass rejects asset classes outside the retained set so that no
// untyped or foreign asset class can participate in typed compilation.
func ValidateAssetClass(class AssetClass) error {
	if _, ok := retainedAssetClasses[class]; !ok {
		return fmt.Errorf("asset class %q is not a retained operational asset type", class)
	}
	return nil
}

// AssetSpec describes one retained operational asset and its compile-time facts.
// A Required asset without a SHA256 fingerprint is invalid because required
// assets must be integrity-checked before mutation.
type AssetSpec struct {
	ID         SemanticID   `json:"id"`
	Class      AssetClass   `json:"class"`
	SourcePath string       `json:"source_path"`
	Required   bool         `json:"required"`
	Phase      SemanticID   `json:"phase,omitempty"`
	Skill      SemanticID   `json:"skill,omitempty"`
	Profiles   []SemanticID `json:"profiles,omitempty"`
	MaxTokens  int          `json:"max_tokens"`
	SHA256     string       `json:"sha256"`
}

// Validate enforces the invariant that every asset spec is well-formed and that
// required assets carry a content fingerprint before they can be installed.
func (s AssetSpec) Validate() error {
	if err := ValidateSemanticID(s.ID); err != nil {
		return fmt.Errorf("asset ID: %w", err)
	}
	if err := ValidateAssetClass(s.Class); err != nil {
		return err
	}
	if s.SourcePath == "" {
		return fmt.Errorf("asset %q source path is required", s.ID)
	}
	if s.MaxTokens < 0 {
		return fmt.Errorf("asset %q max tokens cannot be negative", s.ID)
	}
	if s.Required && s.SHA256 == "" {
		return fmt.Errorf("required asset %q must carry a SHA256 fingerprint", s.ID)
	}
	return nil
}

// AssetCatalogSchema is the compatibility contract for AssetCatalog documents.
var AssetCatalogSchema = SchemaContract{
	ID:      "schema/asset-catalog",
	Current: MustParseVersion("1.0.0"),
	Supported: VersionRange{
		Minimum:       MustParseVersion("1.0.0"),
		MaximumTested: MustParseVersion("1.0.0"),
	},
}

// AssetCatalog is the complete typed inventory of retained operational assets
// consumed by the compiler and installer. A catalog without a mandatory
// root-index is invalid because the always-on root index is non-optional.
type AssetCatalog struct {
	SchemaVersion Version     `json:"schema_version"`
	Assets        []AssetSpec `json:"assets"`
}

// Validate enforces schema versioning, child validity, unique IDs, and the
// mandatory presence of the always-on root index before any mutation.
func (c AssetCatalog) Validate() error {
	if c.SchemaVersion.Major == 0 {
		return fmt.Errorf("asset catalog schema version is required")
	}
	if len(c.Assets) == 0 {
		return fmt.Errorf("asset catalog must declare at least one retained asset")
	}
	seen := make(map[SemanticID]struct{}, len(c.Assets))
	hasRootIndex := false
	for _, asset := range c.Assets {
		if err := asset.Validate(); err != nil {
			return err
		}
		if _, exists := seen[asset.ID]; exists {
			return fmt.Errorf("asset catalog contains duplicate ID %q", asset.ID)
		}
		seen[asset.ID] = struct{}{}
		if asset.Class == AssetRootIndex {
			hasRootIndex = true
		}
	}
	if !hasRootIndex {
		return fmt.Errorf("asset catalog must include a mandatory root-index asset")
	}
	return nil
}

// FingerprintContent computes the deterministic SHA-256 fingerprint of asset
// content as a lowercase hex string. The same content always yields the same
// fingerprint; the installer rejects any asset whose declared fingerprint
// differs from the content it actually lowers.
func FingerprintContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
