// Package prompt defines the prompt-composition contracts that bind the typed
// operational asset catalog, adapter capabilities, profile plan, quality
// template, and model routes into a single composition input.
//
// Adapter renderers lower only destinations and qualified native syntax; the
// composition layer owns prompt layering, token/duplication validation, and the
// deterministic role-to-skill linkage required by REQ-INST-003.
package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// SkillLoadMode records whether a skill is loaded via qualified native preload
// or by a mandatory first-action fallback read.
type SkillLoadMode string

const (
	SkillModeNativePreload  SkillLoadMode = "native-preload"
	SkillModeNativeOnDemand SkillLoadMode = "native-on-demand"
	SkillModeFallbackRead   SkillLoadMode = "fallback-read"
)

// AdapterPromptContract captures one adapter's destination and qualified native
// capabilities. It is the lowering surface: renderers consume it to place assets
// at adapter-native paths and to decide preload versus fallback read.
type AdapterPromptContract struct {
	Target                  renderers.TargetID
	RootPath                string
	AgentPath               func(ir.SemanticID) string
	SkillRoot               string
	CommandRoot             string
	SupportsSlashCommands   bool
	NativeSkillPreload      bool
	NativeSkillOnDemand     bool
	NativeModelField        bool
	NativeWorktreeIsolation bool
	ExpandPath              func(root, relative string) (string, error)
}

// Validate requires every destination root and path function so that an adapter
// without a resolvable destination can never compose a bundle.
func (c AdapterPromptContract) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("adapter prompt contract requires a target")
	}
	if c.RootPath == "" {
		return fmt.Errorf("adapter prompt contract requires a root path")
	}
	if c.AgentPath == nil {
		return fmt.Errorf("adapter prompt contract requires an agent path resolver")
	}
	if c.SkillRoot == "" {
		return fmt.Errorf("adapter prompt contract requires a skill root")
	}
	if c.CommandRoot == "" {
		return fmt.Errorf("adapter prompt contract requires a command root")
	}
	if c.ExpandPath == nil {
		return fmt.Errorf("adapter prompt contract requires an expand-path function")
	}
	return nil
}

// SkillLoadMode returns native-preload only when the adapter has qualified
// native preload; otherwise the first action must read the installed skill
// (fallback-read). This implements REQ-INST-003's preload-vs-fallback rule.
func (c AdapterPromptContract) SkillLoadMode() SkillLoadMode {
	if c.NativeSkillOnDemand {
		return SkillModeNativeOnDemand
	}
	if c.NativeSkillPreload {
		return SkillModeNativePreload
	}
	return SkillModeFallbackRead
}

// canonicalSkillForRole is the deterministic one-to-one mapping from each
// canonical role to its single canonical skill. Exactly one skill per role is
// required by REQ-INST-003; ambiguity or absence blocks before phase effects.
var canonicalSkillForRole = map[ir.SemanticID]ir.SemanticID{
	"role/orchestrator": "skill/orchestrator",
	"role/bootstrap":    "skill/bootstrap",
	"role/explore":      "skill/investigate",
	"role/proposal":     "skill/draft-proposal",
	"role/spec":         "skill/write-specs",
	"role/design":       "skill/architect",
	"role/tasks":        "skill/decompose",
	"role/apply":        "skill/implement",
	"role/verify":       "skill/validate",
	"role/archive":      "skill/finalize",
	// The canonical IR retained these adapter-facing names; they are aliases
	// for the nine phase roles and must resolve to the same skill exactly once.
	"role/investigate":       "skill/investigate",
	"role/draft-proposal":    "skill/draft-proposal",
	"role/write-specs":       "skill/write-specs",
	"role/architect":         "skill/architect",
	"role/decompose":         "skill/decompose",
	"role/implement":         "skill/implement",
	"role/validate":          "skill/validate",
	"role/finalize":          "skill/finalize",
	"role/debate":            "skill/debate",
	"role/parallel-dispatch": "skill/parallel-dispatch",
}

// CanonicalSkillForRole resolves a phase role to its single canonical skill
// deterministically. An unknown or ambiguous role returns an error so that no
// role can start without exactly one bound skill.
func CanonicalSkillForRole(role ir.SemanticID) (ir.SemanticID, error) {
	skill, ok := canonicalSkillForRole[role]
	if !ok {
		return "", fmt.Errorf("role %q has no canonical skill binding", role)
	}
	return skill, nil
}

// SkillBinding binds exactly one role to one canonical skill with its load mode,
// fully expanded installed path, and content hash. A binding without a path or
// hash blocks before phase effects per REQ-INST-003.
type SkillBinding struct {
	Role  ir.SemanticID `json:"role"`
	Skill ir.SemanticID `json:"skill"`
	Mode  SkillLoadMode `json:"mode"`
	Path  string        `json:"path"`
	Hash  string        `json:"hash"`
}

