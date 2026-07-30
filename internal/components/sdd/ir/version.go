// Package ir defines the canonical, runtime-neutral SDD workflow contracts.
package ir

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Version is a semantic version used by workflow and schema contracts.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a strict major.minor.patch semantic version.
func ParseVersion(value string) (Version, error) {
	parts := semanticVersionPattern.FindStringSubmatch(value)
	if parts == nil {
		return Version{}, fmt.Errorf("invalid semantic version %q: expected major.minor.patch", value)
	}
	major, _ := strconv.Atoi(parts[1])
	minor, _ := strconv.Atoi(parts[2])
	patch, _ := strconv.Atoi(parts[3])
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// MustParseVersion parses value and panics when it is invalid. It is intended
// for canonical contracts declared in Go source.
func MustParseVersion(value string) Version {
	version, err := ParseVersion(value)
	if err != nil {
		panic(err)
	}
	return version
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// MarshalJSON represents versions as semantic-version strings.
func (v Version) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// UnmarshalJSON accepts semantic-version strings only.
func (v *Version) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("semantic version must be a string: %w", err)
	}
	parsed, err := ParseVersion(value)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

// VersionRange declares the compatibility interval and qualification edge.
// Versions sharing Minimum's major remain input-compatible; MaximumTested
// records the newest version backed by conformance evidence.
type VersionRange struct {
	Minimum       Version `json:"minimum"`
	MaximumTested Version `json:"maximum_tested"`
}

func (r VersionRange) String() string {
	return fmt.Sprintf(">=%s, <%d.0.0 (tested through %s)", r.Minimum, r.Minimum.Major+1, r.MaximumTested)
}

// SchemaContract identifies a versioned schema and its supported interval.
type SchemaContract struct {
	ID        SemanticID
	Current   Version
	Supported VersionRange
}

var (
	// WorkflowSchema is the compatibility contract for WorkflowIR documents.
	WorkflowSchema = SchemaContract{
		ID:      "schema/workflow-ir",
		Current: MustParseVersion("1.0.0"),
		Supported: VersionRange{
			Minimum:       MustParseVersion("1.0.0"),
			MaximumTested: MustParseVersion("1.2.3"),
		},
	}
	// ContractSchema is the compatibility contract for role input/output contracts.
	ContractSchema = SchemaContract{
		ID:      "schema/role-contract",
		Current: MustParseVersion("1.0.0"),
		Supported: VersionRange{
			Minimum:       MustParseVersion("1.0.0"),
			MaximumTested: MustParseVersion("1.0.0"),
		},
	}
	// ExtensionSchema is the compatibility contract for provider-neutral extension declarations.
	ExtensionSchema = SchemaContract{
		ID:      "schema/extension-contract",
		Current: MustParseVersion("1.0.0"),
		Supported: VersionRange{
			Minimum:       MustParseVersion("1.0.0"),
			MaximumTested: MustParseVersion("1.0.0"),
		},
	}
)
