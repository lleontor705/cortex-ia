package qualification

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

const RaceCommand = "CGO_ENABLED=1 go test -race -count=1 ./..."

type Termination string

const (
	TerminationCompleted           Termination = "completed"
	TerminationCompilerUnavailable Termination = "compiler-unavailable"
	TerminationTimeout             Termination = "timeout"
	TerminationCancelled           Termination = "cancelled"
	TerminationFlakyInfrastructure Termination = "flaky-infrastructure"
)

type RaceAttempt struct {
	Number   int    `json:"number"`
	ExitCode int    `json:"exit_code"`
	Failure  string `json:"failure,omitempty"`
}

type RaceEvidence struct {
	GOOS            string        `json:"goos"`
	GOARCH          string        `json:"goarch"`
	CGOEnabled      bool          `json:"cgo_enabled"`
	Command         string        `json:"command"`
	GoVersion       string        `json:"go_version"`
	Compiler        string        `json:"compiler"`
	Termination     Termination   `json:"termination"`
	ExitCode        int           `json:"exit_code"`
	Attempts        []RaceAttempt `json:"attempts"`
	RetryBudget     int           `json:"retry_budget"`
	OriginalFailure string        `json:"original_failure,omitempty"`
}

type EvidenceDecision struct {
	Status          quality.OutcomeStatus `json:"status"`
	Blocking        bool                  `json:"blocking"`
	Reason          string                `json:"reason"`
	OriginalFailure string                `json:"original_failure,omitempty"`
}

func EvaluateRaceEvidence(evidence RaceEvidence) EvidenceDecision {
	decision := EvidenceDecision{Status: quality.OutcomeInconclusive, Blocking: true, OriginalFailure: evidence.OriginalFailure}
	if evidence.GOOS == "" && evidence.GOARCH == "" && evidence.Command == "" {
		decision.Reason = "missing race qualification evidence"
		return decision
	}
	if evidence.GOOS != "linux" || evidence.GOARCH != "amd64" {
		decision.Reason = "race qualification requires linux/amd64 evidence"
		return decision
	}
	if !evidence.CGOEnabled {
		decision.Reason = "race qualification requires CGO enabled"
		return decision
	}
	if evidence.Command != RaceCommand {
		decision.Reason = fmt.Sprintf("race qualification command must be %q", RaceCommand)
		return decision
	}
	if strings.TrimSpace(evidence.GoVersion) == "" || strings.TrimSpace(evidence.Compiler) == "" {
		decision.Reason = "race qualification requires Go toolchain and compiler facts"
		return decision
	}
	if evidence.RetryBudget < 0 || len(evidence.Attempts) == 0 || len(evidence.Attempts) > evidence.RetryBudget+1 {
		decision.Reason = "race qualification retry evidence exceeds or omits the declared retry budget"
		return decision
	}
	for index, attempt := range evidence.Attempts {
		if attempt.Number != index+1 {
			decision.Reason = "race qualification attempts must be ordered and contiguous"
			return decision
		}
	}
	switch evidence.Termination {
	case TerminationCompilerUnavailable:
		decision.Reason = "race qualification compiler unavailable"
		return decision
	case TerminationTimeout:
		decision.Reason = "race qualification timeout"
		return decision
	case TerminationCancelled:
		decision.Reason = "race qualification cancelled"
		return decision
	case TerminationFlakyInfrastructure:
		decision.Reason = "race qualification flaky infrastructure"
		return decision
	case TerminationCompleted:
	default:
		decision.Reason = "missing or unknown race qualification termination"
		return decision
	}

	firstFailure := ""
	passedAfterFailure := false
	for _, attempt := range evidence.Attempts {
		if attempt.ExitCode != 0 {
			if firstFailure == "" {
				firstFailure = attempt.Failure
			}
			continue
		}
		if firstFailure != "" {
			passedAfterFailure = true
		}
	}
	if firstFailure != "" || evidence.OriginalFailure != "" {
		if decision.OriginalFailure == "" {
			decision.OriginalFailure = firstFailure
		}
		if passedAfterFailure {
			decision.Reason = "race qualification passed only after retry; original failure preserved"
			return decision
		}
	}
	if evidence.ExitCode != 0 {
		decision.Status = quality.OutcomeFail
		decision.Reason = "race detector reported a failure"
		return decision
	}
	decision.Status = quality.OutcomePass
	decision.Reason = "pinned Linux/amd64 CGO race qualification passed"
	return decision
}

type EvidenceKind string

const (
	EvidenceMutation EvidenceKind = "mutation"
	EvidenceRuntime  EvidenceKind = "runtime"
)

type Enforcement string

const (
	EnforcementReportOnly Enforcement = "report-only"
	EnforcementBlocking   Enforcement = "blocking"
)

type EvidenceBudget struct {
	WallTimeSeconds int `json:"wall_time_seconds"`
	Retries         int `json:"retries"`
	Cases           int `json:"cases"`
}

type PolicyEvidence struct {
	Kind               EvidenceKind   `json:"kind"`
	Enforcement        Enforcement    `json:"enforcement"`
	Termination        Termination    `json:"termination"`
	Numerator          int            `json:"numerator"`
	Denominator        int            `json:"denominator"`
	Exclusions         []string       `json:"exclusions"`
	Budget             EvidenceBudget `json:"budget"`
	Tool               string         `json:"tool"`
	EvidenceRefs       []string       `json:"evidence_refs"`
	CredentialMaterial string         `json:"-"`
}

func EvaluatePolicyEvidence(evidence PolicyEvidence) EvidenceDecision {
	decision := EvidenceDecision{Status: quality.OutcomeInconclusive, Blocking: evidence.Enforcement == EnforcementBlocking}
	if evidence.Kind != EvidenceMutation && evidence.Kind != EvidenceRuntime {
		decision.Reason = "missing or unknown qualification evidence kind"
		return decision
	}
	if evidence.Enforcement != EnforcementReportOnly && evidence.Enforcement != EnforcementBlocking {
		decision.Reason = "missing or unknown qualification enforcement"
		return decision
	}
	if evidence.CredentialMaterial != "" {
		decision.Status = quality.OutcomeFail
		decision.Blocking = true
		decision.Reason = "qualification evidence contains credential material"
		return decision
	}
	if evidence.Termination != TerminationCompleted {
		decision.Reason = "qualification evidence is inconclusive: " + string(evidence.Termination)
		return decision
	}
	if evidence.Denominator <= 0 || evidence.Numerator < 0 || evidence.Numerator > evidence.Denominator ||
		evidence.Budget.WallTimeSeconds <= 0 || evidence.Budget.Retries < 0 || evidence.Budget.Cases <= 0 ||
		strings.TrimSpace(evidence.Tool) == "" || len(evidence.EvidenceRefs) == 0 || evidence.Exclusions == nil {
		decision.Reason = "qualification evidence is incomplete or has invalid metrics, exclusions, budget, or tool provenance"
		return decision
	}
	decision.Status = quality.OutcomePass
	decision.Reason = string(evidence.Kind) + " evidence completed under its declared " + string(evidence.Enforcement) + " policy"
	return decision
}
