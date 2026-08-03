package pipeline

import (
	"slices"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

// ProfileDisposition is the explicit outcome of profile qualification.
type ProfileDisposition string

const (
	ProfileDispositionSelected        ProfileDisposition = "selected"
	ProfileDispositionDegraded        ProfileDisposition = "degraded"
	ProfileDispositionBlocked         ProfileDisposition = "blocked"
	ProfileReasonQualified                               = "profile/qualified-evidence"
	ProfileReasonEvidenceInsufficient                    = "profile/insufficient-fresh-evidence"
	ProfileReasonRequestedUnavailable                    = "profile/requested-profile-unavailable"
)

// ProfileResolutionInput contains only caller-provided profile policy and
// capability evidence. An empty Requested value means evidence-backed
// selection, not a production default.
type ProfileResolutionInput struct {
	Requested          sdd.WorkflowProfile
	Facts              []capability.CapabilityFact
	ExperimentalOptIns []capability.CapabilityID
	Now                time.Time
}

// ProfileDecision records requested/effective profile truth and the stable
// reason that led to it. Model routing is intentionally absent from this type.
type ProfileDecision struct {
	Requested             sdd.WorkflowProfile       `json:"requested"`
	Effective             sdd.WorkflowProfile       `json:"effective"`
	Disposition           ProfileDisposition        `json:"disposition"`
	ReasonID              string                    `json:"reason_id"`
	QualifiedCapabilities []capability.CapabilityID `json:"qualified_capabilities"`
	Evidence              []string                  `json:"evidence"`
	Degradations          []string                  `json:"degradations,omitempty"`
}

// ResolveProfileDecision selects the strongest profile qualified by fresh
// runtime evidence, bounded by an explicit request when one is supplied.
// Missing or stale evidence produces an explicit sequential degradation.
func ResolveProfileDecision(input ProfileResolutionInput) ProfileDecision {
	requested := input.Requested
	nativeCapabilities := make([]capability.CapabilityID, 0, len(input.Facts))
	for _, fact := range input.Facts {
		nativeCapabilities = append(nativeCapabilities, fact.ID)
	}
	slices.Sort(nativeCapabilities)
	nativeCapabilities = slices.Compact(nativeCapabilities)
	selection := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
		Now: input.Now, Facts: input.Facts, NativeCapabilities: nativeCapabilities,
		ExperimentalOptIns: input.ExperimentalOptIns,
	})

	effective := selection.Profile
	if requested == sdd.ProfilePortableSequential {
		effective = requested
	} else if requested == sdd.ProfilePortableFlat && profileRank(effective) > profileRank(requested) {
		effective = requested
	} else if requested != "" && requested != sdd.ProfilePortableSequential && requested != sdd.ProfilePortableFlat && requested != sdd.ProfileNativeAdvanced {
		return ProfileDecision{Requested: requested, Effective: "", Disposition: ProfileDispositionBlocked, ReasonID: "profile/unknown-request", Degradations: []string{"unknown requested profile"}}
	}

	decision := ProfileDecision{Requested: requested, Effective: effective, QualifiedCapabilities: slices.Clone(selection.QualifiedCapabilities), Degradations: slices.Clone(selection.Degradations)}
	for _, fact := range input.Facts {
		if slices.Contains(selection.QualifiedCapabilities, fact.ID) && isProvenFreshFact(fact, input.Now) {
			decision.Evidence = append(decision.Evidence, fact.EvidenceRef)
		}
	}
	if requested != "" && profileRank(effective) < profileRank(requested) {
		decision.Disposition, decision.ReasonID = ProfileDispositionDegraded, ProfileReasonRequestedUnavailable
		return decision
	}
	if effective == sdd.ProfilePortableSequential && len(selection.QualifiedCapabilities) == 0 {
		decision.Disposition, decision.ReasonID = ProfileDispositionDegraded, ProfileReasonEvidenceInsufficient
		return decision
	}
	decision.Disposition, decision.ReasonID = ProfileDispositionSelected, ProfileReasonQualified
	return decision
}

func isProvenFreshFact(fact capability.CapabilityFact, now time.Time) bool {
	if fact.Mode != capability.CapabilityAvailable || fact.Cardinality == capability.CardinalityNone || !fact.Current {
		return false
	}
	return !fact.ObservedAt.IsZero() && !fact.ObservedAt.After(now) && fact.FreshUntil.After(now) && fact.EvidenceRef != "" && fact.Confidence > 0 && fact.Confidence <= 1 && fact.Enforcement == capability.EnforcementRuntime
}
