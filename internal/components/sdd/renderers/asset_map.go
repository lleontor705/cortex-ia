package renderers

import (
	"fmt"
	"path"
	"strings"

	opencodelayout "github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// AdapterAssetMap is the declarative native lowering boundary for common
// semantic assets. It contains destinations only; composition remains the
// authority for content, permissions, models, and profile degradation.
type AdapterAssetMap struct {
	Target             TargetID
	WorkflowRoot       string
	SkillsRoot         string
	AgentsRoot         string
	RoleRoot           string
	CommandsRoot       string
	OverlayRoot        string
	QualityRoot        string
	ManifestRoot       string
	ModelRoot          string
	PermissionRoot     string
	ExternalConfigRoot string
	RootModuleRoot     string
	ContractRoot       string
	CompositionPath    string
	SlashCommands      bool
	NativeProfiles     map[string]bool
	state              *assetMapState
}

type assetMapState struct {
	semanticIDs  map[ir.SemanticID]string
	destinations map[string]ir.SemanticID
}

// Map lowers one common semantic asset to its adapter-relative destination.
// The optional path is the common, adapter-neutral path produced by
// composition. Renderers must not independently rewrite it.
func (m AdapterAssetMap) Map(id ir.SemanticID, class ir.AssetClass, relative string) (ir.AssetPath, error) {
	if err := ir.ValidateSemanticID(id); err != nil {
		return ir.AssetPath{}, fmt.Errorf("asset %q: %w", id, err)
	}
	if err := ir.ValidateAssetClass(class); err != nil {
		return ir.AssetPath{}, err
	}
	if strings.TrimSpace(relative) == "" || strings.HasPrefix(relative, "%") || strings.Contains(relative, "\\") || strings.Contains(relative, ":") {
		return ir.AssetPath{}, fmt.Errorf("asset %q has unsafe or external destination %q", id, relative)
	}
	clean := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ir.AssetPath{}, fmt.Errorf("asset %q destination %q escapes workflow root", id, relative)
	}
	if strings.HasPrefix(clean, "internal/") || strings.HasPrefix(clean, "src/") || strings.HasPrefix(clean, "testdata/") {
		return ir.AssetPath{}, fmt.Errorf("asset %q destination %q is source-shaped", id, relative)
	}
	root := m.rootFor(class)
	if m.Target == "opencode" && class == ir.AssetSkill && !isOpenCodeNativeSkill(clean) {
		root = m.ContractRoot
	}
	if root == "" {
		return ir.AssetPath{}, fmt.Errorf("asset %q has no destination root for class %q", id, class)
	}
	// Common materialization may already carry the adapter root. Strip it once
	// so a second lowering cannot produce a double-root destination.
	clean = strings.TrimPrefix(clean, strings.TrimSuffix(root, "/")+"/")
	clean = strings.TrimPrefix(clean, strings.TrimSuffix(m.WorkflowRoot, "/")+"/")
	if m.Target == "opencode" {
		clean = strings.TrimPrefix(clean, strings.TrimSuffix(opencodelayout.NativeLayout().ConfigRoot, "/")+"/")
		clean = trimOpenCodeSourcePrefix(class, clean)
	}
	for _, prefix := range []string{"skills/", "agents/", "roles/", "commands/", "overlays/", "quality/", "manifests/", "steering/"} {
		if strings.HasPrefix(clean, prefix) {
			clean = strings.TrimPrefix(clean, prefix)
			break
		}
	}
	destination := path.Join(root, clean)
	if strings.HasPrefix(destination, "internal/") || strings.HasPrefix(destination, "src/") || strings.HasPrefix(destination, "testdata/") {
		return ir.AssetPath{}, fmt.Errorf("asset %q destination %q is source-shaped", id, destination)
	}
	if m.state == nil {
		m.state = &assetMapState{semanticIDs: map[ir.SemanticID]string{}, destinations: map[string]ir.SemanticID{}}
	}
	if previous, ok := m.state.semanticIDs[id]; ok {
		return ir.AssetPath{}, fmt.Errorf("asset %q was already mapped to %q", id, previous)
	}
	if previous, ok := m.state.destinations[destination]; ok {
		return ir.AssetPath{}, fmt.Errorf("destination %q is already owned by asset %q", destination, previous)
	}
	m.state.semanticIDs[id] = destination
	m.state.destinations[destination] = id
	return ir.AssetPath{Scope: ir.ScopeWorkflowRoot, RootID: ir.SemanticID("root/" + string(m.Target)), Relative: destination}, nil
}

