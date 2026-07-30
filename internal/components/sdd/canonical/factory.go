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

	renderer, err := rendererFor(input.Target, input.ForgeSpecMode, input.CapabilitySnapshotSHA256)
	if err != nil {
		return Product{}, err
	}
	return Product{
		Workflow: Workflow(), Renderer: renderer,
		AllowedAssetKinds: []renderers.AssetKind{
			renderers.AssetInstruction, renderers.AssetRule, renderers.AssetSkill,
			renderers.AssetCommand, renderers.AssetAgent, renderers.AssetPermission,
			renderers.AssetHook, renderers.AssetMCP, renderers.AssetModel,
			renderers.AssetSchema, renderers.AssetFixture,
		},
		AllowedPermissions: []string{
			"filesystem/read", "filesystem/write", "mcp/cortex", "mcp/forgespec",
			"process/execute", "tool/read", "tool/search",
		},
		PhaseSchemas: phasecontract.PhaseSchemas,
	}, nil
}

func rendererFor(target renderers.TargetID, mode manifest.CoordinationMode, capabilityDigest string) (renderers.Renderer, error) {
	switch target {
	case "antigravity":
		return renderers.NewAntigravityRenderer(), nil
	case "claude":
		return renderers.NewClaudeRenderer(canonicalManifestInput(mode, capabilityDigest)), nil
	case "codex":
		return renderers.NewCodexRenderer(), nil
	case "cursor":
		return renderers.NewCursorRenderer(), nil
	case "gemini":
		return renderers.NewGeminiRenderer(), nil
	case "kilocode":
		return renderers.NewKilocodeRenderer(), nil
	case "kimi":
		return renderers.NewKimiRenderer(), nil
	case "kiro":
		return renderers.NewKiroRenderer(), nil
	case "opencode":
		return renderers.NewOpenCodeRenderer(), nil
	case "qwen":
		return renderers.NewQwenRenderer(), nil
	case "vscode":
		return renderers.NewVSCodeRenderer(), nil
	case "windsurf":
		return renderers.NewWindsurfRenderer(), nil
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
