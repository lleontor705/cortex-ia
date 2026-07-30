// Package manifest emits deterministic, validated security and degradation
// disclosures for generated workflow bundles.
package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

type Versions struct {
	ManifestSchema ir.Version `json:"manifest_schema"`
	Compiler       ir.Version `json:"compiler"`
	WorkflowIR     ir.Version `json:"workflow_ir"`
	Workflow       ir.Version `json:"workflow"`
	Catalog        ir.Version `json:"catalog"`
}

type Evidence struct {
	ID           ir.SemanticID            `json:"id"`
	Class        capability.EvidenceClass `json:"class"`
	Reference    string                   `json:"reference"`
	Fresh        bool                     `json:"fresh"`
	Experimental bool                     `json:"experimental"`
	Confidence   capability.Confidence    `json:"confidence"`
}

type TrustBoundary struct {
	Class     ir.TrustClass `json:"class"`
	Authority bool          `json:"authority"`
	Rules     []string      `json:"rules"`
}

type SecretReference struct {
	ID       ir.SemanticID `json:"id"`
	Provider string        `json:"provider"`
}

type ServiceRequirement struct {
	ID       ir.SemanticID   `json:"id"`
	Owner    string          `json:"owner"`
	Versions ir.VersionRange `json:"versions"`
	Required bool            `json:"required"`
}

type AssetHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type ValidationStatus string

const (
	ValidationPassed ValidationStatus = "passed"
	ValidationFailed ValidationStatus = "failed"
)

type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Blocking bool     `json:"blocking"`
}

type Validation struct {
	Status   ValidationStatus `json:"status"`
	Findings []Finding        `json:"findings"`
}

type Degradation struct {
	CapabilityID ir.SemanticID           `json:"capability_id"`
	State        resolution.State        `json:"state"`
	Substitution capability.CapabilityID `json:"substitution,omitempty"`
	Consequence  string                  `json:"consequence"`
	Blocking     bool                    `json:"blocking"`
}

type CoordinationMode string

const (
	CoordinationDirectV1         CoordinationMode = "direct-v1"
	CoordinationLegacySequential CoordinationMode = "legacy-sequential"
	CoordinationBlocked          CoordinationMode = "blocked"
)

type RetiredEntry struct {
	ID     ir.SemanticID `json:"id"`
	Action string        `json:"action"`
	Source string        `json:"source"`
}

type Input struct {
	Versions                 Versions
	GenerationFingerprint    string
	Target                   string
	Profile                  string
	ForgeSpecMode            CoordinationMode
	CapabilitySnapshotSHA256 string
	Evidence                 []Evidence
	Resolutions              []resolution.Resolution
	RequestedPermissions     []string
	EffectivePermissions     []string
	Metadata                 json.RawMessage
	ApprovalIntent           string
	IsolationIntent          string
	TrustBoundaries          []TrustBoundary
	SecretReferences         []SecretReference
	Services                 []ServiceRequirement
	RetiredEntries           []RetiredEntry
	Hashes                   []AssetHash
	Degradations             []Degradation
	Validation               Validation
}

type Output struct {
	SecurityJSON        []byte
	SecurityMarkdown    []byte
	DegradationJSON     []byte
	DegradationMarkdown []byte
}

type commonManifest struct {
	Versions                 Versions                `json:"versions"`
	GenerationFingerprint    string                  `json:"generation_fingerprint"`
	Target                   string                  `json:"target"`
	Profile                  string                  `json:"profile"`
	ForgeSpecMode            CoordinationMode        `json:"forgespec_mode"`
	CapabilitySnapshotSHA256 string                  `json:"capability_snapshot_sha256"`
	Evidence                 []Evidence              `json:"evidence"`
	Resolutions              []resolution.Resolution `json:"resolutions"`
	UnsupportedRequirements  []ir.SemanticID         `json:"unsupported_requirements"`
	RequestedPermissions     []string                `json:"requested_permissions"`
	EffectivePermissions     []string                `json:"effective_permissions"`
	ApprovalIntent           string                  `json:"approval_intent"`
	IsolationIntent          string                  `json:"isolation_intent"`
	TrustBoundaries          []TrustBoundary         `json:"trust_boundaries"`
	SecretReferences         []SecretReference       `json:"secret_references"`
	Services                 []ServiceRequirement    `json:"services"`
	RetiredEntries           []RetiredEntry          `json:"retired_entries"`
	Hashes                   []AssetHash             `json:"hashes"`
	Validation               Validation              `json:"validation"`
	Metadata                 json.RawMessage         `json:"metadata,omitempty"`
}

