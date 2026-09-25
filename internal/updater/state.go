package updater

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const (
	// UpdateStateSchemaVersion is the only accepted update-state schema.
	UpdateStateSchemaVersion = 1
	updateStateFileName      = "update-state.json"

	// UpdateCheckTTL is how long a completed automatic or scheduled check stays
	// authoritative. Explicit `cortex-ia update --check` bypasses it.
	UpdateCheckTTL = 24 * time.Hour
)

// ErrCorruptUpdateState marks persisted state that cannot be trusted. Callers
// receive empty state alongside the error so a damaged file degrades the
// update pipeline instead of blocking it.
var ErrCorruptUpdateState = errors.New("update state is empty or corrupt")

// UpdateState is the machine-local memory of the update pipeline. Available
// holds the cached release tag pending confirmation; AvailableDigest holds the
// manifest digest observed for it.
type UpdateState struct {
	SchemaVersion     int       `json:"schema_version"`
	LastCheckedAt     time.Time `json:"last_checked_at"`
	Available         string    `json:"available,omitempty"`
	AvailableDigest   string    `json:"available_digest,omitempty"`
	ReleaseETag       string    `json:"release_etag,omitempty"`
	AppliedFloor      string    `json:"applied_floor,omitempty"`
	ManagedPath       string    `json:"managed_path,omitempty"`
	InstallCandidates []string  `json:"install_candidates,omitempty"`
}

// UpdateAvailable reports whether a cached release is pending confirmation.
func (s UpdateState) UpdateAvailable() bool {
	return strings.TrimSpace(s.Available) != ""
}

// CheckedWithin reports whether the last completed check falls inside ttl. A
// zero timestamp means the machine has never checked, which is never fresh.
func (s UpdateState) CheckedWithin(now time.Time, ttl time.Duration) bool {
	return !s.LastCheckedAt.IsZero() && now.Sub(s.LastCheckedAt) < ttl
}

// DefaultStateHome resolves the Cortex-IA state root: $CORTEX_IA_HOME when
// set, otherwise ~/.cortex-ia.
func DefaultStateHome() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME")); override != "" {
		absolute, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve CORTEX_IA_HOME: %w", err)
		}
		return filepath.Clean(absolute), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".cortex-ia"), nil
}

// StatePath returns the update-state file path under home. An empty home
// resolves through DefaultStateHome with a relative last resort.
func StatePath(home string) string {
	root := strings.TrimSpace(home)
	if root == "" {
		if resolved, err := DefaultStateHome(); err == nil {
			root = resolved
		}
	}
	if root == "" {
		return updateStateFileName
	}
	return filepath.Join(root, updateStateFileName)
}

// LoadUpdateState reads persisted state. A missing file is a normal first run
// and returns empty state without error; an unreadable, empty, corrupt, or
// future-schema file returns empty state together with ErrCorruptUpdateState.
func LoadUpdateState(home string) (UpdateState, error) {
	empty := UpdateState{SchemaVersion: UpdateStateSchemaVersion}
	path := StatePath(home)

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty, nil
		}
		return empty, fmt.Errorf("%w: read %s: %v", ErrCorruptUpdateState, path, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return empty, fmt.Errorf("%w: %s is empty", ErrCorruptUpdateState, path)
	}

	var state UpdateState
	if err := json.Unmarshal(raw, &state); err != nil {
		return empty, fmt.Errorf("%w: decode %s: %v", ErrCorruptUpdateState, path, err)
	}
	if state.SchemaVersion != UpdateStateSchemaVersion {
		return empty, fmt.Errorf("%w: unsupported schema_version %d in %s", ErrCorruptUpdateState, state.SchemaVersion, path)
	}
	return state, nil
}

// SaveUpdateStateAtomic persists state through filemerge.WriteFileAtomic so a
// reader never observes a partially written file.
func SaveUpdateStateAtomic(home string, state UpdateState) error {
	path := StatePath(home)
	state.SchemaVersion = UpdateStateSchemaVersion
	state.InstallCandidates = dedupePaths(state.InstallCandidates)

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update state: %w", err)
	}
	raw = append(raw, '\n')

	if _, err := filemerge.WriteFileAtomic(path, raw, 0o600); err != nil {
		return fmt.Errorf("persist update state %s: %w", path, err)
	}
	return nil
}

// RecordAppliedFloor raises applied_floor after a verified apply, records the
// executable path that was replaced, and clears the cached release. The floor
// only moves forward and rejects non-canonical versions so a dev build can
// never poison it.
func RecordAppliedFloor(home, version, managedPath string) error {
	applied, err := ParseCanonicalVersion(version)
	if err != nil {
		return fmt.Errorf("record applied floor: %w", err)
	}

	state, _ := LoadUpdateState(home)
	state.Available = ""
	state.AvailableDigest = ""
	if managedPath != "" {
		state.ManagedPath = managedPath
	}

	if current, cerr := ParseCanonicalVersion(state.AppliedFloor); cerr == nil && CompareCanonical(applied, current) <= 0 {
		return SaveUpdateStateAtomic(home, state)
	}
	state.AppliedFloor = applied.Raw
	return SaveUpdateStateAtomic(home, state)
}

// DetectInstallCandidates returns the running executable plus every known
// install location that currently holds a binary. The running executable is
// always reported; other locations are best-effort so probing never fails an
// update check.
func DetectInstallCandidates(execPath string) []string {
	candidates := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		cleaned := filepath.Clean(trimmed)
		key := canonicalPathKey(cleaned)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	add(execPath)
	for _, known := range knownInstallPaths() {
		if info, err := os.Stat(known); err == nil && !info.IsDir() {
			add(known)
		}
	}
	return candidates
}

// DualInstallWarning describes ambiguous installations, or returns an empty
// string when a single install is present.
func DualInstallWarning(candidates []string) string {
	paths := dedupePaths(candidates)
	if len(paths) <= 1 {
		return ""
	}
	return fmt.Sprintf("multiple cortex-ia installations detected: %s", strings.Join(paths, ", "))
}

// systemInstallDirs holds the absolute directories the installer and package
// managers deploy into. It stays a variable so tests can sandbox probing away
// from the developer's real system paths.
var systemInstallDirs = defaultSystemInstallDirs()

func defaultSystemInstallDirs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{"/usr/local/bin", "/opt/homebrew/bin", "/usr/bin"}
}

func knownInstallPaths() []string {
	binary := "cortex-ia"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	paths := make([]string, 0, len(systemInstallDirs)+3)
	for _, dir := range systemInstallDirs {
		paths = append(paths, filepath.Join(dir, binary))
	}
	if runtime.GOOS != "windows" {
		if userHome, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(userHome, ".local", "bin", binary))
		}
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		paths = append(paths, filepath.Join(local, "Programs", "cortex-ia", "bin", binary))
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		if entries := filepath.SplitList(gopath); len(entries) > 0 && strings.TrimSpace(entries[0]) != "" {
			paths = append(paths, filepath.Join(strings.TrimSpace(entries[0]), "bin", binary))
		}
	} else if userHome, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(userHome, "go", "bin", binary))
	}
	return paths
}

func dedupePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		key := canonicalPathKey(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func canonicalPathKey(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = filepath.Clean(path)
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		absolute = strings.ToLower(absolute)
	}
	return absolute
}
