package modelmgr

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

// ModelSource classifies where an agent's effective model reference comes
// from. It is the manager's status taxonomy for list/get reporting.
type ModelSource string

const (
	// SourceManaged is a config agents entry accredited by an ownership record.
	SourceManaged ModelSource = "managed"

	// SourceConfig is a config agents entry with no ownership record: a
	// user-authored entry cortex-ia never wrote.
	SourceConfig ModelSource = "config"

	// SourceMarkdown is a markdown agent frontmatter pin with no overriding
	// config entry. v1 surfaces the pin and never rewrites it.
	SourceMarkdown ModelSource = "markdown"

	// SourceUnset is an agent with no config entry and no markdown pin.
	SourceUnset ModelSource = "unset"
)

// Desired is the typed, validated description of one agent-model assignment.
// It is pure data: validating it performs no I/O, so a malformed request fails
// closed before any configuration access.
type Desired struct {
	Agent    string
	Provider string
	Model    string
	Variant  string
}

// InvalidDesiredError reports a desired reference that violates the free-form
// shape contract. The error names the offending field and reason, never the
// offending value.
type InvalidDesiredError struct {
	Agent  string
	Field  string
	Reason string
}

func (e *InvalidDesiredError) Error() string {
	return fmt.Sprintf("modelmgr: desired model for agent %q field %q is invalid: %s", e.Agent, e.Field, e.Reason)
}

func invalidDesired(agent, field, reason string) error {
	return &InvalidDesiredError{Agent: agent, Field: field, Reason: reason}
}

// ParseModelRef splits and validates a compact reference into its provider,
// model, and optional variant tokens. The variant is split at the rightmost
// "#", so an earlier "#" surfaces as an invalid model token instead of being
// silently accepted.
func ParseModelRef(agent, ref string) (Desired, error) {
	body := ref
	variant := ""
	if idx := strings.LastIndexByte(ref, '#'); idx >= 0 {
		body = ref[:idx]
		variant = ref[idx+1:]
		if variant == "" {
			return Desired{}, invalidDesired(agent, "variant", "must be non-empty when '#' is present")
		}
	}
	provider, model, ok := strings.Cut(body, "/")
	if !ok {
		return Desired{}, invalidDesired(agent, "model", "must be a free-form provider/model reference")
	}
	desired := Desired{Agent: agent, Provider: provider, Model: model, Variant: variant}
	if err := desired.Validate(); err != nil {
		return Desired{}, err
	}
	return desired, nil
}

// ParseDesired builds a validated desired reference from a CLI-style argument.
// A non-empty effort overrides any "#variant" suffix.
func ParseDesired(agent, ref, effort string) (Desired, error) {
	desired, err := ParseModelRef(agent, ref)
	if err != nil {
		return Desired{}, err
	}
	if effort != "" {
		if reason := tokenProblem(effort, false); reason != "" {
			return Desired{}, invalidDesired(agent, "variant", reason)
		}
		desired.Variant = effort
	}
	return desired, nil
}

// Validate enforces the free-form shape contract: non-empty provider and model
// tokens without whitespace, "#", or control characters, and an optional
// non-empty variant token without whitespace or control characters. It is pure
// and returns a typed *InvalidDesiredError on the first violation.
func (d Desired) Validate() error {
	if reason := tokenProblem(d.Agent, true); reason != "" {
		return invalidDesired(d.Agent, "agent", reason)
	}
	if reason := tokenProblem(d.Provider, true); reason != "" {
		return invalidDesired(d.Agent, "provider", reason)
	}
	if reason := tokenProblem(d.Model, true); reason != "" {
		return invalidDesired(d.Agent, "model", reason)
	}
	if d.Variant != "" {
		if reason := tokenProblem(d.Variant, false); reason != "" {
			return invalidDesired(d.Agent, "variant", reason)
		}
	}
	return nil
}

// Compact returns the canonical stored form provider/model#variant.
func (d Desired) Compact() string {
	ref := d.Provider + "/" + d.Model
	if d.Variant != "" {
		ref += "#" + d.Variant
	}
	return ref
}

// Identity returns the digest identity installmeta hashes for ownership.
func (d Desired) Identity() installmeta.AgentModelIdentity {
	return installmeta.AgentModelIdentity{
		Agent:    d.Agent,
		Provider: d.Provider,
		Model:    d.Model,
		Variant:  d.Variant,
	}
}

// Digest returns the versioned ownership digest of the desired reference. The
// encoding is deterministic, so the same reference always yields the same
// digest across runs.
func (d Desired) Digest() (string, error) {
	return installmeta.AgentModelIdentityDigest(d.Identity())
}

// DecodeModelRef accepts a stored agents model value in either the compact
// string or the expanded object form and returns the canonical reference.
// Malformed config values fail closed with a typed *ConflictError.
func DecodeModelRef(agent string, value any) (Desired, error) {
	switch v := value.(type) {
	case string:
		desired, err := ParseModelRef(agent, v)
		if err != nil {
			return Desired{}, configShapeError(agent, "model", invalidReason(err))
		}
		return desired, nil
	case map[string]any:
		return decodeExpandedModel(agent, v)
	default:
		return Desired{}, configShapeError(agent, "model", "must be a compact string or an expanded object")
	}
}

// expandedModelMembers is the exact member set of the documented expanded
// object form. Unknown members fail closed rather than being silently dropped
// by a later compact rewrite.
var expandedModelMembers = map[string]struct{}{
	"providerID": {},
	"model":      {},
	"variant":    {},
}

func decodeExpandedModel(agent string, value map[string]any) (Desired, error) {
	for member := range value {
		if _, known := expandedModelMembers[member]; !known {
			return Desired{}, configShapeError(agent, "model", fmt.Sprintf("expanded object has unknown member %q", member))
		}
	}
	provider, err := stringMember(agent, value, "providerID")
	if err != nil {
		return Desired{}, err
	}
	model, err := stringMember(agent, value, "model")
	if err != nil {
		return Desired{}, err
	}
	variant := ""
	if raw, present := value["variant"]; present {
		text, ok := raw.(string)
		if !ok {
			return Desired{}, configShapeError(agent, "model.variant", "must be a string")
		}
		if text == "" {
			return Desired{}, configShapeError(agent, "model.variant", "must be non-empty when present")
		}
		variant = text
	}
	desired := Desired{Agent: agent, Provider: provider, Model: model, Variant: variant}
	if err := desired.Validate(); err != nil {
		return Desired{}, configShapeError(agent, "model", invalidReason(err))
	}
	return desired, nil
}

func stringMember(agent string, value map[string]any, member string) (string, error) {
	raw, present := value[member]
	if !present {
		return "", configShapeError(agent, "model."+member, "is required in the expanded object form")
	}
	text, ok := raw.(string)
	if !ok {
		return "", configShapeError(agent, "model."+member, "must be a string")
	}
	return text, nil
}

// tokenProblem reports why token violates the free-form token charset, or ""
// when it is acceptable. Provider and model additionally reject "#" because it
// delimits the variant suffix.
func tokenProblem(token string, rejectHash bool) string {
	if token == "" {
		return "must be non-empty"
	}
	for _, r := range token {
		switch {
		case unicode.IsSpace(r):
			return "must not contain whitespace"
		case unicode.IsControl(r):
			return "must not contain control characters"
		case rejectHash && r == '#':
			return "must not contain '#'"
		}
	}
	return ""
}

func invalidReason(err error) string {
	var invalid *InvalidDesiredError
	if errors.As(err, &invalid) {
		return fmt.Sprintf("%s %s", invalid.Field, invalid.Reason)
	}
	return err.Error()
}
