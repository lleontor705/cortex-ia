package quality

type OutcomeStatus string

const (
	OutcomePass         OutcomeStatus = "pass"
	OutcomeFail         OutcomeStatus = "fail"
	OutcomeInconclusive OutcomeStatus = "inconclusive"
	OutcomeDegraded     OutcomeStatus = "degraded"
)

type TerminationCause string

const (
	TerminationNone                TerminationCause = "none"
	TerminationBudgetExhausted     TerminationCause = "budget-exhausted"
	TerminationMissingCapability   TerminationCause = "missing-capability"
	TerminationFlakyInfrastructure TerminationCause = "flaky-infrastructure"
	TerminationTimeout             TerminationCause = "timeout"
	TerminationCancelled           TerminationCause = "cancelled"
	TerminationInsufficientTrials  TerminationCause = "insufficient-trials"
)

// Outcome preserves non-successful termination instead of converting partial
// or exhausted work into a false pass.
type Outcome struct {
	Status          OutcomeStatus
	Cause           TerminationCause
	PartialEvidence []string
	RetryReason     string
	OriginalFailure string
}

func OutcomeForTermination(cause TerminationCause, completed bool) OutcomeStatus {
	switch cause {
	case TerminationBudgetExhausted, TerminationTimeout, TerminationInsufficientTrials:
		return OutcomeInconclusive
	case TerminationMissingCapability, TerminationFlakyInfrastructure, TerminationCancelled:
		return OutcomeDegraded
	default:
		if completed {
			return OutcomePass
		}
		return OutcomeFail
	}
}

// EvaluateActivity treats budget exhaustion as inconclusive even when the
// activity otherwise completed and preserves any evidence produced so far.
func EvaluateActivity(budget ActivityBudget, usage ActivityUsage, completed bool, partialEvidence []string) Outcome {
	cause := TerminationNone
	if budget.ExhaustedBy(usage) {
		cause = TerminationBudgetExhausted
	}
	return Outcome{
		Status:          OutcomeForTermination(cause, completed),
		Cause:           cause,
		PartialEvidence: append([]string(nil), partialEvidence...),
	}
}