type securityManifest struct {
	Kind string `json:"kind"`
	commonManifest
	Degradations []Degradation `json:"degradations"`
}

type degradationManifest struct {
	Kind string `json:"kind"`
	commonManifest
	Degradations []Degradation `json:"degradations"`
}

// Emit validates security claims before returning byte-stable machine and
// human manifests. It never mutates caller-owned input.
func Emit(input Input) (Output, error) {
	normalized := normalize(input)
	if err := validate(normalized); err != nil {
		return Output{}, err
	}
	common := commonManifest{
		Versions: normalized.Versions, GenerationFingerprint: normalized.GenerationFingerprint,
		Target: normalized.Target, Profile: normalized.Profile, ForgeSpecMode: normalized.ForgeSpecMode, CapabilitySnapshotSHA256: normalized.CapabilitySnapshotSHA256, Evidence: normalized.Evidence,
		Resolutions: normalized.Resolutions, UnsupportedRequirements: unsupportedRequirements(normalized.Resolutions), RequestedPermissions: normalized.RequestedPermissions,
		EffectivePermissions: normalized.EffectivePermissions, ApprovalIntent: normalized.ApprovalIntent,
		IsolationIntent: normalized.IsolationIntent, TrustBoundaries: normalized.TrustBoundaries,
		SecretReferences: normalized.SecretReferences, Services: normalized.Services, RetiredEntries: normalized.RetiredEntries,
		Hashes: normalized.Hashes, Validation: normalized.Validation,
		Metadata: normalized.Metadata,
	}
	security := securityManifest{Kind: "security", commonManifest: common, Degradations: normalized.Degradations}
	degradation := degradationManifest{Kind: "degradation", commonManifest: common, Degradations: normalized.Degradations}
	securityJSON, err := json.Marshal(security)
	if err != nil {
		return Output{}, fmt.Errorf("marshal security manifest: %w", err)
	}
	degradationJSON, err := json.Marshal(degradation)
	if err != nil {
		return Output{}, fmt.Errorf("marshal degradation manifest: %w", err)
	}
	return Output{
		SecurityJSON: securityJSON, SecurityMarkdown: renderMarkdown("Security Manifest", common, normalized.Degradations),
		DegradationJSON: degradationJSON, DegradationMarkdown: renderMarkdown("Degradation Manifest", common, normalized.Degradations),
	}, nil
}

