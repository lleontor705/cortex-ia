// Package sdd — profile helpers for OpenCode SDD multi-model setups.
//
// A "profile" is a named bundle of per-phase model assignments. Multiple
// profiles can coexist; the active one drives which provider/model the
// OpenCode adapter writes for each SDD phase.
package sdd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// ProfilePhaseOrder is the canonical SDD phase order used everywhere a profile
// needs to enumerate phases (TUI listing, --profile-phase parser, validation).
func ProfilePhaseOrder() []string {
	return []string{
		"sdd-init", "sdd-explore", "sdd-propose", "sdd-spec",
		"sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive",
	}
}

var phaseSet = func() map[string]struct{} {
	m := make(map[string]struct{}, 9)
	for _, p := range ProfilePhaseOrder() {
		m[p] = struct{}{}
	}
	return m
}()

// IsKnownPhase reports whether name is one of the canonical SDD phases.
func IsKnownPhase(name string) bool {
	_, ok := phaseSet[name]
	return ok
}

// profileNameRE accepts kebab-case identifiers up to 40 chars.
var profileNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

// ValidateProfileName enforces kebab-case and a 40-char limit. Empty is invalid.
func ValidateProfileName(name string) error {
	if !profileNameRE.MatchString(name) {
		return fmt.Errorf("profile name %q must be kebab-case (a-z, 0-9, -) and 1-40 chars", name)
	}
	return nil
}

// ParseProfileSpec parses `name:provider/model` and returns a Profile with the
// given model assignment applied to ALL phases (the "set all" preset shortcut).
//
// Example: "cheap:openai/gpt-4o-mini" → Profile{Name:"cheap", every phase = openai/gpt-4o-mini}
func ParseProfileSpec(spec string) (model.Profile, error) {
	name, providerModel, ok := strings.Cut(spec, ":")
	if !ok {
		return model.Profile{}, fmt.Errorf("invalid profile spec %q: expected name:provider/model", spec)
	}
	if err := ValidateProfileName(name); err != nil {
		return model.Profile{}, err
	}

	provider, modelID, ok := strings.Cut(providerModel, "/")
	if !ok || provider == "" || modelID == "" {
		return model.Profile{}, fmt.Errorf("invalid provider/model %q: expected provider/model", providerModel)
	}

	assignments := make(map[string]model.ClaudeModelAlias, len(ProfilePhaseOrder()))
	for _, phase := range ProfilePhaseOrder() {
		assignments[phase] = model.ClaudeModelAlias(provider + "/" + modelID)
	}
	return model.Profile{Name: name, ModelAssignments: assignments}, nil
}

// ParseProfilePhaseSpec parses `name:phase:provider/model` and returns the
// phase + assignment so callers can update an existing profile in place.
//
// Example: "cheap:sdd-design:anthropic/claude-opus-4" → ("cheap", "sdd-design", "anthropic/claude-opus-4", nil)
func ParseProfilePhaseSpec(spec string) (profileName, phase, providerModel string, err error) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid profile-phase spec %q: expected name:phase:provider/model", spec)
	}
	profileName, phase, providerModel = parts[0], parts[1], parts[2]
	if validateErr := ValidateProfileName(profileName); validateErr != nil {
		return "", "", "", validateErr
	}
	if !IsKnownPhase(phase) {
		return "", "", "", fmt.Errorf("unknown SDD phase %q (valid: %v)", phase, ProfilePhaseOrder())
	}
	if !strings.Contains(providerModel, "/") {
		return "", "", "", fmt.Errorf("invalid provider/model %q: expected provider/model", providerModel)
	}
	return profileName, phase, providerModel, nil
}

// FindProfile returns the profile with the given name (or false if missing).
func FindProfile(profiles []model.Profile, name string) (model.Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return model.Profile{}, false
}

// UpsertProfile inserts or replaces a profile by name in the given slice.
// Returns the new slice — callers should reassign.
func UpsertProfile(profiles []model.Profile, p model.Profile) []model.Profile {
	for i := range profiles {
		if profiles[i].Name == p.Name {
			profiles[i] = p
			return profiles
		}
	}
	return append(profiles, p)
}

// SetProfilePhase mutates an existing profile to set a single phase's model.
// If the profile doesn't exist, a new empty one is created.
func SetProfilePhase(profiles []model.Profile, profileName, phase, providerModel string) []model.Profile {
	existing, ok := FindProfile(profiles, profileName)
	if !ok {
		existing = model.Profile{Name: profileName, ModelAssignments: map[string]model.ClaudeModelAlias{}}
	}
	if existing.ModelAssignments == nil {
		existing.ModelAssignments = map[string]model.ClaudeModelAlias{}
	}
	existing.ModelAssignments[phase] = model.ClaudeModelAlias(providerModel)
	return UpsertProfile(profiles, existing)
}

