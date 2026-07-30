package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func corpusTerm(parts ...string) string { return strings.Join(parts, "") }

func TestScanActiveCorpusAcceptsProviderNeutralCorpusAndImmutableEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "internal/routes.go", "package routes\nconst route = \"route/v1/architecture\"\n")
	historical := "release notes preserve an old compatibility value\n" + corpusTerm("son", "net") + "\n"
	writeFixture(t, root, "CHANGELOG.md", historical)
	hash := sha256.Sum256([]byte(historical))
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	report, err := ScanActiveCorpus(CorpusScanRequest{
		Root: root, Now: now, Budget: 100,
		Allowlist: []CorpusAllowlistEntry{{
			Path: "CHANGELOG.md", SHA256: hex.EncodeToString(hash[:]), Purpose: CorpusPurposeHistorical,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != EvidencePassed || !report.Complete || report.FilesScanned != 2 || len(report.Violations) != 0 {
		t.Fatalf("unexpected clean report: %+v", report)
	}
	if report.InventoryDigest == "" || report.AllowlistDigest == "" || report.AllowedOccurrences != 1 {
		t.Fatalf("missing inventory/hash evidence: %+v", report)
	}
}

func TestScanActiveCorpusRejectsAliasesTablesAndInventedDefaults(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "active.md", "| phase | model |\n| architecture | "+corpusTerm("op", "us")+" |\n")
	writeFixture(t, root, "aliases.txt", strings.Join([]string{
		corpusTerm("son", "net"), corpusTerm("op", "us"), corpusTerm("ha", "iku"),
	}, "\n"))
	defaultTable := strings.Join([]string{"default", "Model", "Table"}, "")
	seededDefaultName := strings.Join([]string{"default", "Model"}, "")
	seededAssignmentName := strings.Join([]string{"phase", "Model", "Assignments"}, "")
	writeFixture(t, root, "defaults.go", "package defaults\nvar "+defaultTable+" = map[string]string{}\nconst "+seededDefaultName+" = \"route/v1/architecture\"\n")
	writeFixture(t, root, "assignments.go", "package assignments\nvar "+seededAssignmentName+" = map[string]string{\"architecture\": \"route/v1/architecture\"}\n")
	report, err := ScanActiveCorpus(CorpusScanRequest{Root: root, Now: time.Now().UTC(), Budget: 100})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != EvidenceFailed || report.Complete || len(report.Violations) < 2 {
		t.Fatalf("active policy violations were not rejected: %+v", report)
	}
	if !hasCorpusPattern(report.Violations, CorpusPatternAlias) || !hasCorpusPattern(report.Violations, CorpusPatternAssignmentTable) || !hasCorpusPattern(report.Violations, CorpusPatternInventedDefault) {
		t.Fatalf("missing violation classes: %+v", report.Violations)
	}
	if !hasCorpusViolation(report.Violations, "assignments.go", CorpusPatternAssignmentTable) {
		t.Fatalf("Go assignment authority was not classified: %+v", report.Violations)
	}
}

func TestScanActiveCorpusRequiresExactImmutableAllowlistAndNonInstallableProof(t *testing.T) {
	root := t.TempDir()
	content := corpusTerm("ha", "iku") + " historical\n"
	writeFixture(t, root, "fixtures/legacy.txt", content)
	now := time.Now().UTC()
	hash := sha256.Sum256([]byte(content))
	base := CorpusScanRequest{Root: root, Now: now, Budget: 100, Allowlist: []CorpusAllowlistEntry{{
		Path: "fixtures/legacy.txt", SHA256: hex.EncodeToString(hash[:]), Purpose: CorpusPurposeNegative,
	}}}
	if report, err := ScanActiveCorpus(base); err != nil || report.Status != EvidencePassed {
		t.Fatalf("exact allowlist rejected: report=%+v err=%v", report, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*CorpusScanRequest)
	}{
		{"changed hash", func(r *CorpusScanRequest) { r.Allowlist[0].SHA256 = strings.Repeat("0", 64) }},
		{"broad path", func(r *CorpusScanRequest) { r.Allowlist[0].Path = "fixtures/*.txt" }},
		{"install catalog", func(r *CorpusScanRequest) { r.InstallCatalog = []string{"fixtures/legacy.txt"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			request.Allowlist = append([]CorpusAllowlistEntry(nil), base.Allowlist...)
			tc.mutate(&request)
			report, err := ScanActiveCorpus(request)
			if err == nil && report.Status == EvidencePassed {
				t.Fatalf("unsafe allowlist passed: %+v", report)
			}
		})
	}
}

func hasCorpusPattern(findings []CorpusViolation, pattern CorpusPattern) bool {
	for _, finding := range findings {
		if finding.Pattern == pattern {
			return true
		}
	}
	return false
}

func hasCorpusViolation(findings []CorpusViolation, path string, pattern CorpusPattern) bool {
	for _, finding := range findings {
		if finding.Path == path && finding.Pattern == pattern {
			return true
		}
	}
	return false
}