func normalize(input Input) Input {
	result := input
	result.Target = strings.TrimSpace(input.Target)
	result.Profile = strings.TrimSpace(input.Profile)
	result.GenerationFingerprint = strings.ToLower(strings.TrimSpace(input.GenerationFingerprint))
	result.CapabilitySnapshotSHA256 = strings.ToLower(strings.TrimSpace(input.CapabilitySnapshotSHA256))
	result.Evidence = slices.Clone(input.Evidence)
	slices.SortFunc(result.Evidence, func(a, b Evidence) int { return strings.Compare(string(a.ID), string(b.ID)) })
	result.Resolutions = slices.Clone(input.Resolutions)
	for index := range result.Resolutions {
		item := &result.Resolutions[index]
		item.Evidence = sortedUnique(item.Evidence)
		item.Binding.Evidence = sortedUnique(item.Binding.Evidence)
		item.PermissionDelta.Added = sortedUnique(item.PermissionDelta.Added)
		item.PermissionDelta.Removed = sortedUnique(item.PermissionDelta.Removed)
		item.Binding.PermissionDelta.Added = sortedUnique(item.Binding.PermissionDelta.Added)
		item.Binding.PermissionDelta.Removed = sortedUnique(item.Binding.PermissionDelta.Removed)
	}
	slices.SortFunc(result.Resolutions, func(a, b resolution.Resolution) int { return strings.Compare(string(a.ID), string(b.ID)) })
	result.RequestedPermissions = sortedUnique(input.RequestedPermissions)
	result.EffectivePermissions = sortedUnique(input.EffectivePermissions)
	result.Metadata = slices.Clone(input.Metadata)
	result.TrustBoundaries = slices.Clone(input.TrustBoundaries)
	for index := range result.TrustBoundaries {
		result.TrustBoundaries[index].Rules = sortedUnique(result.TrustBoundaries[index].Rules)
	}
	slices.SortFunc(result.TrustBoundaries, func(a, b TrustBoundary) int { return strings.Compare(string(a.Class), string(b.Class)) })
	result.SecretReferences = slices.Clone(input.SecretReferences)
	slices.SortFunc(result.SecretReferences, func(a, b SecretReference) int { return strings.Compare(string(a.ID), string(b.ID)) })
	result.Services = slices.Clone(input.Services)
	slices.SortFunc(result.Services, func(a, b ServiceRequirement) int { return strings.Compare(string(a.ID), string(b.ID)) })
	result.RetiredEntries = slices.Clone(input.RetiredEntries)
	slices.SortFunc(result.RetiredEntries, func(a, b RetiredEntry) int {
		return strings.Compare(string(a.ID)+"\x00"+a.Action, string(b.ID)+"\x00"+b.Action)
	})
	result.Hashes = slices.Clone(input.Hashes)
	for index := range result.Hashes {
		result.Hashes[index].SHA256 = strings.ToLower(strings.TrimSpace(result.Hashes[index].SHA256))
	}
	slices.SortFunc(result.Hashes, func(a, b AssetHash) int { return strings.Compare(a.Path, b.Path) })
	result.Degradations = slices.Clone(input.Degradations)
	slices.SortFunc(result.Degradations, func(a, b Degradation) int { return strings.Compare(string(a.CapabilityID), string(b.CapabilityID)) })
	result.Validation.Findings = slices.Clone(input.Validation.Findings)
	slices.SortFunc(result.Validation.Findings, func(a, b Finding) int {
		if difference := strings.Compare(a.Code, b.Code); difference != 0 {
			return difference
		}
		return strings.Compare(a.Message, b.Message)
	})
	return result
}

