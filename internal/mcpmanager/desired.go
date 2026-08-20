package mcpmanager

import (
	"fmt"
	"net/url"
	"strings"
)

// DesiredKind distinguishes the provenance of a desired MCP server: a named
// catalog preset, an explicit local command server, or an explicit remote
// endpoint. The kinds are mutually exclusive by validation: exactly one of
// Preset, Command, or URL may carry a value, and the auxiliary Env and
// Headers assignments are bound to exactly one kind each.
type DesiredKind string

const (
	// DesiredPreset manages a fixed catalog preset entry.
	DesiredPreset DesiredKind = "preset"

	// DesiredLocal manages a custom local stdio server launched from an
	// exact argv vector. No shell is ever involved: the vector is written
	// verbatim into the configuration.
	DesiredLocal DesiredKind = "local"

	// DesiredRemote manages a custom remote server reached over a
	// well-formed http or https URL.
	DesiredRemote DesiredKind = "remote"
)

// Desired is the typed, validated description of exactly one MCP server a
// caller wants cortex-ia to manage in OpenCode's "mcp" object. It is pure
// data: constructing, copying, and validating it performs no I/O, which lets
// a CLI express preset, local custom, and remote custom servers without any
// shell string parsing. Local servers keep the exact argv vector; nothing is
// ever joined into a shell command. Env and Headers carry KEY=VALUE
// assignments whose values reach the configuration file only: by the
// installmeta identity contract they are never representable in digests,
// ownership records, state, lock files, or sanitized listings.
type Desired struct {
	// Name is the OpenCode MCP server key. For the preset kind it must be
	// exactly the catalog preset name; catalog names are reserved for the
	// preset kind.
	Name string

	// Kind selects preset, local, or remote semantics.
	Kind DesiredKind

	// Preset names the catalog preset for the preset kind.
	Preset string

	// Command is the exact local argv vector for the local kind. Order and
	// content are semantic and preserved verbatim.
	Command []string

	// URL is the remote endpoint for the remote kind. It must parse as a
	// valid http or https URL with a host.
	URL string

	// Env holds KEY=VALUE environment assignments for the local kind.
	Env []string

	// Headers holds KEY=VALUE HTTP header assignments for the remote kind.
	Headers []string
}

// InvalidDesiredError reports a desired MCP description that violates the
// typed contract: unknown kind, missing or exclusive fields, malformed
// KEY=VALUE assignments, invalid URLs, or reserved catalog names used by a
// custom kind. Operations fail closed before any read or write of the
// configuration. The error names the offending field and reason, never a
// value.
type InvalidDesiredError struct {
	Name   string
	Field  string
	Reason string
}

func (e *InvalidDesiredError) Error() string {
	return fmt.Sprintf("mcpmanager: desired MCP %q field %q is invalid: %s", e.Name, e.Field, e.Reason)
}

func invalidDesired(name, field, reason string) error {
	return &InvalidDesiredError{Name: name, Field: field, Reason: reason}
}

// Validate enforces the Desired contract and returns a typed
// *InvalidDesiredError on the first violation. It is pure: no filesystem or
// catalog mutation happens, and no assignment value is ever copied into the
// returned error.
func (d Desired) Validate() error {
	if d.Name == "" {
		return invalidDesired(d.Name, "name", "must be non-empty")
	}
	if strings.TrimSpace(d.Name) != d.Name {
		return invalidDesired(d.Name, "name", "must not carry surrounding whitespace")
	}
	if !isPrintableToken(d.Name) {
		return invalidDesired(d.Name, "name", "must be a printable ASCII token without whitespace")
	}

	_, reserved := Lookup(d.Name)
	switch d.Kind {
	case DesiredPreset:
		if strings.TrimSpace(d.Preset) == "" {
			return invalidDesired(d.Name, "preset", "is required for the preset kind")
		}
		if _, ok := Lookup(d.Preset); !ok {
			return invalidDesired(d.Name, "preset", fmt.Sprintf("%q is not a managed catalog preset", d.Preset))
		}
		if d.Name != d.Preset {
			return invalidDesired(d.Name, "name", "a preset desired must use the catalog preset name; aliasing is not supported")
		}
		if len(d.Command) > 0 {
			return invalidDesired(d.Name, "command", "is exclusive to the local kind")
		}
		if strings.TrimSpace(d.URL) != "" {
			return invalidDesired(d.Name, "url", "is exclusive to the remote kind")
		}
		if len(d.Env) > 0 {
			return invalidDesired(d.Name, "env", "is exclusive to the local kind; catalog presets are fixed")
		}
		if len(d.Headers) > 0 {
			return invalidDesired(d.Name, "headers", "is exclusive to the remote kind; catalog presets are fixed")
		}

	case DesiredLocal:
		if reserved {
			return invalidDesired(d.Name, "name", "matches a managed preset name; manage it through the preset kind")
		}
		if len(d.Command) == 0 {
			return invalidDesired(d.Name, "command", "is required for the local kind")
		}
		for i, part := range d.Command {
			if part == "" {
				return invalidDesired(d.Name, "command", fmt.Sprintf("argv part %d is empty; the argv vector must be exact and non-empty", i))
			}
			if strings.ContainsRune(part, 0) {
				return invalidDesired(d.Name, "command", fmt.Sprintf("argv part %d contains NUL", i))
			}
		}
		if strings.TrimSpace(d.URL) != "" {
			return invalidDesired(d.Name, "url", "is exclusive to the remote kind")
		}
		if len(d.Headers) > 0 {
			return invalidDesired(d.Name, "headers", "are exclusive to the remote kind")
		}
		if _, err := parseAssignments(d.Name, "env", d.Env); err != nil {
			return err
		}

	case DesiredRemote:
		if reserved {
			return invalidDesired(d.Name, "name", "matches a managed preset name; manage it through the preset kind")
		}
		if strings.TrimSpace(d.URL) == "" {
			return invalidDesired(d.Name, "url", "is required for the remote kind")
		}
		if d.URL != strings.TrimSpace(d.URL) {
			return invalidDesired(d.Name, "url", "must not carry surrounding whitespace")
		}
		if !validRemoteURL(d.URL) {
			return invalidDesired(d.Name, "url", "must be a valid http or https URL with a host")
		}
		if len(d.Command) > 0 {
			return invalidDesired(d.Name, "command", "is exclusive to the local kind")
		}
		if len(d.Env) > 0 {
			return invalidDesired(d.Name, "env", "is exclusive to the local kind")
		}
		if _, err := parseAssignments(d.Name, "headers", d.Headers); err != nil {
			return err
		}

	default:
		return invalidDesired(d.Name, "kind", fmt.Sprintf("unknown desired kind %q", d.Kind))
	}
	return nil
}

