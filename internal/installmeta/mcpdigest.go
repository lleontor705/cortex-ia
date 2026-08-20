// Package installmeta defines the shared, versioned, secret-free semantic
// identity digests used by installer state/lock metadata (v2) and the OpenCode
// MCP manager. It is a standard-library-only leaf package: it must never
// import other repository packages, so internal/state and internal/mcpmanager
// can share exactly one canonical digest encoding without import cycles.
//
// Secret-free by construction: the canonical MCP identity carries the server
// name, entry type, local command vector, and environment/header variable
// NAMES only. Environment and header VALUES, URLs (which may embed
// credentials), and runtime flags such as "enabled" are never hashed and never
// appear in digest strings.
package installmeta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MCPDigestVersion is the current version of the MCP semantic digest. The
// version is embedded both in the hashed payload and in the digest string
// prefix, so a digest produced by a different version can always be detected
// and rejected instead of silently comparing unequal encodings.
const MCPDigestVersion = 1

// mcpDigestPrefix prefixes every MCP digest string as "mcpv<version>:".
const mcpDigestPrefix = "mcpv"

// mcpDigestDomain separates this digest from every other sha256 use in the
// repository; it is hashed before the canonical payload.
const mcpDigestDomain = "cortex-ia/installmeta/mcp-server-digest\n"

// MCPServerIdentity is the canonical, secret-free identity of one MCP server
// entry. Callers must populate EnvNames and HeaderNames with variable NAMES
// only; values are not representable in this struct and can therefore never
// reach a digest, a state file, or a lock file.
type MCPServerIdentity struct {
	// Name is the MCP server key inside OpenCode's "mcp" object. It is part
	// of the identity: the same entry body under two names is two servers.
	Name string
	// Type is the entry "type" value (e.g. "local", "remote"); empty when
	// the entry does not declare one.
	Type string
	// Command is the local command program: the first element of the entry
	// "command" vector. Empty when the entry declares no command.
	Command string
	// Args are the local command arguments after the program, in declared
	// order. Order is semantic and is never normalized.
	Args []string
	// EnvNames are the entry "env" variable names, any order; the digest
	// normalizes them to a sorted, deduplicated set.
	EnvNames []string
	// HeaderNames are the entry "headers" names, any order; the digest
	// normalizes them to a sorted, deduplicated set.
	HeaderNames []string
}

