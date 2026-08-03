package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type EvidenceStatus string

const (
	EvidencePassed       EvidenceStatus = "passed"
	EvidenceFailed       EvidenceStatus = "failed"
	EvidenceInconclusive EvidenceStatus = "inconclusive"
)

type CollectorRequest struct {
	Root            string
	Allowances      []LegacyAllowance
	CorpusAllowlist []CorpusAllowlistEntry
	InstallCatalog  []string
	Now             time.Time
	Budget          int
}

type SourceViolation struct {
	Path    string
	Pattern string
	Count   int
}

type RepositoryEvidence struct {
	Command            string
	CWD                string
	Tree               string
	StartedAt          time.Time
	FinishedAt         time.Time
	Status             EvidenceStatus
	Digest             string
	Budget             int
	FilesScanned       int
	Occurrences        int
	AllowedOccurrences int
	Complete           bool
	Violations         []SourceViolation
	ActiveCorpus       CorpusScanReport
}

var forbiddenCurrentPatterns = []string{"agent-mailbox", "team-lead"}

func CollectRepository(request CollectorRequest) (RepositoryEvidence, error) {
	root, err := filepath.Abs(strings.TrimSpace(request.Root))
	if err != nil || strings.TrimSpace(request.Root) == "" {
		return RepositoryEvidence{}, fmt.Errorf("repository root is required")
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	}
	if request.Budget <= 0 {
		return RepositoryEvidence{}, fmt.Errorf("positive repository scan budget is required")
	}
	if err := ValidateLegacyAllowances(request.Allowances, request.Now); err != nil {
		return RepositoryEvidence{}, err
	}
	allowances := make(map[string]map[string]LegacyAllowance)
	for _, allowance := range request.Allowances {
		path := filepath.ToSlash(filepath.Clean(allowance.Path))
		if allowances[path] == nil {
			allowances[path] = make(map[string]LegacyAllowance)
		}
		allowances[path][allowance.Pattern] = allowance
	}
	evidence := RepositoryEvidence{
		Command: "repository-source-scan", CWD: root, StartedAt: request.Now, FinishedAt: request.Now,
		Budget: request.Budget, Status: EvidencePassed, Complete: true,
	}
	records := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if evidence.FilesScanned >= request.Budget {
			evidence.Complete = false
			evidence.Status = EvidenceInconclusive
			return fs.SkipAll
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.IndexByte(string(content), 0) >= 0 {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		evidence.FilesScanned++
		hash := sha256.Sum256(content)
		records = append(records, relative+":"+hex.EncodeToString(hash[:]))
		text := string(content)
		for _, pattern := range forbiddenCurrentPatterns {
			count := strings.Count(text, pattern)
			if count == 0 {
				continue
			}
			evidence.Occurrences += count
			if _, allowed := allowances[relative][pattern]; allowed {
				evidence.AllowedOccurrences += count
				continue
			}
			evidence.Violations = append(evidence.Violations, SourceViolation{Path: relative, Pattern: pattern, Count: count})
		}
		return nil
	})
	if err != nil {
		return RepositoryEvidence{}, fmt.Errorf("collect repository conformance: %w", err)
	}
	active, activeErr := ScanActiveCorpus(CorpusScanRequest{
		Root: root, Allowlist: request.CorpusAllowlist, InstallCatalog: request.InstallCatalog,
		Now: request.Now, Budget: request.Budget,
	})
	if activeErr != nil {
		return RepositoryEvidence{}, activeErr
	}
	evidence.ActiveCorpus = active
	evidence.Occurrences += active.Occurrences
	evidence.AllowedOccurrences += active.AllowedOccurrences
	for _, violation := range active.Violations {
		evidence.Violations = append(evidence.Violations, SourceViolation{Path: violation.Path, Pattern: string(violation.Pattern), Count: violation.Count})
	}
	switch active.Status {
	case EvidenceInconclusive:
		evidence.Status = EvidenceInconclusive
		evidence.Complete = false
	case EvidenceFailed:
		evidence.Status = EvidenceFailed
		evidence.Complete = false
	}
	slices.Sort(records)
	tree := sha256.Sum256([]byte(strings.Join(records, "\n")))
	evidence.Tree = hex.EncodeToString(tree[:])
	slices.SortFunc(evidence.Violations, func(left, right SourceViolation) int {
		if difference := strings.Compare(left.Path, right.Path); difference != 0 {
			return difference
		}
		return strings.Compare(left.Pattern, right.Pattern)
	})
	if evidence.Complete && len(evidence.Violations) != 0 {
		evidence.Status = EvidenceFailed
	}
	digestInput := evidence
	digestInput.StartedAt = time.Time{}
	digestInput.FinishedAt = time.Time{}
	digestInput.Digest = ""
	encoded, err := json.Marshal(digestInput)
	if err != nil {
		return RepositoryEvidence{}, err
	}
	digest := sha256.Sum256(encoded)
	evidence.Digest = hex.EncodeToString(digest[:])
	return evidence, nil
}
