package qualification

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type ProvenanceClassification string

const (
	ProvenanceProven        ProvenanceClassification = "proven"
	ProvenancePartial       ProvenanceClassification = "partial"
	ProvenanceHistoricalGap ProvenanceClassification = "historical-gap"
	ProvenanceException     ProvenanceClassification = "exception"
)

type CycleEvidence struct {
	Command    string    `json:"command"`
	CWD        string    `json:"cwd"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	TreeRef    string    `json:"tree_ref"`
	ExitStatus int       `json:"exit_status"`
}

type HistoricalRecord struct {
	TaskID         string                   `json:"task_id"`
	SourceID       string                   `json:"source_id"`
	SourceDigest   string                   `json:"source_digest"`
	Classification ProvenanceClassification `json:"classification"`
	GapReason      string                   `json:"gap_reason,omitempty"`
	Red            *CycleEvidence           `json:"red,omitempty"`
	Green          *CycleEvidence           `json:"green,omitempty"`
	Refactor       *CycleEvidence           `json:"refactor,omitempty"`
}

type HistoricalProvenance struct {
	SchemaVersion string             `json:"schema_version"`
	ExpectedCount int                `json:"expected_count"`
	Records       []HistoricalRecord `json:"records"`
	CurrentGreen  *CycleEvidence     `json:"current_green"`
}

func DecodeHistoricalProvenance(reader io.Reader) (HistoricalProvenance, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var provenance HistoricalProvenance
	if err := decoder.Decode(&provenance); err != nil {
		return HistoricalProvenance{}, fmt.Errorf("decode historical provenance: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return HistoricalProvenance{}, fmt.Errorf("decode historical provenance: %w", err)
	}
	if err := provenance.Validate(); err != nil {
		return HistoricalProvenance{}, err
	}
	return provenance, nil
}

func (provenance HistoricalProvenance) Validate() error {
	if provenance.SchemaVersion != "1.0.0" {
		return fmt.Errorf("historical provenance schema_version must be 1.0.0")
	}
	if provenance.ExpectedCount < 0 {
		return fmt.Errorf("historical provenance expected_count cannot be negative")
	}
	seenTasks := make(map[string]struct{}, len(provenance.Records))
	seenDigests := make(map[string]struct{}, len(provenance.Records))
	for index, record := range provenance.Records {
		path := fmt.Sprintf("records[%d]", index)
		if strings.TrimSpace(record.TaskID) == "" || strings.TrimSpace(record.SourceID) == "" || !strings.HasPrefix(record.SourceDigest, "sha256:") {
			return fmt.Errorf("%s requires task_id, source_id, and sha256 source_digest", path)
		}
		if _, exists := seenTasks[record.TaskID]; exists {
			return fmt.Errorf("duplicate task ID %q", record.TaskID)
		}
		seenTasks[record.TaskID] = struct{}{}
		if _, exists := seenDigests[record.SourceDigest]; exists {
			return fmt.Errorf("duplicate source digest %q", record.SourceDigest)
		}
		seenDigests[record.SourceDigest] = struct{}{}

		phases := []struct {
			name     string
			evidence *CycleEvidence
		}{{"red", record.Red}, {"green", record.Green}, {"refactor", record.Refactor}}
		for _, phase := range phases {
			if phase.evidence != nil {
				if err := validateCycleEvidence(*phase.evidence); err != nil {
					return fmt.Errorf("%s.%s: %w", path, phase.name, err)
				}
			}
		}
		switch record.Classification {
		case ProvenanceProven:
			if record.Red == nil || record.Green == nil || record.Refactor == nil {
				return fmt.Errorf("%s proven classification requires RED, GREEN, and REFACTOR evidence", path)
			}
			if record.Red.ExitStatus == 0 || record.Green.ExitStatus != 0 || record.Refactor.ExitStatus != 0 ||
				record.Red.FinishedAt.After(record.Green.StartedAt) || record.Green.FinishedAt.After(record.Refactor.StartedAt) {
				return fmt.Errorf("%s proven classification has invalid RED/GREEN/REFACTOR order or exit status", path)
			}
		case ProvenancePartial, ProvenanceHistoricalGap, ProvenanceException:
			if strings.TrimSpace(record.GapReason) == "" {
				return fmt.Errorf("%s %s classification requires a truthful gap_reason", path, record.Classification)
			}
		default:
			return fmt.Errorf("%s has unknown classification %q", path, record.Classification)
		}
	}
	if len(provenance.Records) != provenance.ExpectedCount {
		return fmt.Errorf("historical provenance record count %d does not match expected_count %d", len(provenance.Records), provenance.ExpectedCount)
	}
	if provenance.CurrentGreen == nil {
		return fmt.Errorf("current_green must be recorded separately from historical evidence")
	}
	if err := validateCycleEvidence(*provenance.CurrentGreen); err != nil {
		return fmt.Errorf("current_green: %w", err)
	}
	if provenance.CurrentGreen.ExitStatus != 0 {
		return fmt.Errorf("current_green must have a zero exit status")
	}
	return nil
}

func validateCycleEvidence(evidence CycleEvidence) error {
	if strings.TrimSpace(evidence.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.Contains(evidence.Command, "./....") {
		return fmt.Errorf("invalid Go test command %q", evidence.Command)
	}
	if strings.TrimSpace(evidence.CWD) == "" {
		return fmt.Errorf("cwd is required")
	}
	if evidence.StartedAt.IsZero() || evidence.FinishedAt.IsZero() || evidence.FinishedAt.Before(evidence.StartedAt) {
		return fmt.Errorf("valid started_at and finished_at timestamp order is required")
	}
	if strings.TrimSpace(evidence.TreeRef) == "" {
		return fmt.Errorf("tree_ref is required")
	}
	return nil
}
