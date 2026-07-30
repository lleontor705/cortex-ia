package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// MandatoryQualityCommands is the complete command surface owned by the N3.3 gate.
var MandatoryQualityCommands = []string{"go test ./...", "go build ./...", "go vet ./...", "golangci-lint", "gofmt", "git diff --check"}

type CommandEvidence struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Environment map[string]string `json:"environment"`
	Revision    string            `json:"revision"`
	ExitCode    int               `json:"exit_code"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	OutputHash  string            `json:"output_hash"`
}

type StaticFinding struct {
	Rule       string `json:"rule"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Introduced bool   `json:"introduced"`
}

type RaceCapabilityEvidence struct {
	GOOS       string    `json:"goos"`
	GOARCH     string    `json:"goarch"`
	CGOEnabled bool      `json:"cgo_enabled"`
	Compiler   string    `json:"compiler"`
	Command    string    `json:"command"`
	Revision   string    `json:"revision"`
	ExitCode   int       `json:"exit_code"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	OutputHash string    `json:"output_hash"`
}

type RaceStatus string

const (
	RacePass         RaceStatus = "pass"
	RaceFail         RaceStatus = "fail"
	RaceInconclusive RaceStatus = "inconclusive"
)

type RaceDecision struct {
	Status   RaceStatus `json:"status"`
	Blocking bool       `json:"blocking"`
	Reason   string     `json:"reason"`
}

type QualityGateInput struct {
	Revision       string                 `json:"revision"`
	Environment    map[string]string      `json:"environment"`
	Commands       []CommandEvidence      `json:"commands"`
	StaticFindings []StaticFinding        `json:"static_findings"`
	Race           RaceCapabilityEvidence `json:"race"`
	RaceMandatory  bool                   `json:"race_mandatory"`
}

type QualityGateEvidence struct {
	Passed              bool                   `json:"passed"`
	Revision            string                 `json:"revision"`
	Environment         map[string]string      `json:"environment"`
	Commands            []CommandEvidence      `json:"commands"`
	RaceEvidence        RaceCapabilityEvidence `json:"race_evidence"`
	Race                RaceDecision           `json:"race"`
	StaticFindings      []StaticFinding        `json:"introduced_findings"`
	PreexistingFindings []StaticFinding        `json:"preexisting_findings"`
	Blockers            []string               `json:"blockers"`
	GeneratedAt         time.Time              `json:"generated_at"`
	Fingerprint         string                 `json:"fingerprint"`
}

// EvaluateRaceCapability is deliberately strict: an unqualified or unavailable
// race run is evidence of inability, never evidence of success.
func EvaluateRaceCapability(evidence RaceCapabilityEvidence, mandatory bool) RaceDecision {
	decision := RaceDecision{Status: RaceInconclusive, Blocking: mandatory}
	if evidence.GOOS == "" || evidence.GOARCH == "" || !evidence.CGOEnabled || strings.TrimSpace(evidence.Compiler) == "" {
		decision.Reason = "race capability unavailable: CGO and compiler are required"
		return decision
	}
	if evidence.GOARCH != "amd64" || (evidence.GOOS != "linux" && evidence.GOOS != "windows") {
		decision.Reason = "race capability requires qualified linux/amd64 or windows/amd64 execution"
		return decision
	}
	if evidence.Command != "CGO_ENABLED=1 go test -race -count=1 ./..." {
		decision.Reason = "race capability command is not the qualified race command"
		return decision
	}
	if strings.TrimSpace(evidence.Revision) == "" || strings.TrimSpace(evidence.OutputHash) == "" || evidence.StartedAt.IsZero() || evidence.FinishedAt.IsZero() || !evidence.FinishedAt.After(evidence.StartedAt) {
		decision.Reason = "race capability execution evidence is incomplete"
		return decision
	}
	if evidence.ExitCode != 0 {
		decision.Status = RaceFail
		decision.Blocking = true
		decision.Reason = "race command failed"
		return decision
	}
	decision.Status = RacePass
	decision.Blocking = false
	decision.Reason = "qualified race command succeeded"
	return decision
}

// AggregateQualityEvidence normalizes all command, static-analysis, and race
// facts into a deterministic release artifact. It never turns missing evidence
// into a pass and keeps pre-existing findings separate from introduced defects.
func AggregateQualityEvidence(input QualityGateInput) (QualityGateEvidence, error) {
	if strings.TrimSpace(input.Revision) == "" {
		return QualityGateEvidence{}, fmt.Errorf("quality gate revision is required")
	}
	if len(input.Environment) == 0 {
		return QualityGateEvidence{}, fmt.Errorf("quality gate environment is required")
	}
	report := QualityGateEvidence{Revision: input.Revision, Environment: cloneEnvironment(input.Environment), GeneratedAt: time.Now().UTC()}
	report.Commands = append([]CommandEvidence(nil), input.Commands...)
	sort.Slice(report.Commands, func(i, j int) bool { return report.Commands[i].Name < report.Commands[j].Name })
	seen := make(map[string]bool, len(report.Commands))
	for index, command := range report.Commands {
		if command.Environment == nil {
			report.Commands[index].Environment = cloneEnvironment(input.Environment)
		}
		seen[command.Name] = true
		if command.Command == "" || command.Revision == "" || command.OutputHash == "" || command.StartedAt.IsZero() || command.FinishedAt.IsZero() {
			report.Blockers = append(report.Blockers, "missing mandatory command attribution: "+command.Name)
		}
		if command.Revision != input.Revision {
			report.Blockers = append(report.Blockers, "stale command revision: "+command.Name)
		}
		if !command.FinishedAt.After(command.StartedAt) {
			report.Blockers = append(report.Blockers, "invalid command time window: "+command.Name)
		}
		if command.ExitCode != 0 {
			report.Blockers = append(report.Blockers, "command failed: "+command.Name)
		}
	}
	for _, name := range MandatoryQualityCommands {
		if !seen[name] {
			report.Blockers = append(report.Blockers, "missing mandatory command: "+name)
		}
	}
	for _, finding := range input.StaticFindings {
		if finding.Introduced {
			report.StaticFindings = append(report.StaticFindings, finding)
			report.Blockers = append(report.Blockers, "introduced static-analysis finding: "+finding.Rule)
		} else {
			report.PreexistingFindings = append(report.PreexistingFindings, finding)
		}
	}
	report.RaceEvidence = input.Race
	report.Race = EvaluateRaceCapability(input.Race, input.RaceMandatory)
	if input.Race.Revision != input.Revision {
		report.Blockers = append(report.Blockers, "stale race revision")
	}
	if !input.Race.StartedAt.IsZero() && !input.Race.FinishedAt.After(input.Race.StartedAt) {
		report.Blockers = append(report.Blockers, "invalid race time window")
	}
	if report.Race.Blocking {
		report.Blockers = append(report.Blockers, "race gate blocked: "+report.Race.Reason)
	}
	slices.Sort(report.Blockers)
	report.Passed = len(report.Blockers) == 0
	report.Fingerprint = fingerprintQualityGate(report)
	return report, nil
}

func cloneEnvironment(environment map[string]string) map[string]string {
	clone := make(map[string]string, len(environment))
	for key, value := range environment {
		clone[key] = value
	}
	return clone
}

func fingerprintQualityGate(report QualityGateEvidence) string {
	copy := report
	copy.Fingerprint = ""
	copy.GeneratedAt = time.Time{}
	data, _ := json.Marshal(copy)
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}