// Entry returns the canonical OpenCode "mcp" entry for the desired server.
// The desired description is validated first, so an invalid desired never
// produces an entry. Local entries carry the exact argv vector and optional
// env object; remote entries carry the URL and optional headers object;
// preset entries are deep copies of the catalog template.
func (d Desired) Entry() (map[string]any, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	switch d.Kind {
	case DesiredPreset:
		preset, _ := Lookup(d.Preset)
		return deepCopyEntry(preset.Entry), nil
	case DesiredLocal:
		env, err := parseAssignments(d.Name, "env", d.Env)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{
			"type":    "local",
			"command": commandVector(d.Command),
			"enabled": true,
		}
		if len(env) > 0 {
			entry["env"] = env
		}
		return entry, nil
	case DesiredRemote:
		headers, err := parseAssignments(d.Name, "headers", d.Headers)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{
			"type":    "remote",
			"url":     d.URL,
			"enabled": true,
		}
		if len(headers) > 0 {
			entry["headers"] = headers
		}
		return entry, nil
	default:
		return nil, invalidDesired(d.Name, "kind", fmt.Sprintf("unknown desired kind %q", d.Kind))
	}
}

// parseAssignments validates KEY=VALUE assignments and returns them as a
// JSON-shaped object. Keys must be non-empty printable ASCII tokens without
// whitespace (valid as both environment variable names and HTTP header
// tokens), unique within the list, and free of NUL; values may be empty but
// must not carry NUL. Values are returned for the configuration entry only
// and never reach an error, a digest, or metadata.
func parseAssignments(name, field string, items []string) (map[string]any, error) {
	assignments := make(map[string]any, len(items))
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return nil, invalidDesired(name, field, "assignments must be KEY=VALUE")
		}
		if key == "" {
			return nil, invalidDesired(name, field, "assignment keys must be non-empty")
		}
		if strings.TrimSpace(key) != key || !isPrintableToken(key) {
			return nil, invalidDesired(name, field, "assignment keys must be printable ASCII without whitespace")
		}
		if _, duplicate := assignments[key]; duplicate {
			return nil, invalidDesired(name, field, fmt.Sprintf("key %q is assigned twice", key))
		}
		if strings.ContainsRune(value, 0) {
			return nil, invalidDesired(name, field, "assignment values must not contain NUL")
		}
		assignments[key] = value
	}
	return assignments, nil
}

// validRemoteURL reports whether raw is a well-formed http or https URL with
// a non-empty host. Credentials embedded in the URL are allowed (OpenCode
// accepts them) but are never hashed or echoed by the manager.
func validRemoteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

// commandVector converts an exact argv slice into the JSON vector shape
// without joining, trimming, or otherwise normalizing any part.
func commandVector(command []string) []any {
	vector := make([]any, len(command))
	for i, part := range command {
		vector[i] = part
	}
	return vector
}

// isPrintableToken reports whether s is a non-empty run of printable ASCII
// characters excluding space: the conservative shared charset for server
// names, environment variable names, and HTTP header tokens.
func isPrintableToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x21 || r > 0x7E {
			return false
		}
	}
	return true
}
