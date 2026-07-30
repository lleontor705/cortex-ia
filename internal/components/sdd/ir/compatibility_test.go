package ir

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type compatibilityFixture struct {
	Name          string       `json:"name"`
	Document      string       `json:"document"`
	WantVersion   Version      `json:"want_version"`
	WantSemantic  SemanticID   `json:"want_semantic_id"`
	WantDegraded  []SemanticID `json:"want_degraded"`
	WantErrorCode ErrorCode    `json:"want_error_code"`
	WantPath      string       `json:"want_path"`
}

func TestDecodeWorkflowCompatibilityFixtures(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "compatibility.json"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var fixtures []compatibilityFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("decode fixtures: %v", err)
	}

	for _, tt := range fixtures {
		t.Run(tt.Name, func(t *testing.T) {
			result, err := DecodeWorkflow([]byte(tt.Document))
			if tt.WantErrorCode == "" {
				if err != nil {
					t.Fatalf("DecodeWorkflow() error = %v", err)
				}
				if result.Workflow.SchemaVersion != tt.WantVersion {
					t.Errorf("schema version = %s, want %s", result.Workflow.SchemaVersion, tt.WantVersion)
				}
				if result.Workflow.ID != tt.WantSemantic {
					t.Errorf("semantic ID = %q, want %q", result.Workflow.ID, tt.WantSemantic)
				}
				if len(result.Degradations) != len(tt.WantDegraded) {
					t.Fatalf("degradations = %+v, want %v", result.Degradations, tt.WantDegraded)
				}
				for i, degradation := range result.Degradations {
					if degradation.SemanticID != tt.WantDegraded[i] || degradation.Reason == "" {
						t.Errorf("degradation[%d] = %+v, want semantic ID %q and a reason", i, degradation, tt.WantDegraded[i])
					}
				}
				return
			}

			if err == nil {
				t.Fatal("DecodeWorkflow() error = nil, want compatibility error")
			}
			var compatibilityErr *CompatibilityError
			if !errors.As(err, &compatibilityErr) {
				t.Fatalf("error type = %T, want *CompatibilityError", err)
			}
			if compatibilityErr.Code != tt.WantErrorCode {
				t.Errorf("error code = %q, want %q", compatibilityErr.Code, tt.WantErrorCode)
			}
			if compatibilityErr.Path != tt.WantPath {
				t.Errorf("error path = %q, want %q", compatibilityErr.Path, tt.WantPath)
			}
			for label, value := range map[string]string{
				"schema":      compatibilityErr.Schema,
				"observed":    compatibilityErr.Observed,
				"supported":   compatibilityErr.Supported,
				"remediation": compatibilityErr.Remediation,
			} {
				if strings.TrimSpace(value) == "" {
					t.Errorf("%s diagnostic is empty: %+v", label, compatibilityErr)
				}
			}
		})
	}
}

func TestParseVersionRejectsInvalidSemanticVersions(t *testing.T) {
	for _, input := range []string{"", "1", "1.2", "v1.2.3", "1.2.3.4", "1.-2.3"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseVersion(input); err == nil {
				t.Fatalf("ParseVersion(%q) error = nil", input)
			}
		})
	}
}