func validate(input Input) error {
	for name, version := range map[string]ir.Version{
		"manifest schema": input.Versions.ManifestSchema, "compiler": input.Versions.Compiler,
		"workflow IR": input.Versions.WorkflowIR, "workflow": input.Versions.Workflow, "catalog": input.Versions.Catalog,
	} {
		if version.Major == 0 {
			return fmt.Errorf("%s version is required", name)
		}
	}
	if !validSHA256(input.GenerationFingerprint) {
		return fmt.Errorf("generation fingerprint must be a lowercase sha256 digest")
	}
	if !validSHA256(input.CapabilitySnapshotSHA256) {
		return fmt.Errorf("capability snapshot must be a lowercase sha256 digest")
	}
	if input.ForgeSpecMode != CoordinationDirectV1 && input.ForgeSpecMode != CoordinationLegacySequential && input.ForgeSpecMode != CoordinationBlocked {
		return fmt.Errorf("forgespec mode must be direct-v1, legacy-sequential, or blocked")
	}
	if input.Target == "" || input.Profile == "" {
		return fmt.Errorf("target and profile are required")
	}
	evidence := make(map[ir.SemanticID]struct{}, len(input.Evidence))
	for _, item := range input.Evidence {
		if err := ir.ValidateSemanticID(item.ID); err != nil {
			return err
		}
		if _, duplicate := evidence[item.ID]; duplicate {
			return fmt.Errorf("duplicate evidence %q", item.ID)
		}
		evidence[item.ID] = struct{}{}
		if item.Reference == "" || item.Confidence <= 0 || item.Confidence > 1 {
			return fmt.Errorf("evidence %q is incomplete", item.ID)
		}
		if containsSecretMaterial(item.Reference) {
			return fmt.Errorf("evidence %q contains secret material", item.ID)
		}
	}
	requested := stringSet(input.RequestedPermissions)
	for _, permission := range input.EffectivePermissions {
		if _, allowed := requested[permission]; !allowed {
			return fmt.Errorf("permission widening: effective permission %q was not requested", permission)
		}
	}
	for _, item := range input.Resolutions {
		if err := validateResolution(item, evidence, requested); err != nil {
			return err
		}
	}
	for _, boundary := range input.TrustBoundaries {
		if !validTrustClass(boundary.Class) {
			return fmt.Errorf("unknown trust class %q", boundary.Class)
		}
	}
	for _, secret := range input.SecretReferences {
		if err := ir.ValidateSemanticID(secret.ID); err != nil {
			return err
		}
		if secret.Provider == "" || containsSecretMaterial(secret.Provider) {
			return fmt.Errorf("secret reference %q is not opaque", secret.ID)
		}
	}
	for _, service := range input.Services {
		if err := ir.ValidateSemanticID(service.ID); err != nil {
			return err
		}
		if service.Owner == "" || service.Versions.Minimum.Major == 0 || service.Versions.Minimum.Major != service.Versions.MaximumTested.Major {
			return fmt.Errorf("service %q has incomplete owner or version interval", service.ID)
		}
	}
	for _, retired := range input.RetiredEntries {
		if err := ir.ValidateSemanticID(retired.ID); err != nil {
			return err
		}
		if strings.TrimSpace(retired.Action) == "" || strings.TrimSpace(retired.Source) == "" {
			return fmt.Errorf("retired entry %q must disclose action and source", retired.ID)
		}
	}
	for _, hash := range input.Hashes {
		if hash.Path == "" || !validSHA256(hash.SHA256) {
			return fmt.Errorf("asset %q must have a valid sha256 hash", hash.Path)
		}
	}
	for _, degradation := range input.Degradations {
		if err := ir.ValidateSemanticID(degradation.CapabilityID); err != nil {
			return err
		}
		if degradation.State == resolution.StateNative || degradation.Consequence == "" {
			return fmt.Errorf("degradation %q must disclose a non-native state and consequence", degradation.CapabilityID)
		}
		if degradation.State == resolution.StateEmulated && degradation.Substitution == "" {
			return fmt.Errorf("emulated degradation %q must disclose its substitution", degradation.CapabilityID)
		}
	}
	blocking := false
	for _, finding := range input.Validation.Findings {
		if finding.Code == "" || finding.Message == "" {
			return fmt.Errorf("validation finding is incomplete")
		}
		blocking = blocking || finding.Blocking || finding.Severity == SeverityError
	}
	if input.Validation.Status == ValidationPassed && blocking {
		return fmt.Errorf("validation status passed contradicts a blocking or error finding")
	}
	if input.Validation.Status != ValidationPassed && input.Validation.Status != ValidationFailed {
		return fmt.Errorf("validation status must be passed or failed")
	}
	return nil
}

