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

// ErrAssetCollision reports two embedded assets claiming the same
// home-relative destination beneath the OpenCode config root.
var ErrAssetCollision = errors.New("opencode asset mapping collision")

// HasNativeKind reports whether an embedded asset kind has a native
// destination on OpenCode's discovery surface. Top-level _shared workflow
// contracts are compile-time data for Cortex, not installable files, so
// they carry no native destination and must be filtered out of a
// selection before mapping. It is a pure predicate: callers can never
// mutate global mapping policy.
func HasNativeKind(kind assets.Kind) bool {
	switch kind {
	case assets.KindAgentsDoc,
		assets.KindConfig,
		assets.KindAgent,
		assets.KindCommand,
		assets.KindSkill,
		assets.KindPlugin:
		return true
	default:
		return false
	}
}

// Destination returns the slash-separated home-relative destination for an
// embedded asset given its source path and structural kind.
func Destination(source string, kind assets.Kind) string {
	if kind == assets.KindSkill {
		return path.Join(NativeLayout().SkillsRoot, strings.TrimPrefix(source, "skills/"))
	}
	return path.Join(NativeLayout().ConfigRoot, source)
}

// DestinationForArtifact returns the slash-separated home-relative destination
// for an artifact given its recorded path and kind.
func DestinationForArtifact(source string, kind string) string {
	if kind == "skill" {
		return path.Join(NativeLayout().SkillsRoot, strings.TrimPrefix(source, "skills/"))
	}
	return path.Join(NativeLayout().ConfigRoot, source)
}

// IsNativeAsset reports whether an embedded inventory file maps to a
// native OpenCode destination. It is the single filtering predicate
// callers must use before MapAssets: it applies HasNativeKind and then
// validates the would-be destination against NativeLayout, the one
// declaration of the native surface, so selection and mapping can never
// disagree about plugin files, non-SKILL.md skill fragments such as
// skills/_shared, or any other off-surface path.
func IsNativeAsset(file assets.File) bool {
	if !HasNativeKind(file.Kind) {
		return false
	}
	dest := Destination(file.Path, file.Kind)
	return BeneathAllowedRoots(dest) && NativeLayout().IsNativePath(dest)
}

// Mapping pairs one embedded asset with its deterministic home-relative,
// slash-separated destination beneath the OpenCode config root or skills root.
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

// MapAssets maps embedded asset files beneath their native destinations.
// The mapping is pure and structure-preserving: every destination equals
// Destination(source, kind) and is validated against the same NativeLayout,
// the single source of the native surface. It fails closed on unsafe paths,
// kinds without a native destination, destinations off the native surface,
// kind/path disagreement, and destination collisions, and never returns a partial result.
// On platforms with case-insensitive filesystems, destinations that differ
// only by case are rejected as collisions.
func MapAssets(files []assets.File) ([]Mapping, error) {
	return mapAssets(files, caseInsensitiveFS())
}

func mapAssets(files []assets.File, foldCase bool) ([]Mapping, error) {
	layout := NativeLayout()
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
		if !HasNativeKind(kind) {
			return nil, fmt.Errorf("%w: %q of kind %s has no native OpenCode destination", assets.ErrUnmappedRoot, file.Path, kind)
		}
		dest := Destination(file.Path, kind)
		if !BeneathAllowedRoots(dest) {
			return nil, fmt.Errorf("%w: %q maps outside the allowed roots", assets.ErrUnsafePath, file.Path)
		}
		if !layout.IsNativePath(dest) {
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
// cleaning, denotes a path strictly beneath one of the declared native roots.
// Absolute paths and traversal outside the roots are rejected.
func BeneathAllowedRoots(dest string) bool {
	clean := path.Clean(strings.ReplaceAll(dest, "\\", "/"))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	layout := NativeLayout()
	return isBeneath(clean, layout.ConfigRoot) || isBeneath(clean, layout.SkillsRoot)
}

func isBeneath(clean, root string) bool {
	return clean == root || strings.HasPrefix(clean, root+"/")
}

// BeneathConfigRoot reports whether dest denotes a safe native destination root.
func BeneathConfigRoot(dest string) bool {
	return BeneathAllowedRoots(dest)
}
