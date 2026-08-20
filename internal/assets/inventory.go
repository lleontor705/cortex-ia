package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Kind classifies an embedded asset by the structural root it occupies in
// the embedded tree. Kinds are derived from path shape only: this package
// never hardcodes per-agent, per-skill, or per-command catalogs.
type Kind string

const (
	// KindAgentsDoc is the AGENTS.md system prompt at the embedded root.
	KindAgentsDoc Kind = "agents-doc"
	// KindConfig is the opencode.jsonc settings template at the embedded
	// root.
	KindConfig Kind = "config"
	// KindShared is a top-level workflow contract under _shared/.
	KindShared Kind = "shared"
	// KindAgent is a sub-agent definition under agents/.
	KindAgent Kind = "agent"
	// KindCommand is a slash-command definition under commands/.
	KindCommand Kind = "command"
	// KindSkill is a skill file under skills/.
	KindSkill Kind = "skill"
	// KindPlugin is an OpenCode plugin file under plugin/.
	KindPlugin Kind = "plugin"
)

// Asset file classification and inventory errors. They are wrapped with
// the offending path and fail closed: callers never receive a partial
// inventory that silently drops a rejected entry.
var (
	// ErrUnsafePath reports an absolute, traversing, backslashed, or
	// otherwise non-canonical relative asset path.
	ErrUnsafePath = errors.New("unsafe asset path")
	// ErrUnmappedRoot reports a path whose top-level segment is not a
	// known asset root.
	ErrUnmappedRoot = errors.New("unmapped asset root")
)

// File describes one embedded asset with its structural kind, size in
// bytes, and lowercase hex SHA-256 of its content.
type File struct {
	Path   string
	Kind   Kind
	Size   int64
	SHA256 string
}

// Inventory walks the embedded assets and returns every file sorted by
// path. Two calls over the same embedded FS return identical slices, so
// planning over an inventory is deterministic. Any unsafe or unmapped
// path aborts the walk with a wrapped error instead of being skipped.
func Inventory() ([]File, error) {
	return InventoryFS(FS)
}

// InventoryFS builds the Inventory of fsys. It accepts any read-only
// filesystem, typically the embedded FS, and applies the same
// classification, hashing, and ordering rules as Inventory.
func InventoryFS(fsys fs.FS) ([]File, error) {
	files := make([]File, 0)
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		kind, err := Classify(p)
		if err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("read asset %q: %w", p, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, File{
			Path:   p,
			Kind:   kind,
			Size:   int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk embedded assets: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Classify maps a slash-relative asset path to its Kind. The path must be
// canonical: already clean, relative, forward-slashed, volume-free, and
// free of traversal. A root file must be AGENTS.md or opencode.jsonc; a
// nested file must live one level or deeper under a known root.
func Classify(rel string) (Kind, error) {
	clean, err := SafeRelative(rel)
	if err != nil {
		return "", err
	}
	switch clean {
	case "AGENTS.md":
		return KindAgentsDoc, nil
	case "opencode.jsonc":
		return KindConfig, nil
	}
	root, rest, found := strings.Cut(clean, "/")
	if !found || rest == "" {
		return "", fmt.Errorf("%w: %q is not a known asset root", ErrUnmappedRoot, rel)
	}
	switch root {
	case "_shared":
		return KindShared, nil
	case "agents":
		return KindAgent, nil
	case "commands":
		return KindCommand, nil
	case "skills":
		return KindSkill, nil
	case "plugin":
		return KindPlugin, nil
	}
	return "", fmt.Errorf("%w: %q is not a known asset root", ErrUnmappedRoot, rel)
}

// SafeRelative validates that rel is a canonical relative slash path
// suitable as an embedded asset key: non-empty, free of backslashes,
// volume separators, absolute prefixes, dot segments, and traversal. It
// returns the validated path unchanged.
func SafeRelative(rel string) (string, error) {
	unsafe := func(reason string) (string, error) {
		return "", fmt.Errorf("%w: %q %s", ErrUnsafePath, rel, reason)
	}
	switch {
	case rel == "":
		return unsafe("is empty")
	case strings.Contains(rel, "\\"):
		return unsafe("contains a backslash")
	case path.IsAbs(rel):
		return unsafe("is absolute")
	}
	if i := strings.IndexByte(rel, ':'); i >= 0 {
		return unsafe("contains a volume separator")
	}
	clean := path.Clean(rel)
	if clean != rel {
		return unsafe("is not a clean path")
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return unsafe("escapes the asset root")
	}
	return clean, nil
}

// ReadBytes returns the raw content of an embedded asset file. It exists
// because copy flows need bytes, while the legacy Read helper returns a
// string for prompt composition.
func ReadBytes(name string) ([]byte, error) {
	if _, err := SafeRelative(name); err != nil {
		return nil, err
	}
	return fs.ReadFile(FS, name)
}
