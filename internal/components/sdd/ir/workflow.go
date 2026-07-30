package ir

import (
	"fmt"
	"regexp"
)

// SemanticID is a stable, canonical identifier shared by generated assets.
type SemanticID string

var semanticIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*(/[a-z0-9][a-z0-9.-]*)+$`)

// ValidateSemanticID requires a lower-case namespace and at least one name segment.
func ValidateSemanticID(id SemanticID) error {
	if !semanticIDPattern.MatchString(string(id)) {
		return fmt.Errorf("semantic ID %q must be a lower-case namespaced path", id)
	}
	return nil
}

// WorkflowIR is the canonical runtime-neutral workflow definition.
type WorkflowIR struct {
	SchemaVersion Version              `json:"schema_version"`
	ID            SemanticID           `json:"id"`
	Version       Version              `json:"version"`
	Roles         []Role               `json:"roles,omitempty"`
	Phases        []Phase              `json:"phases,omitempty"`
	Tools         []ToolRequirement    `json:"tools,omitempty"`
	Context       ContextPolicy        `json:"context,omitempty"`
	Services      []ServiceRequirement `json:"services,omitempty"`
	Profiles      []Profile            `json:"profiles,omitempty"`
	Extensions    []ExtensionContract  `json:"extensions,omitempty"`
}

// Role defines one generated agent's objective and authority boundary.
type Role struct {
	ID             SemanticID      `json:"id"`
	Objective      string          `json:"objective"`
	Inputs         []Contract      `json:"inputs,omitempty"`
	Outputs        []Contract      `json:"outputs,omitempty"`
	NonGoals       []string        `json:"non_goals,omitempty"`
	AllowedEffects []Effect        `json:"allowed_effects,omitempty"`
	Evidence       []SemanticID    `json:"evidence,omitempty"`
	TerminalStates []TerminalState `json:"terminal_states,omitempty"`
}

// Contract is a versioned role input or output contract.
type Contract struct {
	ID            SemanticID `json:"id"`
	SchemaVersion Version    `json:"schema_version"`
	Required      bool       `json:"required"`
}

// Phase expresses dependency intent without prescribing runtime scheduling.
type Phase struct {
	ID        SemanticID   `json:"id"`
	Role      SemanticID   `json:"role"`
	DependsOn []SemanticID `json:"depends_on,omitempty"`
}

// ToolRequirement requests a semantic tool rather than a target-specific name.
type ToolRequirement struct {
	ID       SemanticID `json:"id"`
	Required bool       `json:"required"`
}

// ContextPolicy declares the trust classes that a workflow may consume.
type ContextPolicy struct {
	Classes []TrustClass `json:"classes,omitempty"`
}

// ServiceRequirement declares an externally owned service compatibility bound.
type ServiceRequirement struct {
	ID      SemanticID   `json:"id"`
	Version VersionRange `json:"version"`
}

// Profile identifies a deterministic lowering profile.
type Profile struct {
	ID           SemanticID `json:"id"`
	Experimental bool       `json:"experimental"`
}

// ResolutionState is the complete semantic capability resolution vocabulary.
type ResolutionState string

const (
	ResolutionNative      ResolutionState = "native"
	ResolutionEmulated    ResolutionState = "emulated"
	ResolutionAdvisory    ResolutionState = "advisory"
	ResolutionUnsupported ResolutionState = "unsupported"
)

// ExtensionContract defines a provider-neutral extension boundary. Unsupported
// extensions are deliberately unbound and therefore cannot contribute tools or
// permissions to generated assets.
type ExtensionContract struct {
	ID                SemanticID      `json:"id"`
	SchemaVersion     Version         `json:"schema_version"`
	DefaultResolution ResolutionState `json:"default_resolution"`
	Provider          string          `json:"provider,omitempty"`
	Tools             []SemanticID    `json:"tools,omitempty"`
	Permissions       []string        `json:"permissions,omitempty"`
}

func (e ExtensionContract) Validate() error {
	if err := ValidateSemanticID(e.ID); err != nil {
		return fmt.Errorf("extension ID: %w", err)
	}
	if e.ID != "extension/remote-agent-a2a" {
		return fmt.Errorf("extension %q is not a supported provider-neutral boundary", e.ID)
	}
	if e.SchemaVersion.Major == 0 {
		return fmt.Errorf("extension %q schema version is required", e.ID)
	}
	if e.DefaultResolution != ResolutionUnsupported {
		return fmt.Errorf("extension %q must default to unsupported", e.ID)
	}
	if e.Provider != "" || len(e.Tools) != 0 || len(e.Permissions) != 0 {
		return fmt.Errorf("extension %q cannot expose provider tools or permissions without qualification", e.ID)
	}
	return nil
}

type Effect SemanticID
type TerminalState string
type TrustClass string

const (
	TerminalSuccess TerminalState = "success"
	TerminalBlocked TerminalState = "blocked"
	TerminalFailed  TerminalState = "failed"

	TrustTrustedPolicy   TrustClass = "trusted_policy"
	TrustTrustedSchema   TrustClass = "trusted_schema"
	TrustOperatorInput   TrustClass = "operator_input"
	TrustRepositoryData  TrustClass = "repository_data"
	TrustToolOutput      TrustClass = "tool_output"
	TrustPeerMessage     TrustClass = "peer_message"
	TrustRemoteUntrusted TrustClass = "remote_untrusted"
	TrustSecretReference TrustClass = "secret_reference"
)