// Validate requires a resolved skill, mode, path, and hash.
func (b SkillBinding) Validate() error {
	if _, err := CanonicalSkillForRole(b.Role); err != nil {
		return fmt.Errorf("skill binding role: %w", err)
	}
	if b.Mode == "" {
		return fmt.Errorf("skill binding for %q requires a load mode", b.Role)
	}
	if b.Path == "" {
		return fmt.Errorf("skill binding for %q requires an expanded path", b.Role)
	}
	if b.Hash == "" {
		return fmt.Errorf("skill binding for %q requires a content hash", b.Role)
	}
	return nil
}

// NewSkillBinding deterministically binds a role to its canonical skill under an
// adapter contract, computing the expanded skill path and load mode.
func NewSkillBinding(role ir.SemanticID, contract AdapterPromptContract) (SkillBinding, error) {
	skill, err := CanonicalSkillForRole(role)
	if err != nil {
		return SkillBinding{}, err
	}
	if err := contract.Validate(); err != nil {
		return SkillBinding{}, fmt.Errorf("skill binding contract: %w", err)
	}
	relative := strings.TrimPrefix(string(skill), "skill/") + "/SKILL.md"
	skillPath, err := contract.ExpandPath(contract.SkillRoot, relative)
	if err != nil {
		return SkillBinding{}, fmt.Errorf("expand skill path for %q: %w", role, err)
	}
	return SkillBinding{
		Role:  role,
		Skill: skill,
		Mode:  contract.SkillLoadMode(),
		Path:  skillPath,
		Hash:  FingerprintSkillBinding(role, skill, skillPath),
	}, nil
}

// FingerprintSkillBinding computes the deterministic identity fingerprint of a
// skill binding from its role, canonical skill, and expanded path. The compiler
// verifies that installed skill content matches this expected binding identity.
func FingerprintSkillBinding(role, skill ir.SemanticID, skillPath string) string {
	identity := string(role) + "|" + string(skill) + "|" + skillPath
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

// ModelRoute aliases the canonical provider-neutral resolution. No second route
// vocabulary is maintained at the prompt boundary.
type ModelRoute = modelroute.ResolvedRoute

// ModelTable is the composition-layer view of the model routing table. The full
// model routing resolver (modelroute package) lowers into this type.
type ModelTable struct {
	Routes []ModelRoute `json:"routes"`
}

// ModelFor resolves the model route for a role, returning an error if no route
// exists so that a role can never run with an unconfigured model.
func (t ModelTable) ModelFor(role ir.SemanticID) (ModelRoute, error) {
	for _, route := range t.Routes {
		if route.Role == role {
			if route.PrimaryID != "" {
				if err := route.Requested.Validate(); err != nil {
					return ModelRoute{}, fmt.Errorf("model route %q: %w", role, err)
				}
				if route.Primary.Provider == "" || route.Primary.Model == "" || len(route.Evidence) == 0 {
					return ModelRoute{}, fmt.Errorf("model route %q has incomplete resolution evidence", role)
				}
				return route, nil
			}
			return ModelRoute{}, fmt.Errorf("model route %q has incomplete resolution evidence", role)
		}
	}
	return ModelRoute{}, fmt.Errorf("no model route for role %q", role)
}

// CompositionInput is the complete input to the prompt composer: the workflow,
// typed asset catalog, adapter contract, profile plan, quality template, and
// model table. It references ir.AssetCatalog so composition is bound to the
// typed installation path.
type CompositionInput struct {
	Workflow        ir.WorkflowIR               `json:"workflow"`
	Catalog         ir.AssetCatalog             `json:"catalog"`
	Adapter         AdapterPromptContract       `json:"-"`
	Profile         quality.ProfilePlan         `json:"profile"`
	QualityTemplate quality.QualityPlanTemplate `json:"quality_template"`
	QualityPlan     quality.QualityPlan         `json:"quality_plan"`
	Models          ModelTable                  `json:"models"`
	Metadata        json.RawMessage             `json:"metadata,omitempty"`
}

// Validate requires a valid adapter contract and asset catalog so that
// composition can never proceed against an invalid or untyped installation.
func (i CompositionInput) Validate() error {
	if err := i.Adapter.Validate(); err != nil {
		return fmt.Errorf("composition adapter: %w", err)
	}
	if err := i.Catalog.Validate(); err != nil {
		return fmt.Errorf("composition catalog: %w", err)
	}
	return nil
}

// defaultExpandPath joins a root and relative path and rejects traversal so
// that no adapter destination can escape its declared root.
func defaultExpandPath(root, relative string) (string, error) {
	cleaned := path.Clean("/" + relative)
	if strings.Contains(relative, "..") {
		return "", fmt.Errorf("relative path %q contains a traversal segment", relative)
	}
	return path.Join(root, cleaned), nil
}

// validAdapterContract returns a contract usable in tests with safe defaults.
// (Kept minimal; production contracts are built by adapter qualification.)
func validAdapterContract() AdapterPromptContract {
	return AdapterPromptContract{
		Target:      "claude",
		RootPath:    ".claude",
		AgentPath:   func(id ir.SemanticID) string { return path.Join(".claude/agents", string(id)) },
		SkillRoot:   "internal/assets/skills",
		CommandRoot: "internal/assets/opencode/commands",
		ExpandPath:  defaultExpandPath,
	}
}
