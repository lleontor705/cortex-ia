package capability

import (
	"errors"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestCatalogValidateRejectsInvalidFactsWithRemediation(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		mutate   func(*Catalog)
		wantPath string
	}{
		{
			name: "missing provenance",
			mutate: func(catalog *Catalog) {
				catalog.Facts[0].EvidenceRef = ""
			},
			wantPath: "$.facts[0].evidence_ref",
		},
		{
			name: "invalid cardinality",
			mutate: func(catalog *Catalog) {
				catalog.Facts[0].Cardinality = Cardinality("sometimes")
			},
			wantPath: "$.facts[0].cardinality",
		},
		{
			name: "expired current fact",
			mutate: func(catalog *Catalog) {
				catalog.Facts[0].FreshUntil = now.Add(-time.Second)
			},
			wantPath: "$.facts[0].fresh_until",
		},
		{
			name: "incompatible runtime interval",
			mutate: func(catalog *Catalog) {
				catalog.Facts[0].RuntimeVersions.MaximumTested = ir.MustParseVersion("2.0.0")
			},
			wantPath: "$.facts[0].runtime_versions",
		},
		{
			name: "probe evidence lacks executable record",
			mutate: func(catalog *Catalog) {
				catalog.Facts[0].EvidenceClass = EvidenceExecutableProbe
				catalog.Facts[0].Probe = nil
			},
			wantPath: "$.facts[0].probe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validCatalog(now)
			tt.mutate(&catalog)

			err := catalog.Validate(now)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error type = %T, want *ValidationError", err)
			}
			if validationErr.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", validationErr.Path, tt.wantPath)
			}
			if validationErr.Remediation == "" {
				t.Error("remediation is empty")
			}
		})
	}
}

func TestCatalogValidateRejectsContradictoryOverlappingFacts(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	catalog := validCatalog(now)
	contradiction := catalog.Facts[0]
	contradiction.RuntimeVersions = versionRange("1.4.0", "1.8.0")
	contradiction.Cardinality = CardinalityMany
	catalog.Facts = append(catalog.Facts, contradiction)

	err := catalog.Validate(now)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if validationErr.Code != ErrorContradictoryOverlap {
		t.Errorf("code = %q, want %q", validationErr.Code, ErrorContradictoryOverlap)
	}
	if validationErr.Path != "$.facts[1].runtime_versions" {
		t.Errorf("path = %q", validationErr.Path)
	}
}

func TestCatalogValidateRuntimeVersionIntervals(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		minimum string
		maximum string
		wantErr bool
	}{
		{name: "ordered same-major major-zero interval", minimum: "0.1.0", maximum: "0.9.3"},
		{name: "default zero interval", minimum: "0.0.0", maximum: "0.0.0", wantErr: true},
		{name: "reversed major-zero interval", minimum: "0.9.0", maximum: "0.8.9", wantErr: true},
		{name: "cross-major interval", minimum: "0.9.0", maximum: "1.0.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validCatalog(now)
			catalog.Facts[0].RuntimeVersions = versionRange(tt.minimum, tt.maximum)

			err := catalog.Validate(now)
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestCatalogValidateAcceptsCompatibleNonOverlappingEvidence(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	catalog := validCatalog(now)
	newer := catalog.Facts[0]
	newer.RuntimeVersions = versionRange("1.6.0", "1.9.0")
	newer.Cardinality = CardinalityMany
	catalog.Facts[0].RuntimeVersions = versionRange("1.0.0", "1.5.0")
	catalog.Facts = append(catalog.Facts, newer)

	if err := catalog.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDecodeCatalogReportsOptionalExtensionDegradation(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	document := `{
		"schema_version":"1.0.0",
		"version":"1.0.0",
		"facts":[],
		"vendor/telemetry":{"optional":true,"value":{"sample":1}}
	}`

	result, err := DecodeCatalog([]byte(document), now)
	if err != nil {
		t.Fatalf("DecodeCatalog() error = %v", err)
	}
	if len(result.Degradations) != 1 {
		t.Fatalf("degradations = %+v, want one", result.Degradations)
	}
	if result.Degradations[0].SemanticID != "vendor/telemetry" || result.Degradations[0].Reason == "" {
		t.Errorf("degradation = %+v", result.Degradations[0])
	}
}

func TestDecodeCatalogRejectsUnsupportedSchemaVersion(t *testing.T) {
	result, err := DecodeCatalog([]byte(`{"schema_version":"2.0.0","version":"1.0.0","facts":[]}`), time.Now())
	if err == nil {
		t.Fatalf("DecodeCatalog() = %+v, nil error", result)
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Path != "$.schema_version" || validationErr.Remediation == "" {
		t.Errorf("diagnostic = %+v", validationErr)
	}
}

func validCatalog(now time.Time) Catalog {
	return Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts: []Fact{{
			ID:              "delegation/direct-child",
			Mode:            CapabilityAvailable,
			Cardinality:     CardinalityOne,
			Target:          "claude",
			RuntimeID:       "claude-code",
			AdapterID:       "cortex-ia/claude",
			RuntimeVersions: versionRange("1.0.0", "1.5.0"),
			EvidenceClass:   EvidenceRuntimeObserved,
			EvidenceRef:     "qualification/claude/direct-child/2026-07-20",
			ObservedAt:      now.Add(-24 * time.Hour),
			FreshUntil:      now.Add(30 * 24 * time.Hour),
			Confidence:      0.95,
			Current:         true,
			Enforcement:     EnforcementRuntime,
		}},
	}
}

func versionRange(minimum, maximum string) ir.VersionRange {
	return ir.VersionRange{
		Minimum:       ir.MustParseVersion(minimum),
		MaximumTested: ir.MustParseVersion(maximum),
	}
}
