package updater

import (
	"errors"
	"fmt"
	"math"
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
