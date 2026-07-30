package quality

// ChangeKind classifies the primary intent of a change without using model confidence.
type ChangeKind string

const (
	ChangeTrivial       ChangeKind = "trivial"
	ChangeBehavior      ChangeKind = "behavior"
	ChangeRefactor      ChangeKind = "refactor"
	ChangeDocumentation ChangeKind = "documentation"
	ChangeSecurity      ChangeKind = "security"
	ChangeMigration     ChangeKind = "migration"
)

type RiskLevel uint8

const (
	RiskLow RiskLevel = iota + 1
	RiskMedium
	RiskHigh
	RiskCritical
)

type Reversibility string

const (
	ReversibilityEasy       Reversibility = "easy"
	ReversibilityDifficult  Reversibility = "difficult"
	ReversibilityImpossible Reversibility = "impossible"
)

type TrustBoundary string

const (
	TrustBoundaryInternal TrustBoundary = "internal"
	TrustBoundaryCrossed  TrustBoundary = "crossed"
)

type DependencyBreadth string

const (
	DependencyLocal       DependencyBreadth = "local"
	DependencyPackage     DependencyBreadth = "package"
	DependencyCrossDomain DependencyBreadth = "cross-domain"
	DependencyExternal    DependencyBreadth = "external"
)

type MigrationImpact string

const (
	MigrationNone          MigrationImpact = "none"
	MigrationConfiguration MigrationImpact = "configuration"
	MigrationData          MigrationImpact = "data"
	MigrationCompatibility MigrationImpact = "compatibility"
)

type DeliveryStage string

const (
	StageLocal       DeliveryStage = "local"
	StagePullRequest DeliveryStage = "pull-request"
	StageRelease     DeliveryStage = "release"
	StageScheduled   DeliveryStage = "scheduled"
)

type EvidenceNeed string

const (
	EvidenceRegression     EvidenceNeed = "regression"
	EvidenceSecurityReview EvidenceNeed = "security-review"
	EvidenceMigration      EvidenceNeed = "migration"
	EvidenceRuntime        EvidenceNeed = "runtime"
)

// TestCapabilities are policy inputs, not claims that a runtime will enforce a test mode.
type TestCapabilities struct {
	DeterministicFocusedRunner bool
	WritableTests              bool
	BaselineEvidence           bool
}

func (c TestCapabilities) VerticalTDDEligible() bool {
	return c.DeterministicFocusedRunner && c.WritableTests && c.BaselineEvidence
}

type ChangeContext struct {
	Kind               ChangeKind
	ObservableBehavior bool
	Risk               RiskLevel
	Reversibility      Reversibility
	TrustBoundary      TrustBoundary
	DependencyBreadth  DependencyBreadth
	MigrationImpact    MigrationImpact
	EvidenceNeeds      []EvidenceNeed
	Tests              TestCapabilities
	Stage              DeliveryStage
}

// ChangeSelector declares only objective change attributes. Model confidence is
// intentionally absent because confidence cannot waive required planning.
type ChangeSelector struct {
	Kinds              []ChangeKind
	ObservableBehavior *bool
	MinimumRisk        *RiskLevel
	Reversibility      []Reversibility
	TrustBoundaries    []TrustBoundary
	DependencyBreadth  []DependencyBreadth
	MigrationImpacts   []MigrationImpact
	EvidenceNeeds      []EvidenceNeed
	RequiresTDD        *bool
	Stages             []DeliveryStage
}

func (s ChangeSelector) Matches(change ChangeContext) bool {
	if !containsOrAny(s.Kinds, change.Kind) ||
		!containsOrAny(s.Reversibility, change.Reversibility) ||
		!containsOrAny(s.TrustBoundaries, change.TrustBoundary) ||
		!containsOrAny(s.DependencyBreadth, change.DependencyBreadth) ||
		!containsOrAny(s.MigrationImpacts, change.MigrationImpact) ||
		!containsAll(change.EvidenceNeeds, s.EvidenceNeeds) ||
		!containsOrAny(s.Stages, change.Stage) {
		return false
	}
	if s.ObservableBehavior != nil && *s.ObservableBehavior != change.ObservableBehavior {
		return false
	}
	if s.MinimumRisk != nil && change.Risk < *s.MinimumRisk {
		return false
	}
	if s.RequiresTDD != nil && *s.RequiresTDD != change.Tests.VerticalTDDEligible() {
		return false
	}
	return true
}

func containsAll[T comparable](available, required []T) bool {
	for _, requirement := range required {
		if !contains(available, requirement) {
			return false
		}
	}
	return true
}

func contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsOrAny[T comparable](allowed []T, observed T) bool {
	if len(allowed) == 0 {
		return true
	}
	return contains(allowed, observed)
}
