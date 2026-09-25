package updater

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrInvalidVersionFormat = errors.New("invalid canonical version: must be vMAJOR.MINOR.PATCH")
	ErrVersionOverflow      = errors.New("version component overflow")
	ErrDevUnknownVersion    = errors.New("cannot apply update to dev or unknown version")
	ErrDowngradeOrReplay    = errors.New("update version must strictly exceed current installed version and applied floor")
)

// CanonicalVersion represents a validated stable semantic version vMAJOR.MINOR.PATCH.
type CanonicalVersion struct {
	Major uint64
	Minor uint64
	Patch uint64
	Raw   string
}

// ParseCanonicalVersion parses a strict canonical stable version string.
func ParseCanonicalVersion(s string) (CanonicalVersion, error) {
	if !strings.HasPrefix(s, "v") {
		return CanonicalVersion{}, fmt.Errorf("%w: missing 'v' prefix in %q", ErrInvalidVersionFormat, s)
	}
	raw := s
	trimmed := s[1:]
	if trimmed == "" {
		return CanonicalVersion{}, fmt.Errorf("%w: empty version in %q", ErrInvalidVersionFormat, s)
	}

	if strings.ContainsAny(trimmed, "+-") {
		return CanonicalVersion{}, fmt.Errorf("%w: suffixes not permitted in %q", ErrInvalidVersionFormat, s)
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return CanonicalVersion{}, fmt.Errorf("%w: expected 3 components in %q", ErrInvalidVersionFormat, s)
	}

	nums := make([]uint64, 3)
	for i, p := range parts {
		if p == "" {
			return CanonicalVersion{}, fmt.Errorf("%w: empty component in %q", ErrInvalidVersionFormat, s)
		}
		if len(p) > 1 && p[0] == '0' {
			return CanonicalVersion{}, fmt.Errorf("%w: leading zeros not permitted in %q", ErrInvalidVersionFormat, s)
		}
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				return CanonicalVersion{}, fmt.Errorf("%w: non-digit character in %q", ErrInvalidVersionFormat, s)
			}
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return CanonicalVersion{}, fmt.Errorf("%w: %q", ErrVersionOverflow, p)
			}
			return CanonicalVersion{}, fmt.Errorf("%w: %s", ErrInvalidVersionFormat, p)
		}
		if n > math.MaxInt64 {
			return CanonicalVersion{}, fmt.Errorf("%w: %q", ErrVersionOverflow, p)
		}
		nums[i] = n
	}

	return CanonicalVersion{
		Major: nums[0],
		Minor: nums[1],
		Patch: nums[2],
		Raw:   raw,
	}, nil
}

// CompareCanonical compares two canonical versions: -1 if a < b, 0 if a == b, 1 if a > b.
func CompareCanonical(a, b CanonicalVersion) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// IsDevOrUnknown reports whether the version is an un-updatable development or unknown identifier.
func IsDevOrUnknown(v string) bool {
	norm := strings.ToLower(strings.TrimSpace(v))
	return norm == "" || norm == "dev" || norm == "unknown" || norm == "development"
}

// BuildKind classifies a running version for self-update eligibility.
type BuildKind int

const (
	// ReleaseBuild is a canonical stable release that may participate in a self-update.
	ReleaseBuild BuildKind = iota
	// DevelopmentBuild is a dev, unknown, or empty version carrying no release identity.
	DevelopmentBuild
	// NonCanonicalBuild is any other non-canonical identifier, such as a git-describe string.
	NonCanonicalBuild
)

// ClassifyBuild reports whether v is a canonical release, a development build,
// or a non-canonical identifier. Only ReleaseBuild may be compared against
// release tags; the other kinds must be told to install a release instead of
// having a raw version-parser failure surfaced.
func ClassifyBuild(v string) BuildKind {
	if IsDevOrUnknown(v) {
		return DevelopmentBuild
	}
	if _, err := ParseCanonicalVersion(v); err != nil {
		return NonCanonicalBuild
	}
	return ReleaseBuild
}

// SelfUpdateDisabledNotice is the operator-facing message for a build that
// cannot self-update because it carries no stable release identity.
func SelfUpdateDisabledNotice(v string) string {
	return fmt.Sprintf(
		"cortex-ia is running a development build (%s); self-update is disabled.\nInstall a release from https://github.com/%s/releases to enable updates.",
		v, DefaultRepo,
	)
}

// VerifyVersionFloor checks that candidate exceeds current and any applied floor.
func VerifyVersionFloor(currentVer, candidateVer, appliedFloor string) error {
	if IsDevOrUnknown(currentVer) {
		return fmt.Errorf("%w: current version is %q", ErrDevUnknownVersion, currentVer)
	}
	cur, err := ParseCanonicalVersion(currentVer)
	if err != nil {
		return fmt.Errorf("invalid current version: %w", err)
	}

	cand, err := ParseCanonicalVersion(candidateVer)
	if err != nil {
		return fmt.Errorf("invalid candidate version: %w", err)
	}

	if CompareCanonical(cand, cur) <= 0 {
		return fmt.Errorf("%w: candidate %s <= current %s", ErrDowngradeOrReplay, candidateVer, currentVer)
	}

	if appliedFloor != "" && !IsDevOrUnknown(appliedFloor) {
		floor, err := ParseCanonicalVersion(appliedFloor)
		if err == nil {
			if CompareCanonical(cand, floor) <= 0 {
				return fmt.Errorf("%w: candidate %s <= applied floor %s", ErrDowngradeOrReplay, candidateVer, appliedFloor)
			}
		}
	}

	return nil
}

// CheckUpdateCandidate evaluates whether candidate is newer than current.
func CheckUpdateCandidate(currentVer, candidateVer string) (bool, error) {
	if IsDevOrUnknown(currentVer) {
		return false, nil
	}
	cur, err := ParseCanonicalVersion(currentVer)
	if err != nil {
		return false, err
	}
	cand, err := ParseCanonicalVersion(candidateVer)
	if err != nil {
		return false, err
	}
	return CompareCanonical(cand, cur) > 0, nil
}

// projectModulePath is the Go module this binary is built from. Git metadata is
// trusted only when the checked-out repository declares the same module, so an
// unrelated repository's tags can never be reported as the running version.
const projectModulePath = "github.com/" + DefaultRepo

// GitDescribeVersion returns the git-describe identifier of the checked-out
// cortex-ia source tree, or "" when the working directory is not this project's
// repository.
func GitDescribeVersion() string {
	if !inProjectRepository() {
		return ""
	}
	out, err := exec.Command("git", "describe", "--tags", "--always").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// inProjectRepository reports whether the working directory belongs to a git
// repository whose go.mod declares this project's module path. A missing git
// binary, a non-repository directory, and a repository with a different module
// all report false.
func inProjectRepository() bool {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	return declaredModule(data) == projectModulePath
}

// declaredModule extracts the module path declared by a go.mod file.
func declaredModule(goMod []byte) string {
	for _, line := range strings.Split(string(goMod), "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}