func (m AdapterAssetMap) rootFor(class ir.AssetClass) string {
	switch class {
	case ir.AssetRootIndex, ir.AssetRootModule:
		if m.RootModuleRoot != "" {
			return m.RootModuleRoot
		}
		return m.WorkflowRoot
	case ir.AssetSharedContract:
		if m.ContractRoot != "" {
			return m.ContractRoot
		}
		return m.WorkflowRoot
	case ir.AssetSkill:
		return m.SkillsRoot
	case ir.AssetCommand:
		if m.CommandsRoot != "" {
			return m.CommandsRoot
		}
		return m.WorkflowRoot
	case ir.AssetRoleStub:
		return m.RoleRoot
	case ir.AssetProfileOverlay:
		return m.OverlayRoot
	case ir.AssetQualityTemplate:
		return m.QualityRoot
	case ir.AssetContractSchema:
		if m.ContractRoot != "" {
			return m.ContractRoot
		}
		return m.ManifestRoot
	case ir.AssetManifest:
		return m.ManifestRoot
	default:
		return ""
	}
}

var adapterAssetMaps = map[TargetID]AdapterAssetMap{
	"claude":   {Target: "claude", WorkflowRoot: ".claude", SkillsRoot: ".claude/skills", AgentsRoot: ".claude/agents", RoleRoot: ".claude/.cortex-ia/roles", OverlayRoot: ".claude/overlays", QualityRoot: ".claude/quality", ManifestRoot: ".cortex-ia", ModelRoot: ".cortex-ia/models", PermissionRoot: ".cortex-ia/permissions", NativeProfiles: map[string]bool{"portable-sequential": true, "portable-flat": true, "native-advanced": true}},
	"opencode": openCodeAssetMap(),
	"vscode":   {Target: "vscode", WorkflowRoot: ".copilot", SkillsRoot: ".copilot/skills", AgentsRoot: ".copilot/agents", RoleRoot: ".copilot/.cortex-ia/roles", OverlayRoot: ".copilot/overlays", QualityRoot: ".copilot/quality", ManifestRoot: ".cortex-ia", ModelRoot: ".cortex-ia/models", PermissionRoot: ".cortex-ia/permissions", NativeProfiles: map[string]bool{"portable-sequential": true, "portable-flat": true, "native-advanced": true}},
	"codex":    {Target: "codex", WorkflowRoot: ".codex", SkillsRoot: ".codex/skills", AgentsRoot: ".codex/agents", RoleRoot: ".codex/.cortex-ia/roles", OverlayRoot: ".codex/overlays", QualityRoot: ".codex/quality", ManifestRoot: ".cortex-ia", ModelRoot: ".cortex-ia/models", PermissionRoot: ".cortex-ia/permissions", NativeProfiles: map[string]bool{"portable-sequential": true, "portable-flat": true, "native-advanced": true}},
}

func openCodeAssetMap() AdapterAssetMap {
	layout := opencodelayout.NativeLayout()
	return AdapterAssetMap{
		Target: "opencode", WorkflowRoot: layout.WorkflowRoot, SkillsRoot: layout.SkillsRoot, AgentsRoot: layout.AgentsRoot,
		RoleRoot: layout.RoleRoot, CommandsRoot: layout.CommandsRoot, OverlayRoot: layout.OverlayRoot, QualityRoot: layout.QualityRoot,
		ManifestRoot: layout.ManifestRoot, ModelRoot: layout.ModelRoot, PermissionRoot: layout.PermissionRoot,
		RootModuleRoot: layout.RootModuleRoot, ContractRoot: layout.ContractRoot, CompositionPath: layout.CompositionPath,
		SlashCommands: true, NativeProfiles: map[string]bool{"portable-sequential": true, "portable-flat": true, "native-advanced": true},
	}
}

func isOpenCodeNativeSkill(relative string) bool {
	parts := strings.Split(strings.TrimPrefix(relative, "skills/"), "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "SKILL.md"
}

func trimOpenCodeSourcePrefix(class ir.AssetClass, relative string) string {
	prefixes := []string{}
	switch class {
	case ir.AssetRootIndex, ir.AssetRootModule:
		prefixes = []string{"generic/sdd-root/", "sdd-root/", "generic/", "root/"}
	case ir.AssetSharedContract, ir.AssetContractSchema:
		prefixes = []string{"skills/", "contracts/"}
	case ir.AssetRoleStub:
		prefixes = []string{"roles/"}
	case ir.AssetProfileOverlay:
		prefixes = []string{"generic/profiles/", "profiles/", "overlays/"}
	case ir.AssetQualityTemplate:
		prefixes = []string{"generated/", "quality/"}
	case ir.AssetManifest:
		prefixes = []string{"manifests/"}
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(relative, prefix) {
			return strings.TrimPrefix(relative, prefix)
		}
	}
	return relative
}

func AdapterAssetMapFor(target TargetID) (AdapterAssetMap, error) {
	assetMap, ok := adapterAssetMaps[target]
	if !ok {
		return AdapterAssetMap{}, fmt.Errorf("no declarative asset map for adapter %q", target)
	}
	assetMap.state = &assetMapState{semanticIDs: map[ir.SemanticID]string{}, destinations: map[string]ir.SemanticID{}}
	return assetMap, nil
}

func (m AdapterAssetMap) ValidateProfile(profile string) error {
	if profile != "portable-sequential" && profile != "portable-flat" && profile != "native-advanced" {
		return fmt.Errorf("unsupported profile %q", profile)
	}
	return nil
}
