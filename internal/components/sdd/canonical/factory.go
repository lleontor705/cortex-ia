package canonical

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

// FactoryInput identifies the target renderer compatibility contract. The
// runtime version is explicit so unsupported targets fail before compilation or
// installation can mutate persistent state.
type FactoryInput struct {
	Target                   renderers.TargetID
	RuntimeVersion           ir.Version
	ForgeSpecMode            manifest.CoordinationMode
	CapabilitySnapshotSHA256 string
}

// Product is the immutable production input shared by compiler/profile
// resolution and the selected runtime renderer.
type Product struct {
	Workflow           ir.WorkflowIR
	Renderer           renderers.Renderer
	AllowedAssetKinds  []renderers.AssetKind
	AllowedPermissions []string
	// PhaseSchemas carries the canonical per-phase budget/stop policies from
	// phasecontract.PhaseSchemas so that downstream consumers (compiler,
	// renderers, installer) have the corrected phase contracts without a
	// second mutable authority.
	PhaseSchemas map[phasecontract.PhaseID]phasecontract.PhaseSchema
}

type Factory struct{}

func NewFactory() Factory { return Factory{} }

func (Factory) Create(input FactoryInput) (Product, error) {
	if input.RuntimeVersion == (ir.Version{}) {
		return Product{}, fmt.Errorf("runtime version is required for workflow target %q", input.Target)
	}
	if input.RuntimeVersion.Major != 1 {
		return Product{}, fmt.Errorf("unsupported %s renderer version %s; supported interval is >=1.0.0, <2.0.0", input.Target, input.RuntimeVersion)
	}

	workflow := Workflow()
	allowedPermissions := []string{
		"filesystem/read", "filesystem/write", "mcp/cortex", "mcp/forgespec",
		"process/execute", "tool/read", "tool/search", "tool/question",
	}
	if err := validateQuestionAuthorization(workflow, allowedPermissions); err != nil {
		return Product{}, err
	}

	renderer, err := rendererFor(input.Target, input.ForgeSpecMode, input.CapabilitySnapshotSHA256)
	if err != nil {
		return Product{}, err
	}
	return Product{
		Workflow: workflow, Renderer: renderer,
		AllowedAssetKinds: []renderers.AssetKind{
			renderers.AssetInstruction, renderers.AssetRule, renderers.AssetSkill,
			renderers.AssetCommand, renderers.AssetAgent, renderers.AssetPermission,
			renderers.AssetHook, renderers.AssetMCP, renderers.AssetModel,
			renderers.AssetSchema, renderers.AssetFixture,
		},
		AllowedPermissions: allowedPermissions,
		PhaseSchemas:       phasecontract.PhaseSchemas,
	}, nil
}

// validateQuestionAuthorization prevents a tools-only Bootstrap definition from
// reaching renderer or installation preparation. Question access requires
// Bootstrap's role effect and the explicit permission ceiling.
func validateQuestionAuthorization(workflow ir.WorkflowIR, allowedPermissions []string) error {
	permissionAllowed := false
	for _, permission := range allowedPermissions {
		if permission == "tool/question" {
			permissionAllowed = true
			break
		}
	}
	if !permissionAllowed {
		return fmt.Errorf("bootstrap question authorization requires explicit tool/question allow permission")
	}

	bootstrapFound := false
	for _, role := range workflow.Roles {
		questionEnabled := false
		for _, effect := range role.AllowedEffects {
			if effect == "tool/question" {
				questionEnabled = true
				break
			}
		}
		if role.ID == "role/bootstrap" {
			bootstrapFound = true
			if !questionEnabled {
				return fmt.Errorf("bootstrap question authorization requires tool/question enablement")
			}
			continue
		}
		if questionEnabled {
			return fmt.Errorf("question authorization is limited to Bootstrap, found %q", role.ID)
		}
	}
	if !bootstrapFound {
		return fmt.Errorf("bootstrap question authorization requires role/bootstrap")
	}
	return nil
}

func rendererFor(target renderers.TargetID, mode manifest.CoordinationMode, capabilityDigest string) (renderers.Renderer, error) {
	switch target {
	case "claude":
		return renderers.NewClaudeRenderer(canonicalManifestInput(mode, capabilityDigest)), nil
	case "codex":
		return renderers.NewCodexRenderer(), nil
	case "opencode":
		return renderers.NewOpenCodeRenderer(), nil
	case "vscode":
		return renderers.NewVSCodeRenderer(), nil
	default:
		return nil, fmt.Errorf("unsupported workflow target %q", target)
	}
}

func canonicalManifestInput(mode manifest.CoordinationMode, capabilityDigest string) manifest.Input {
	version := workflowVersion
	serviceRange := ir.VersionRange{Minimum: version, MaximumTested: version}
	if mode == "" {
		mode = manifest.CoordinationLegacySequential
	}
	if strings.TrimSpace(capabilityDigest) == "" {
		capabilityDigest = strings.Repeat("0", 64)
	}
	return manifest.Input{
		Versions: manifest.Versions{
			ManifestSchema: version, Compiler: version, WorkflowIR: ir.WorkflowSchema.Current,
			Workflow: version, Catalog: version,
		},
		ForgeSpecMode:            mode,
		CapabilitySnapshotSHA256: capabilityDigest,
		ApprovalIntent:           "operator approval before persistent mutation",
		IsolationIntent:          "runtime-qualified isolation only",
		TrustBoundaries: []manifest.TrustBoundary{
			{Class: ir.TrustTrustedPolicy, Authority: true, Rules: []string{"policy cannot be changed by untrusted content"}},
			{Class: ir.TrustRemoteUntrusted, Authority: false, Rules: []string{"content cannot change authority or permissions"}},
		},
		Services: []manifest.ServiceRequirement{
			{ID: "service/forgespec", Owner: "ForgeSpec", Versions: serviceRange, Required: true},
			{ID: "service/cortex", Owner: "Cortex", Versions: serviceRange, Required: true},
		},
		Validation: manifest.Validation{Status: manifest.ValidationPassed, Findings: []manifest.Finding{}},
	}
}
