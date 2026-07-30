package quality

import (
	"fmt"
	"strings"
	"time"
)

type VerificationLevel string

const (
	VerificationUnit        VerificationLevel = "unit"
	VerificationIntegration VerificationLevel = "integration"
	VerificationEndToEnd    VerificationLevel = "end-to-end"
)

type TestResult struct {
	At             time.Time
	ExitStatus     int
	OutputDigest   string
	FailureDetails string
}

type TDDExceptionReason string

const (
	TDDInapplicable        TDDExceptionReason = "inapplicable"
	TDDUnavailableRunner   TDDExceptionReason = "unavailable-runner"
	TDDUnwritableTests     TDDExceptionReason = "unwritable-tests"
	TDDMissingBaseline     TDDExceptionReason = "missing-baseline"
	TDDNonDeterministicRun TDDExceptionReason = "non-deterministic-runner"
)

type TDDException struct {
	Reason               TDDExceptionReason
	Detail               string
	CompensatingEvidence []string
}

// VerticalTDDEvidence proves ordering and results for one observable work unit.
// A narrative mode flag is intentionally insufficient.
type VerticalTDDEvidence struct {
	RequirementIDs        []string
	ExampleIDs            []string
	CommitOrTreeState     string
	Command               string
	WorkingDirectory      string
	RedAt                 time.Time
	ProductionChangedAt   time.Time
	RedExitStatus         int
	RedFailureFingerprint string
	Green                 TestResult
	Refactor              TestResult
	Artifacts             []string
	VerificationLevel     VerificationLevel
	Exception             *TDDException
}

func (e VerticalTDDEvidence) Validate() error {
	if e.Exception != nil {
		if e.Exception.Reason == "" || e.Exception.Detail == "" || len(e.Exception.CompensatingEvidence) == 0 {
			return fmt.Errorf("TDD exception requires reason, detail, and compensating evidence")
		}
		if !recognizedExceptionReason(e.Exception.Reason) {
			return fmt.Errorf("TDD exception requires a recognized reason")
		}
		return nil
	}
	if len(e.RequirementIDs) == 0 || e.Command == "" || e.WorkingDirectory == "" ||
		e.CommitOrTreeState == "" || len(e.Artifacts) == 0 || e.VerificationLevel == "" {
		return fmt.Errorf("vertical TDD evidence is missing requirement, tree, command, cwd, artifact, or verification data")
	}
	if normalizedCommand := strings.Join(strings.Fields(e.Command), " "); e.Command != normalizedCommand {
		return fmt.Errorf("TDD evidence command must be normalized")
	}
	if e.RedAt.IsZero() || e.ProductionChangedAt.IsZero() || e.Green.At.IsZero() || e.Refactor.At.IsZero() {
		return fmt.Errorf("vertical TDD evidence requires RED, production, GREEN, and REFACTOR timestamps")
	}
	if !e.ProductionChangedAt.After(e.RedAt) {
		return fmt.Errorf("production changed before RED evidence")
	}
	if e.RedExitStatus == 0 || e.RedFailureFingerprint == "" {
		return fmt.Errorf("RED must retain a failing exit status and intended failure fingerprint")
	}
	if e.Green.ExitStatus != 0 || e.Refactor.ExitStatus != 0 {
		return fmt.Errorf("GREEN and REFACTOR must pass")
	}
	if e.Green.At.Before(e.ProductionChangedAt) || e.Refactor.At.Before(e.Green.At) {
		return fmt.Errorf("TDD evidence timestamps are out of order")
	}
	return nil
}

func recognizedExceptionReason(reason TDDExceptionReason) bool {
	switch reason {
	case TDDInapplicable, TDDUnavailableRunner, TDDUnwritableTests, TDDMissingBaseline, TDDNonDeterministicRun:
		return true
	default:
		return false
	}
}