func validateResolution(item resolution.Resolution, evidence map[ir.SemanticID]struct{}, requested map[string]struct{}) error {
	if err := ir.ValidateSemanticID(item.ID); err != nil {
		return err
	}
	if item.State != resolution.StateNative && item.State != resolution.StateEmulated && item.State != resolution.StateAdvisory && item.State != resolution.StateUnsupported {
		return fmt.Errorf("resolution %q has invalid state", item.ID)
	}
	for _, reference := range item.Evidence {
		if _, found := evidence[ir.SemanticID(reference)]; !found {
			return fmt.Errorf("resolution %q references unknown evidence %q", item.ID, reference)
		}
	}
	for _, permission := range item.PermissionDelta.Added {
		if _, allowed := requested[permission]; !allowed {
			return fmt.Errorf("permission widening: resolution %q adds unrequested permission %q", item.ID, permission)
		}
	}
	if item.State != resolution.StateUnsupported {
		if item.Binding.ID == "" || item.Binding.Enforcement == "" || len(item.Evidence) == 0 {
			return fmt.Errorf("resolution %q omits binding, enforcement, or evidence", item.ID)
		}
		if item.Guarantee == resolution.GuaranteeNone {
			return fmt.Errorf("resolution %q omits its guarantee", item.ID)
		}
	}
	if item.State == resolution.StateAdvisory {
		if item.Binding.Enforcement != capability.EnforcementPrompt && item.Binding.Enforcement != capability.EnforcementNone {
			return fmt.Errorf("advisory resolution %q claims non-advisory enforcement", item.ID)
		}
		if item.Guarantee == resolution.GuaranteeEnforced || item.Guarantee == resolution.GuaranteeEquivalent {
			return fmt.Errorf("advisory resolution %q cannot claim %q guarantee", item.ID, item.Guarantee)
		}
	}
	if item.State == resolution.StateNative && (item.Binding.Enforcement == capability.EnforcementPrompt || item.Binding.Enforcement == capability.EnforcementNone) {
		return fmt.Errorf("native resolution %q makes an unsupported enforcement claim", item.ID)
	}
	if item.State == resolution.StateEmulated && item.Binding.Enforcement == capability.EnforcementNone {
		return fmt.Errorf("emulated resolution %q omits its enforcement mechanism", item.ID)
	}
	if item.State == resolution.StateUnsupported && (item.Binding.Enforcement != "" && item.Binding.Enforcement != capability.EnforcementNone || item.Guarantee != resolution.GuaranteeNone) {
		return fmt.Errorf("unsupported resolution %q cannot claim enforcement", item.ID)
	}
	if item.Guarantee == resolution.GuaranteeEnforced && (item.Binding.Enforcement == capability.EnforcementPrompt || item.Binding.Enforcement == capability.EnforcementNone) {
		return fmt.Errorf("resolution %q makes an unsupported enforcement claim", item.ID)
	}
	if item.State == resolution.StateEmulated && item.Substitution == "" {
		return fmt.Errorf("emulated resolution %q must name its substitution", item.ID)
	}
	return nil
}