// canonicalMCPServer is the exact wire form hashed into the digest. Struct
// field order fixes member order, so encoding is deterministic without
// relying on map iteration or caller-supplied ordering.
type canonicalMCPServer struct {
	Version     int      `json:"version"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	EnvNames    []string `json:"env_names"`
	HeaderNames []string `json:"header_names"`
}

// MCPIdentityDigest returns the versioned semantic digest of an explicit MCP
// identity. The digest string is "mcpv<version>:<64 lowercase hex>" where the
// hex is sha256 over the domain separator followed by the canonical JSON
// encoding of the identity. Nil and empty Args/EnvNames/HeaderNames produce
// the same digest; EnvNames and HeaderNames are treated as sets (sorted,
// deduplicated) while Args keeps its order.
func MCPIdentityDigest(identity MCPServerIdentity) (string, error) {
	canonical := canonicalMCPServer{
		Version:     MCPDigestVersion,
		Name:        identity.Name,
		Type:        identity.Type,
		Command:     identity.Command,
		Args:        orderedCopy(identity.Args),
		EnvNames:    sortedUniqueCopy(identity.EnvNames),
		HeaderNames: sortedUniqueCopy(identity.HeaderNames),
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("installmeta: encode MCP identity: %w", err)
	}
	hashed := append([]byte(mcpDigestDomain), payload...)
	sum := sha256.Sum256(hashed)
	return mcpDigestPrefix + strconv.Itoa(MCPDigestVersion) + ":" + hex.EncodeToString(sum[:]), nil
}

// MCPServerIdentityFromEntry extracts the canonical secret-free identity from
// one OpenCode "mcp" entry value keyed by name. Only identity inputs are
// read: "type", "command" (string vector), and the key sets of "env" and
// "headers". Env and header VALUES, URLs, "enabled", and any other members
// are ignored by definition, so secrets can never influence the digest.
//
// Shapes that are present but not canonical fail closed with an error naming
// the offending field (never its value): a non-string "type", a "command"
// that is not a vector of strings, or an "env"/"headers" that is not an
// object. Absent members are defined as empty identity inputs.
func MCPServerIdentityFromEntry(name string, entry map[string]any) (MCPServerIdentity, error) {
	if name == "" {
		return MCPServerIdentity{}, fmt.Errorf("installmeta: MCP entry name is empty")
	}
	if entry == nil {
		return MCPServerIdentity{Name: name}, nil
	}

	identity := MCPServerIdentity{Name: name}

	if value, present := entry["type"]; present && value != nil {
		text, ok := value.(string)
		if !ok {
			return MCPServerIdentity{}, fmt.Errorf("installmeta: MCP entry %q field %q must be a string", name, "type")
		}
		identity.Type = text
	}

	if value, present := entry["command"]; present && value != nil {
		raw, ok := value.([]any)
		if !ok {
			return MCPServerIdentity{}, fmt.Errorf("installmeta: MCP entry %q field %q must be a vector of strings", name, "command")
		}
		parts := make([]string, 0, len(raw))
		for _, item := range raw {
			part, ok := item.(string)
			if !ok {
				return MCPServerIdentity{}, fmt.Errorf("installmeta: MCP entry %q field %q must be a vector of strings", name, "command")
			}
			parts = append(parts, part)
		}
		if len(parts) > 0 {
			identity.Command = parts[0]
			identity.Args = parts[1:]
		}
	}

	envNames, err := objectKeySet(name, entry, "env")
	if err != nil {
		return MCPServerIdentity{}, err
	}
	identity.EnvNames = envNames

	headerNames, err := objectKeySet(name, entry, "headers")
	if err != nil {
		return MCPServerIdentity{}, err
	}
	identity.HeaderNames = headerNames

	return identity, nil
}

// MCPServerDigest extracts the identity of the named entry and returns its
// versioned semantic digest in one step.
func MCPServerDigest(name string, entry map[string]any) (string, error) {
	identity, err := MCPServerIdentityFromEntry(name, entry)
	if err != nil {
		return "", err
	}
	return MCPIdentityDigest(identity)
}

// ParseMCPServerDigest splits a digest string into its declared version and
// sha256 sum. Malformed digests and non-numeric versions are errors. A
// syntactically valid version this binary does not know parses successfully;
// compare the returned version against MCPDigestVersion (or use
// ValidMCPServerDigest) so unknown versions fail closed at the consumer.
func ParseMCPServerDigest(digest string) (version int, sum string, err error) {
	prefix, sum, ok := strings.Cut(digest, ":")
	if !ok || !strings.HasPrefix(prefix, mcpDigestPrefix) {
		return 0, "", fmt.Errorf("installmeta: malformed MCP digest: missing %q version prefix", mcpDigestPrefix+"<version>:")
	}
	version, err = strconv.Atoi(strings.TrimPrefix(prefix, mcpDigestPrefix))
	if err != nil || version < 1 {
		return 0, "", fmt.Errorf("installmeta: malformed MCP digest version in %q", prefix)
	}
	if !isHex64(sum) {
		return 0, "", fmt.Errorf("installmeta: malformed MCP digest sum")
	}
	return version, sum, nil
}

// MCPPostImageDigestVersion is the version of the full-postimage ownership
// fingerprint ("mcpv2:"). It is deliberately a PARALLEL version line to the
// identity digest (MCPDigestVersion stays 1): legacy mcpv1 identity records
// remain readable as incompatible ownership evidence instead of breaking
// state validation, while destructive accreditation demands the mcpv2
// fingerprint. Compatibility contract: an ownership record without an mcpv2
// fingerprint can never accredit a destructive custom removal; the user
// re-runs the add command (which verifies full-value equality first) to
// accredit the postimage, then removes.
const MCPPostImageDigestVersion = 2

// mcpPostImageDomain separates the keyed postimage fingerprint from the
// identity digest and from every other hash use in the repository; it is the
// HMAC message prefix.
const mcpPostImageDomain = "cortex-ia/installmeta/mcp-postimage-digest\n"

// mcpPostImageValueDomain prefixes every keyed per-value hash (URL, env and
// header values) so no raw value, empty or not, is ever hashed under the
// same encoding as another field class.
const mcpPostImageValueDomain = "cortex-ia/installmeta/mcp-postimage-value\n"

// MCPServerPostImage is the canonical full postimage of one MCP entry: the
// secret-free identity plus every mutable field ownership must bind for
// destructive operations — the remote URL, the enabled flag, the env and
// header VALUES, and the config file path. Values never leave this struct in
// clear text: MCPPostImageDigest replaces them with keyed HMAC-SHA256 hashes
// over a locally generated random salt, so the fingerprint is not reversible
// and cannot be correlated across homes (each home owns an independent
// salt). A nil Enabled means the entry does not declare the flag; HasURL
// distinguishes an absent URL from an explicitly empty one.
type MCPServerPostImage struct {
	Identity     MCPServerIdentity
	ConfigPath   string
	URL          string
	HasURL       bool
	Enabled      *bool
	EnvValues    map[string]string
	HeaderValues map[string]string
}

// canonicalMCPServerPostImage is the exact wire form MACed into the mcpv2
// fingerprint. Struct field order fixes member order; map keys are sorted by
// encoding/json, so the encoding is deterministic. Every secret-bearing
// member is already a keyed hash; only nonsecret identity and the config
// path appear in clear text.
type canonicalMCPServerPostImage struct {
	Version      int               `json:"version"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	EnvNames     []string          `json:"env_names"`
	HeaderNames  []string          `json:"header_names"`
	ConfigPath   string            `json:"config_path"`
	Enabled      string            `json:"enabled"`
	URLHash      string            `json:"url_hash"`
	EnvHashes    map[string]string `json:"env_hashes"`
	HeaderHashes map[string]string `json:"header_hashes"`
}

