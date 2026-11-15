package installmeta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AgentModelDigestVersion is the current version of the agent-model semantic
// digest. The version is embedded both in the hashed payload and in the digest
// string prefix, so a digest produced by a different version is always
// detectable and rejected instead of silently compared against a different
// encoding.
const AgentModelDigestVersion = 1

// agentModelDigestPrefix prefixes every agent-model digest as "amdv<version>:".
const agentModelDigestPrefix = "amdv"

// agentModelDigestDomain separates this digest from every other sha256 use in
// the repository; it is hashed before the canonical payload.
const agentModelDigestDomain = "cortex-ia/installmeta/agent-model-digest\n"

// AgentModelIdentity is the canonical identity of one managed agent-model
// entry. Model references are non-secret, so the identity carries them in
// clear text; unlike the MCP postimage there are no URLs or environment values
// to protect, and a plain versioned sha256 is sufficient.
type AgentModelIdentity struct {
	// Agent is the OpenCode agent name the model reference targets.
	Agent string
	// Provider is the provider token: the segment before "/".
	Provider string
	// Model is the model token: the segment after "/" and before any "#".
	Model string
	// Variant is the optional effort suffix after "#"; empty when the
	// reference declares none.
	Variant string
}

// canonicalAgentModel is the exact wire form hashed into the digest. Struct
// field order fixes member order, so encoding is deterministic without
// relying on map iteration or caller-supplied ordering.
type canonicalAgentModel struct {
	Version  int    `json:"version"`
	Agent    string `json:"agent"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Variant  string `json:"variant"`
}

// AgentModelIdentityDigest returns the versioned semantic digest of an agent
// model identity. The digest string is "amdv<version>:<64 lowercase hex>"
// where the hex is sha256 over the domain separator followed by the canonical
// JSON encoding of the identity. The encoding is deterministic: the same
// identity always yields the same digest and any field difference yields a
// different one.
func AgentModelIdentityDigest(identity AgentModelIdentity) (string, error) {
	canonical := canonicalAgentModel{
		Version:  AgentModelDigestVersion,
		Agent:    identity.Agent,
		Provider: identity.Provider,
		Model:    identity.Model,
		Variant:  identity.Variant,
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("installmeta: encode agent model identity: %w", err)
	}
	hashed := append([]byte(agentModelDigestDomain), payload...)
	sum := sha256.Sum256(hashed)
	return agentModelDigestPrefix + strconv.Itoa(AgentModelDigestVersion) + ":" +
		hex.EncodeToString(sum[:]), nil
}

// ParseAgentModelDigest splits a digest string into its declared version and
// sha256 sum. Malformed digests and non-numeric versions are errors. A
// syntactically valid version this binary does not know parses successfully;
// compare the returned version against AgentModelDigestVersion (or use
// ValidAgentModelDigest) so unknown versions fail closed at the consumer.
func ParseAgentModelDigest(digest string) (version int, sum string, err error) {
	prefix, sum, ok := strings.Cut(digest, ":")
	if !ok || !strings.HasPrefix(prefix, agentModelDigestPrefix) {
		return 0, "", fmt.Errorf("installmeta: malformed agent model digest: missing %q version prefix",
			agentModelDigestPrefix+"<version>:")
	}
	version, err = strconv.Atoi(strings.TrimPrefix(prefix, agentModelDigestPrefix))
	if err != nil || version < 1 {
		return 0, "", fmt.Errorf("installmeta: malformed agent model digest version in %q", prefix)
	}
	if !isHex64(sum) {
		return 0, "", fmt.Errorf("installmeta: malformed agent model digest sum")
	}
	return version, sum, nil
}

// ValidAgentModelDigest reports whether digest is exactly the encoding
// produced by the current AgentModelDigestVersion. Unknown versions, legacy
// raw-hex strings, and malformed values are invalid, which lets every consumer
// fail closed on mismatch instead of degrading.
func ValidAgentModelDigest(digest string) bool {
	version, _, err := ParseAgentModelDigest(digest)
	return err == nil && version == AgentModelDigestVersion
}
