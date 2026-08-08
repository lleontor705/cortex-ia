// Package sdd — profile helpers for OpenCode SDD multi-model setups.
//
// A "profile" is a named bundle of per-phase model assignments. Multiple
// profiles can coexist; the active one drives which provider/model the
// OpenCode adapter writes for each SDD phase.
package sdd

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// WorkflowProfile is the capability-aware lowering profile selected for a
// generated workflow bundle.
type WorkflowProfile string

const (
	ProfilePortableSequential WorkflowProfile = "portable-sequential"
	ProfilePortableFlat       WorkflowProfile = "portable-flat"
	ProfileNativeAdvanced     WorkflowProfile = "native-advanced"

	directChildDelegation capability.CapabilityID = "delegation/direct-child"
)

// ProfileSelectionInput contains only caller-supplied evidence and policy.
// Now is explicit so identical inputs always produce identical output.
type ProfileSelectionInput struct {
	Now                time.Time
	Facts              []capability.CapabilityFact
	NativeCapabilities []capability.CapabilityID
	ExperimentalOptIns []capability.CapabilityID
}

// ProfileSelection records both the selected profile and conservative
// degradation reasons for capabilities that could not qualify it.
type ProfileSelection struct {
	Profile               WorkflowProfile           `json:"profile"`
	QualifiedCapabilities []capability.CapabilityID `json:"qualified_capabilities"`
	Degradations          []string                  `json:"degradations"`
}

// SelectWorkflowProfile chooses the strongest qualified profile. Sequential is
// always available without delegation; flat requires proven direct-child
// delegation; native additionally requires every requested native capability.
func SelectWorkflowProfile(input ProfileSelectionInput) ProfileSelection {
	selection := ProfileSelection{
		Profile:               ProfilePortableSequential,
		QualifiedCapabilities: []capability.CapabilityID{},
		Degradations:          []string{},
	}
	optIns := capabilitySet(input.ExperimentalOptIns)

	qualified, reason := qualifyProfileCapability(directChildDelegation, input.Facts, input.Now, optIns)
	if !qualified {
		selection.Degradations = append(selection.Degradations, capabilityDegradation(directChildDelegation, reason))
		return selection
	}
	selection.Profile = ProfilePortableFlat
	selection.QualifiedCapabilities = append(selection.QualifiedCapabilities, directChildDelegation)

	for _, id := range sortedCapabilityIDs(input.NativeCapabilities) {
		if id == directChildDelegation {
			continue
		}
		qualified, reason = qualifyProfileCapability(id, input.Facts, input.Now, optIns)
		if !qualified {
			selection.Degradations = append(selection.Degradations, capabilityDegradation(id, reason))
			continue
		}
		selection.QualifiedCapabilities = append(selection.QualifiedCapabilities, id)
	}

	if len(selection.Degradations) == 0 && len(selection.QualifiedCapabilities) > 1 {
		selection.Profile = ProfileNativeAdvanced
	}
	return selection
}

// SelectCompiledWorkflowProfile derives profile selection from the normalized
// compiler snapshot and rejects a snapshot whose recorded profile disagrees.
// This prevents injection from consulting mutable runtime state after compile.
func SelectCompiledWorkflowProfile(compiled compiler.Result, nativeCapabilities, experimentalOptIns []capability.CapabilityID) (ProfileSelection, error) {
	evaluationTime, err := time.Parse(time.RFC3339Nano, compiled.Normalized.EvaluationTime)
	if err != nil {
		return ProfileSelection{}, fmt.Errorf("compiled evaluation time: %w", err)
	}
	selection := SelectWorkflowProfile(ProfileSelectionInput{
		Now:                evaluationTime,
		Facts:              compiled.Normalized.Catalog.Facts,
		NativeCapabilities: nativeCapabilities,
		ExperimentalOptIns: experimentalOptIns,
	})
	if compiled.Normalized.Profile != string(selection.Profile) {
		return ProfileSelection{}, fmt.Errorf("compiled profile %q does not match deterministic selection %q", compiled.Normalized.Profile, selection.Profile)
	}
	return selection, nil
}

func qualifyProfileCapability(id capability.CapabilityID, facts []capability.CapabilityFact, now time.Time, optIns map[capability.CapabilityID]struct{}) (bool, string) {
	experimentalQualified := false
	for _, fact := range facts {
		if fact.ID != id || !isProvenFreshFact(fact, now) {
			continue
		}
		if !fact.Experimental {
			return true, ""
		}
		experimentalQualified = true
		if _, optedIn := optIns[id]; optedIn {
			return true, ""
		}
	}
	if experimentalQualified {
		return false, "experimental capability requires explicit opt-in"
	}
	return false, "no fresh proven capability fact"
}