// RemoveProfile drops the profile with the given name. Returns the new slice
// and a bool indicating whether anything was removed.
func RemoveProfile(profiles []model.Profile, name string) ([]model.Profile, bool) {
	for i, p := range profiles {
		if p.Name == name {
			return append(profiles[:i], profiles[i+1:]...), true
		}
	}
	return profiles, false
}

// ProfileToOpenCodeAssignments converts a saved Profile into the
// OpenCodeModelAssignments shape consumed by opencode.ApplyToOpenCodeConfig.
//
// Profile.ModelAssignments stores values either as a Claude alias
// ("opus" / "sonnet" / "haiku") or as a fully-qualified "provider/model"
// (what ParseProfileSpec emits). Both shapes are normalised here.
//
// Phase keys lose their "sdd-" prefix because ApplyToOpenCodeConfig re-adds it
// when looking up agents in opencode.json.
func ProfileToOpenCodeAssignments(p model.Profile) model.OpenCodeModelAssignments {
	out := make(model.OpenCodeModelAssignments, len(p.ModelAssignments))
	for phase, value := range p.ModelAssignments {
		assignment := parseProfileValue(string(value))
		if assignment.Provider == "" || assignment.Model == "" {
			continue
		}
		for _, agentName := range profileKeyToOpenCodeAgents(phase) {
			out[agentName] = assignment
		}
	}
	return out
}

func profileKeyToOpenCodeAgents(key string) []string {
	normalized := strings.TrimSpace(key)
	switch normalized {
	case "sdd-init", "init", "bootstrap":
		return []string{"bootstrap"}
	case "sdd-explore", "explore", "investigate":
		return []string{"investigate"}
	case "sdd-propose", "propose", "draft-proposal":
		return []string{"draft-proposal"}
	case "sdd-spec", "spec", "write-specs":
		return []string{"write-specs"}
	case "sdd-design", "design", "architect":
		return []string{"architect"}
	case "sdd-tasks", "tasks", "decompose":
		return []string{"decompose"}
	case "sdd-apply", "apply":
		return []string{"team-lead", "implement"}
	case "team-lead", "implement":
		return []string{normalized}
	case "sdd-verify", "verify", "validate":
		return []string{"validate"}
	case "sdd-archive", "archive", "finalize":
		return []string{"finalize"}
	case "orchestrator", "parallel-dispatch":
		return []string{normalized}
	default:
		return nil
	}
}

// parseProfileValue handles both shapes profiles can store:
//   - "anthropic/claude-opus-4"      → split into provider + model
//   - "opus" / "sonnet" / "haiku"    → expand to anthropic/claude-<alias>-N
//   - anything else                  → zero assignment (caller filters out)
func parseProfileValue(value string) model.OpenCodeModelAssignment {
	v := strings.TrimSpace(value)
	if v == "" {
		return model.OpenCodeModelAssignment{}
	}

	if provider, modelID, ok := strings.Cut(v, "/"); ok {
		return model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
	}

	switch v {
	case string(model.ModelOpus):
		return model.OpenCodeModelAssignment{Provider: "anthropic", Model: "claude-opus-4"}
	case string(model.ModelSonnet):
		return model.OpenCodeModelAssignment{Provider: "anthropic", Model: "claude-sonnet-4-6"}
	case string(model.ModelHaiku):
		return model.OpenCodeModelAssignment{Provider: "anthropic", Model: "claude-haiku-4-5"}
	default:
		return model.OpenCodeModelAssignment{}
	}
}

// ProfileSummary renders a one-line description of a profile for CLI listing.
func ProfileSummary(p model.Profile) string {
	if len(p.ModelAssignments) == 0 {
		return fmt.Sprintf("%-20s (no phase assignments)", p.Name)
	}
	// Identify the dominant model — if every phase shares the same value,
	// summarise as "<name> → <model>". Otherwise show "(N phases configured)".
	var seen string
	uniform := true
	for _, phase := range ProfilePhaseOrder() {
		v, ok := p.ModelAssignments[phase]
		if !ok {
			uniform = false
			continue
		}
		if seen == "" {
			seen = string(v)
			continue
		}
		if string(v) != seen {
			uniform = false
		}
	}
	if uniform && seen != "" {
		return fmt.Sprintf("%-20s → %s (all phases)", p.Name, seen)
	}
	return fmt.Sprintf("%-20s %d phase(s) configured", p.Name, len(p.ModelAssignments))
}