// MCPPostImageDigest returns the versioned keyed fingerprint of one full MCP
// postimage: "mcpv2:<64 lowercase hex>" where the hex is HMAC-SHA256(salt,
// postimage domain || canonical JSON). The canonical payload embeds the
// complete identity (name, type, command/args, env/header names), the config
// path, the enabled state, and keyed hashes of the URL and of every env and
// header VALUE. No clear secret value is representable in the output. An
// empty salt is an error: without the local salt the fingerprint cannot be
// computed or verified, and callers must fail closed.
func MCPPostImageDigest(postImage MCPServerPostImage, salt []byte) (string, error) {
	if len(salt) == 0 {
		return "", fmt.Errorf("installmeta: MCP postimage fingerprint requires a non-empty local salt")
	}
	canonical := canonicalMCPServerPostImage{
		Version:      MCPPostImageDigestVersion,
		Name:         postImage.Identity.Name,
		Type:         postImage.Identity.Type,
		Command:      postImage.Identity.Command,
		Args:         orderedCopy(postImage.Identity.Args),
		EnvNames:     sortedUniqueCopy(postImage.Identity.EnvNames),
		HeaderNames:  sortedUniqueCopy(postImage.Identity.HeaderNames),
		ConfigPath:   postImage.ConfigPath,
		Enabled:      enabledState(postImage.Enabled),
		EnvHashes:    keyedValueSet(salt, postImage.EnvValues),
		HeaderHashes: keyedValueSet(salt, postImage.HeaderValues),
	}
	if postImage.HasURL {
		canonical.URLHash = keyedValue(salt, postImage.URL)
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("installmeta: encode MCP postimage: %w", err)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(mcpPostImageDomain))
	mac.Write(payload)
	return mcpDigestPrefix + strconv.Itoa(MCPPostImageDigestVersion) + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}

// ValidMCPServerPostImageDigest reports whether digest is exactly the mcpv2
// encoding produced by MCPPostImageDigest. Legacy mcpv1 strings, unknown
// versions, and malformed values are invalid, so every consumer fails closed.
func ValidMCPServerPostImageDigest(digest string) bool {
	version, _, err := ParseMCPServerDigest(digest)
	return err == nil && version == MCPPostImageDigestVersion
}