func isProvenFreshFact(fact capability.CapabilityFact, now time.Time) bool {
	if fact.Mode != capability.CapabilityAvailable || fact.Cardinality == capability.CardinalityNone || !fact.Current {
		return false
	}
	if fact.ObservedAt.IsZero() || fact.ObservedAt.After(now) || !fact.FreshUntil.After(now) {
		return false
	}
	if strings.TrimSpace(fact.EvidenceRef) == "" || fact.Confidence <= 0 || fact.Confidence > 1 {
		return false
	}
	if fact.Enforcement != capability.EnforcementRuntime {
		return false
	}
	return fact.EvidenceClass == capability.EvidenceInstalledSchema ||
		fact.EvidenceClass == capability.EvidenceExecutableProbe ||
		fact.EvidenceClass == capability.EvidenceRuntimeObserved
}

func capabilitySet(ids []capability.CapabilityID) map[capability.CapabilityID]struct{} {
	result := make(map[capability.CapabilityID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func sortedCapabilityIDs(ids []capability.CapabilityID) []capability.CapabilityID {
	result := slices.Clone(ids)
	slices.Sort(result)
	return slices.Compact(result)
}

func capabilityDegradation(id capability.CapabilityID, reason string) string {
	return string(id) + ": " + reason
}

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

	p := model.Profile{Name: name, ConfiguredAssignments: make(map[string]model.OpenCodeModelAssignment, len(ProfilePhaseOrder()))}
	for _, phase := range ProfilePhaseOrder() {
		p.ConfiguredAssignments[phase] = model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
	}
	if route, routeErr := modelroute.NewRouteID(providerModel); routeErr == nil {
		p.Routes = make(map[string]modelroute.RouteRequest, len(ProfilePhaseOrder()))
		p.ConfiguredAssignments = nil
		for _, phase := range ProfilePhaseOrder() {
			p.Routes[phase] = modelroute.RouteRequest{RouteID: route}
		}
	}
	return p, nil
}

// ParseProfilePhaseSpec parses `name:phase:provider/model` and returns the
// phase + assignment so callers can update an existing profile in place.
//
// Example: "cheap:sdd-design:provider-test/model-test" → ("cheap", "sdd-design", "provider-test/model-test", nil)
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
		existing = model.Profile{Name: profileName, ModelAssignments: map[string]string{}}
	}
	if route, err := modelroute.NewRouteID(providerModel); err == nil {
		if existing.Routes == nil {
			existing.Routes = map[string]modelroute.RouteRequest{}
		}
		existing.Routes[phase] = modelroute.RouteRequest{RouteID: route}
	} else {
		provider, modelID, ok := strings.Cut(strings.TrimSpace(providerModel), "/")
		if !ok || provider == "" || modelID == "" {
			return profiles
		}
		if existing.ConfiguredAssignments == nil {
			existing.ConfiguredAssignments = map[string]model.OpenCodeModelAssignment{}
		}
		existing.ConfiguredAssignments[phase] = model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
	}
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
// Profile.ModelAssignments stores explicit provider/model values or semantic
// route identifiers. Both shapes are normalised here.
//
// Phase keys lose their "sdd-" prefix because ApplyToOpenCodeConfig re-adds it
// when looking up agents in opencode.json.
func ProfileToOpenCodeAssignments(p model.Profile) model.OpenCodeModelAssignments {
	out := make(model.OpenCodeModelAssignments, len(p.ModelAssignments))
	for phase, value := range p.ConfiguredAssignments {
		assignment := value
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
		return []string{"implement"}
	case "implement":
		return []string{normalized}
	case "sdd-verify", "verify", "validate":
		return []string{"validate"}
	case "sdd-archive", "archive", "finalize":
		return []string{"finalize"}
	case "orchestrator", "debate", "parallel-dispatch":
		return []string{normalized}
	default:
		return nil
	}
}

// ProfileSummary renders a one-line description of a profile for CLI listing.
func ProfileSummary(p model.Profile) string {
	assignments := p.ConfiguredAssignments
	if len(assignments) == 0 {
		assignments = map[string]model.OpenCodeModelAssignment{}
	}
	if len(p.Routes) == 0 && len(assignments) == 0 {
		return fmt.Sprintf("%-20s (no phase assignments)", p.Name)
	}
	// Identify the dominant model — if every phase shares the same value,
	// summarise as "<name> → <model>". Otherwise show "(N phases configured)".
	var seen string
	uniform := true
	for _, phase := range ProfilePhaseOrder() {
		v, ok := assignments[phase]
		if !ok {
			uniform = false
			continue
		}
		if seen == "" {
			seen = v.FormatOpenCodeModel()
			continue
		}
		if v.FormatOpenCodeModel() != seen {
			uniform = false
		}
	}
	if uniform && seen != "" {
		return fmt.Sprintf("%-20s → %s (all phases)", p.Name, seen)
	}
	return fmt.Sprintf("%-20s %d phase(s) configured", p.Name, len(assignments)+len(p.Routes))
}