func renderMarkdown(title string, common commonManifest, degradations []Degradation) []byte {
	var output bytes.Buffer
	fmt.Fprintf(&output, "# %s\n\n", title)
	fmt.Fprintf(&output, "- Manifest schema: `%s`\n- Compiler: `%s`\n- Workflow IR: `%s`\n- Workflow: `%s`\n- Catalog: `%s`\n- Target/profile: `%s` / `%s`\n- ForgeSpec mode: `%s`\n- Capability snapshot (sha256): `%s`\n- Generation fingerprint (sha256): `%s`\n- Validation: **%s**\n\n", common.Versions.ManifestSchema, common.Versions.Compiler, common.Versions.WorkflowIR, common.Versions.Workflow, common.Versions.Catalog, common.Target, common.Profile, common.ForgeSpecMode, common.CapabilitySnapshotSHA256, common.GenerationFingerprint, common.Validation.Status)
	output.WriteString("## Capability resolutions and evidence\n\n")
	for _, item := range common.Resolutions {
		fmt.Fprintf(&output, "- `%s`: **%s**, enforcement `%s`, guarantee `%s`, substitution `%s`, evidence `%s`, reason: %s\n", item.ID, item.State, item.Binding.Enforcement, item.Guarantee, item.Substitution, joinEvidence(item.Evidence), item.Reason)
	}
	for _, item := range common.Evidence {
		fmt.Fprintf(&output, "  - `%s` (%s): `%s`; fresh=%t; experimental=%t; confidence=%g\n", item.ID, item.Class, item.Reference, item.Fresh, item.Experimental, item.Confidence)
	}
	output.WriteString("\n## Unsupported requirements\n\n")
	if len(common.UnsupportedRequirements) == 0 {
		output.WriteString("None.\n")
	}
	for _, item := range common.UnsupportedRequirements {
		fmt.Fprintf(&output, "- `%s`\n", item)
	}
	output.WriteString("\n## Permissions, approval, and isolation\n\n")
	fmt.Fprintf(&output, "- Requested permissions: `%s`\n- Effective permissions: `%s`\n- Approval intent: %s\n- Isolation intent: %s\n", strings.Join(common.RequestedPermissions, "`, `"), strings.Join(common.EffectivePermissions, "`, `"), common.ApprovalIntent, common.IsolationIntent)
	output.WriteString("\n## Trust boundaries and secret references\n\n")
	for _, item := range common.TrustBoundaries {
		fmt.Fprintf(&output, "- `%s`: authority=%t; %s\n", item.Class, item.Authority, strings.Join(item.Rules, "; "))
	}
	for _, item := range common.SecretReferences {
		fmt.Fprintf(&output, "- Secret reference `%s` via `%s` (opaque; value not rendered)\n", item.ID, item.Provider)
	}
	output.WriteString("\n## Services and versions\n\n")
	for _, item := range common.Services {
		fmt.Fprintf(&output, "- `%s` owner `%s`, versions `%s`, required=%t\n", item.ID, item.Owner, item.Versions.String(), item.Required)
	}
	output.WriteString("\n## Retired entries\n\n")
	if len(common.RetiredEntries) == 0 {
		output.WriteString("None.\n")
	}
	for _, item := range common.RetiredEntries {
		fmt.Fprintf(&output, "- `%s`: action `%s`, source `%s`\n", item.ID, item.Action, item.Source)
	}
	output.WriteString("\n## Asset hashes\n\n")
	for _, item := range common.Hashes {
		fmt.Fprintf(&output, "- `%s` sha256 `%s`\n", item.Path, item.SHA256)
	}
	output.WriteString("\n## Degradations\n\n")
	if len(degradations) == 0 {
		output.WriteString("None.\n")
	}
	for _, item := range degradations {
		fmt.Fprintf(&output, "- `%s`: **%s**, substitution `%s`, blocking=%t; %s\n", item.CapabilityID, item.State, item.Substitution, item.Blocking, item.Consequence)
	}
	output.WriteString("\n## Validation findings\n\n")
	if len(common.Validation.Findings) == 0 {
		output.WriteString("None.\n")
	}
	for _, item := range common.Validation.Findings {
		fmt.Fprintf(&output, "- `%s` [%s] blocking=%t: %s\n", item.Code, item.Severity, item.Blocking, item.Message)
	}
	return output.Bytes()
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func containsSecretMaterial(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"token=", "password=", "secret=", "authorization:", "begin private key", "sk-"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.User != nil
}

func validTrustClass(class ir.TrustClass) bool {
	switch class {
	case ir.TrustTrustedPolicy, ir.TrustTrustedSchema, ir.TrustOperatorInput, ir.TrustRepositoryData, ir.TrustToolOutput, ir.TrustPeerMessage, ir.TrustRemoteUntrusted, ir.TrustSecretReference:
		return true
	default:
		return false
	}
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sortedUnique[T ~string](values []T) []T {
	result := slices.Clone(values)
	if result == nil {
		result = []T{}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func joinEvidence(values []resolution.EvidenceRef) string {
	items := make([]string, len(values))
	for index := range values {
		items[index] = string(values[index])
	}
	return strings.Join(items, "`, `")
}

func unsupportedRequirements(values []resolution.Resolution) []ir.SemanticID {
	result := make([]ir.SemanticID, 0)
	for _, item := range values {
		if item.State == resolution.StateUnsupported {
			result = append(result, item.ID)
		}
	}
	return result
}