// MCPServerPostImageFromEntry extracts the full postimage of the named entry.
// It reads the identity inputs plus "url" (string), "enabled" (bool), and the
// env/header NAME=VALUE assignments. Shapes that are present but not
// canonical fail closed with an error naming the offending field, never its
// value: a non-string URL or env/header value cannot be fingerprinted, and an
// unfingerprintable entry can never be destructively accredited. Absent
// members are defined as empty postimage inputs.
func MCPServerPostImageFromEntry(name string, entry map[string]any) (MCPServerPostImage, error) {
	identity, err := MCPServerIdentityFromEntry(name, entry)
	if err != nil {
		return MCPServerPostImage{}, err
	}
	postImage := MCPServerPostImage{Identity: identity}
	if value, present := entry["url"]; present && value != nil {
		text, ok := value.(string)
		if !ok {
			return MCPServerPostImage{}, fmt.Errorf("installmeta: MCP entry %q field %q must be a string", name, "url")
		}
		postImage.URL, postImage.HasURL = text, true
	}
	if value, present := entry["enabled"]; present && value != nil {
		flag, ok := value.(bool)
		if !ok {
			return MCPServerPostImage{}, fmt.Errorf("installmeta: MCP entry %q field %q must be a boolean", name, "enabled")
		}
		postImage.Enabled = &flag
	}
	envValues, err := objectStringValues(name, entry, "env")
	if err != nil {
		return MCPServerPostImage{}, err
	}
	postImage.EnvValues = envValues
	headerValues, err := objectStringValues(name, entry, "headers")
	if err != nil {
		return MCPServerPostImage{}, err
	}
	postImage.HeaderValues = headerValues
	return postImage, nil
}

// enabledState maps the tri-state enabled flag onto its canonical encoding.
func enabledState(enabled *bool) string {
	switch {
	case enabled == nil:
		return "unset"
	case *enabled:
		return "true"
	default:
		return "false"
	}
}

// keyedValue returns hex(HMAC-SHA256(salt, value domain || value)). The key
// binds the digest to this home's local salt; the domain separates value
// hashes from every other MAC use.
func keyedValue(salt []byte, value string) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(mcpPostImageValueDomain))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// keyedValueSet replaces every NAME=VALUE assignment with NAME=keyedValue,
// preserving the name binding structurally (JSON object) so no clear value
// survives. An empty or nil set canonicalizes to an empty object.
func keyedValueSet(salt []byte, values map[string]string) map[string]string {
	hashes := make(map[string]string, len(values))
	for name, value := range values {
		hashes[name] = keyedValue(salt, value)
	}
	return hashes
}

// objectStringSet is the value-reading counterpart of objectKeySet: it
// requires every value of entry[field] to be a string and returns the
// NAME=VALUE map. Non-string values fail closed naming the field only.
func objectStringValues(name string, entry map[string]any, field string) (map[string]string, error) {
	value, present := entry[field]
	if !present || value == nil {
		return map[string]string{}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("installmeta: MCP entry %q field %q must be an object of names", name, field)
	}
	values := make(map[string]string, len(object))
	for key, raw := range object {
		text, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("installmeta: MCP entry %q field %q values must be strings", name, field)
		}
		values[key] = text
	}
	return values, nil
}

// ValidMCPServerDigest reports whether digest is exactly the encoding
// produced by the current MCPDigestVersion. Unknown versions, legacy raw-hex
// strings, and malformed values are invalid, which lets every consumer fail
// closed on mismatch instead of degrading.
func ValidMCPServerDigest(digest string) bool {
	version, _, err := ParseMCPServerDigest(digest)
	return err == nil && version == MCPDigestVersion
}

// objectKeySet returns the sorted key names of entry[field] when present.
// Absent or null is an empty set; any other shape is a fail-closed error.
// Values are never read.
func objectKeySet(name string, entry map[string]any, field string) ([]string, error) {
	value, present := entry[field]
	if !present || value == nil {
		return nil, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("installmeta: MCP entry %q field %q must be an object of names", name, field)
	}
	names := make([]string, 0, len(object))
	for key := range object {
		names = append(names, key)
	}
	sort.Strings(names)
	return names, nil
}

// orderedCopy clones a list preserving order and duplicates, normalizing nil
// to the empty slice so nil and empty encode identically.
func orderedCopy(list []string) []string {
	if len(list) == 0 {
		return []string{}
	}
	return append([]string(nil), list...)
}

// sortedUniqueCopy clones a name set sorted and deduplicated, normalizing nil
// to the empty slice. Name sets are unordered by definition, so ordering and
// duplicates are non-semantic variants.
func sortedUniqueCopy(names []string) []string {
	if len(names) == 0 {
		return []string{}
	}
	copied := append([]string(nil), names...)
	sort.Strings(copied)
	unique := copied[:1]
	for _, name := range copied[1:] {
		if name != unique[len(unique)-1] {
			unique = append(unique, name)
		}
	}
	return unique
}

// isHex64 reports whether s is exactly 64 lowercase hexadecimal characters.
func isHex64(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
