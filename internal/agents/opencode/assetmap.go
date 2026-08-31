package opencode

import (
	"errors"
	"fmt"
	"path"
	"runtime"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

// ErrAssetCollision reports two embedded assets claiming the same managed
// home-relative destination.
var ErrAssetCollision = errors.New("opencode asset mapping collision")

// HasManagedKind reports whether an embedded asset kind has a managed
// OpenCode or Cortex-IA support destination. Shared contracts are installed
// below WorkflowRoot even though OpenCode does not discover them directly.
func HasManagedKind(kind assets.Kind) bool {
	switch kind {
	case assets.KindAgentsDoc,
		assets.KindConfig,
		assets.KindShared,
		assets.KindAgent,
		assets.KindCommand,
		assets.KindSkill,
		assets.KindPlugin,
		assets.KindTUI:
		return true
	default:
		return false
	}
}

// Destination returns the slash-separated home-relative destination for an
// embedded asset given its source path and structural kind.
func Destination(source string, kind assets.Kind) string {
	return DestinationWithHome(source, kind, "")
}

// DestinationWithHome returns the slash-separated home-relative destination for an
// embedded asset given its source path, structural kind, and target home directory.
func DestinationWithHome(source string, kind assets.Kind, homeDir string) string {
	if kind == assets.KindShared {
		return path.Join(NativeLayout().ContractRoot, sharedContractRelative(source))
	}
	if kind == assets.KindSkill {
		return path.Join(NativeLayout().SkillsRoot, strings.TrimPrefix(source, "skills/"))
	}
	if kind == assets.KindConfig {
		return NativeLayout().ResolveConfigRelPath(homeDir)
	}
	if kind == assets.KindPlugin {
		p := strings.TrimPrefix(source, "plugin/")
		p = strings.TrimPrefix(p, "plugins/")
		return path.Join(NativeLayout().PluginRoot, p)
	}
	if kind == assets.KindTUI {
		return path.Join(NativeLayout().TUIPluginRoot, strings.TrimPrefix(source, "tui/"))
	}
	return path.Join(NativeLayout().ConfigRoot, source)
}

// DestinationForArtifact returns the slash-separated home-relative destination
// for an artifact given its recorded path and kind.
func DestinationForArtifact(source string, kind string) string {
	return DestinationForArtifactWithHome(source, kind, "")
}

// DestinationForArtifactWithHome returns the slash-separated home-relative destination
// for an artifact given its recorded path, kind, and target home directory.
func DestinationForArtifactWithHome(source string, kind string, homeDir string) string {
	if kind == "shared" {
		return path.Join(NativeLayout().ContractRoot, sharedContractRelative(source))
	}
	if kind == "skill" {
		return path.Join(NativeLayout().SkillsRoot, strings.TrimPrefix(source, "skills/"))
	}
	if kind == "config" || kind == "mcp-config" || source == "opencode.jsonc" || source == "opencode.json" {
		return NativeLayout().ResolveConfigRelPath(homeDir)
	}
	if kind == "plugin" {
		p := strings.TrimPrefix(source, "plugin/")
		p = strings.TrimPrefix(p, "plugins/")
		return path.Join(NativeLayout().PluginRoot, p)
	}
	if kind == "tui" {
		return path.Join(NativeLayout().TUIPluginRoot, strings.TrimPrefix(source, "tui/"))
	}
	return path.Join(NativeLayout().ConfigRoot, source)
}

// IsManagedAsset reports whether an embedded inventory file maps to a managed
// OpenCode destination or the declared Cortex-IA contract root.
func IsManagedAsset(file assets.File) bool {
	if !HasManagedKind(file.Kind) {
		return false
	}
	dest := Destination(file.Path, file.Kind)
	return BeneathAllowedRoots(dest) && managedDestination(file.Kind, dest)
}

// Mapping pairs one embedded asset with its deterministic home-relative,
// slash-separated managed destination.
type Mapping struct {
	// Source is the embedded asset path, for example
	// "skills/implement/SKILL.md".
	Source string
	// Dest is the home-relative destination, for example
	// ".agents/skills/implement/SKILL.md".
	Dest string
	// Kind is the structural asset kind re-derived from Source.
	Kind assets.Kind
	// SHA256 is the content hash of the embedded source.
	SHA256 string
}

// MapAssets maps embedded asset files beneath their managed destinations.
func MapAssets(files []assets.File) ([]Mapping, error) {
	return MapAssetsForHome(files, "")
}

// MapAssetsForHome maps embedded asset files beneath their native destinations,
// resolving config destinations based on existing files in homeDir.
func MapAssetsForHome(files []assets.File, homeDir string) ([]Mapping, error) {
	return mapAssets(files, caseInsensitiveFS(), homeDir)
}

func mapAssets(files []assets.File, foldCase bool, homeDir string) ([]Mapping, error) {
	seen := make(map[string]string, len(files))
	seenFold := make(map[string]string, len(files))
	mapped := make([]Mapping, 0, len(files))
	for _, file := range files {
		kind, err := assets.Classify(file.Path)
		if err != nil {
			return nil, err
		}
		if kind != file.Kind {
			return nil, fmt.Errorf("opencode asset mapping: %q derives kind %s but carried %s", file.Path, kind, file.Kind)
		}
		if !HasManagedKind(kind) {
			return nil, fmt.Errorf("%w: %q of kind %s has no native OpenCode destination", assets.ErrUnmappedRoot, file.Path, kind)
		}
		dest := DestinationWithHome(file.Path, kind, homeDir)
		if !BeneathAllowedRoots(dest) {
			return nil, fmt.Errorf("%w: %q maps outside the allowed roots", assets.ErrUnsafePath, file.Path)
		}
		if !managedDestination(kind, dest) {
			return nil, fmt.Errorf("%w: %q of kind %s has no native OpenCode destination", assets.ErrUnmappedRoot, file.Path, kind)
		}
		if previous, dup := seen[dest]; dup {
			return nil, fmt.Errorf("%w: %q and %q both map to %q", ErrAssetCollision, previous, file.Path, dest)
		}
		if foldCase {
			folded := strings.ToLower(dest)
			if previous, dup := seenFold[folded]; dup {
				return nil, fmt.Errorf("%w: %q and %q collide case-insensitively on %q", ErrAssetCollision, previous, file.Path, dest)
			}
			seenFold[folded] = file.Path
		}
		seen[dest] = file.Path
		mapped = append(mapped, Mapping{Source: file.Path, Dest: dest, Kind: kind, SHA256: file.SHA256})
	}
	sort.Slice(mapped, func(i, j int) bool { return mapped[i].Dest < mapped[j].Dest })
	return mapped, nil
}

// caseInsensitiveFS reports whether destinations on the current platform
// compare case-insensitively, so distinct-cased paths such as Foo and foo
// would denote the same file and must be rejected as collisions.
func caseInsensitiveFS() bool {
	switch runtime.GOOS {
	case "windows", "darwin":
		return true
	default:
		return false
	}
}

// BeneathAllowedRoots reports whether dest, after slash normalization and
// cleaning, denotes a path strictly beneath one of the declared managed roots.
// Absolute paths and traversal outside the roots are rejected.
func BeneathAllowedRoots(dest string) bool {
	clean := path.Clean(strings.ReplaceAll(dest, "\\", "/"))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	layout := NativeLayout()
	return isBeneath(clean, layout.ConfigRoot) || isBeneath(clean, layout.SkillsRoot) || isBeneath(clean, layout.WorkflowRoot)
}

func isBeneath(clean, root string) bool {
	return clean == root || strings.HasPrefix(clean, root+"/")
}

func sharedContractRelative(source string) string {
	value := strings.TrimPrefix(source, "skills/_shared/")
	value = strings.TrimPrefix(value, "_shared/")
	return value
}

func managedDestination(kind assets.Kind, dest string) bool {
	layout := NativeLayout()
	if kind == assets.KindShared {
		return nativeMarkdownChild(dest, layout.ContractRoot)
	}
	if kind == assets.KindTUI {
		return nativeScriptChild(dest, layout.TUIPluginRoot)
	}
	return layout.IsNativePath(dest)
}

func nativeScriptChild(relative, root string) bool {
	value := strings.TrimPrefix(relative, root+"/")
	if value == relative || value == "" || strings.Contains(value, "/") {
		return false
	}
	ext := strings.ToLower(path.Ext(value))
	return ext == ".js" || ext == ".ts" || ext == ".tsx"
}
