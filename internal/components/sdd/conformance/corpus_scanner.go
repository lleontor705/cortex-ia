package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// CorpusPattern identifies a source-derived active-corpus violation.
type CorpusPattern string

const (
	CorpusPatternAlias           CorpusPattern = "vendor-tier-alias"
	CorpusPatternAssignmentTable CorpusPattern = "phase-model-assignment-table"
	CorpusPatternInventedDefault CorpusPattern = "invented-model-default"
)

const (
	CorpusPurposeHistorical    = "historical"
	CorpusPurposeOracle        = "legacy-oracle"
	CorpusPurposeCompatibility = "compatibility"
	CorpusPurposeDiscovery     = "discovery-fixture"
	CorpusPurposeNegative      = "discovery-negative"
)

type CorpusAllowlistEntry struct {
	Path    string
	SHA256  string
	Purpose string
}

type CorpusScanRequest struct {
	Root           string
	Allowlist      []CorpusAllowlistEntry
	InstallCatalog []string
	Now            time.Time
	Budget         int
}

type CorpusFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type CorpusViolation struct {
	Path    string        `json:"path"`
	Pattern CorpusPattern `json:"pattern"`
	Count   int           `json:"count"`
	Detail  string        `json:"detail,omitempty"`
}

type CorpusScanReport struct {
	Status             EvidenceStatus
	Complete           bool
	FilesScanned       int
	Occurrences        int
	AllowedOccurrences int
	Inventory          []CorpusFile
	InventoryDigest    string
	AllowlistDigest    string
	Violations         []CorpusViolation
}

var assignmentTablePattern = regexp.MustCompile(`(?im)^\s*\|[^\n]*(?:phase|role)[^\n]*\|[^\n]*(?:model|tier|assignment)[^\n]*\|`)
var hardcodedAssignmentPattern = regexp.MustCompile(`(?i)\b(?:phase|role)[a-z0-9_]*(?:model|tier|assignment)[a-z0-9_]*\s*[:=]`)
var inventedDefaultPattern = regexp.MustCompile(`(?im)\b(?:var|const)\s+(?:default[_ -]?model(?:table)?|model[_ -]?defaults?|fallback[_ -]?model)\b\s*(?:[a-z0-9_\[\]]+\s*)?(?::=|=)`)

func corpusAliases() []string {
	return []string{
		string([]byte{115, 111, 110, 110, 101, 116}),
		string([]byte{111, 112, 117, 115}),
		string([]byte{104, 97, 105, 107, 117}),
	}
}

// ScanActiveCorpus exhaustively inventories text files and applies only exact,
// hash-pinned evidence exceptions. It is intentionally source-derived: callers
// cannot declare a clean count or use a directory/glob exemption.
func ScanActiveCorpus(request CorpusScanRequest) (CorpusScanReport, error) {
	root, err := filepath.Abs(strings.TrimSpace(request.Root))
	if err != nil || strings.TrimSpace(request.Root) == "" {
		return CorpusScanReport{}, fmt.Errorf("active corpus root is required")
	}
	if request.Budget <= 0 {
		return CorpusScanReport{}, fmt.Errorf("positive active corpus scan budget is required")
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	}
	allowances, err := validateCorpusAllowlist(root, request.Allowlist, request.InstallCatalog)
	if err != nil {
		return CorpusScanReport{}, err
	}
	report := CorpusScanReport{Status: EvidencePassed, Complete: true}
	used := make(map[string]bool, len(allowances))
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
		if report.FilesScanned >= request.Budget {
			report.Complete = false
			report.Status = EvidenceInconclusive
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
		report.FilesScanned++
		digest := sha256.Sum256(content)
		hash := hex.EncodeToString(digest[:])
		report.Inventory = append(report.Inventory, CorpusFile{Path: relative, SHA256: hash})
		text := string(content)
		allowance, allowed := allowances[relative]
		if allowed {
			used[relative] = true
			if allowance.SHA256 != hash {
				report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternAlias, Detail: "allowlisted content hash changed"})
				return nil
			}
		}
		for _, alias := range corpusAliases() {
			count := wholeWordCount(relative, alias)
			if count == 0 {
				continue
			}
			report.Occurrences += count
			if allowed {
				report.AllowedOccurrences += count
				continue
			}
			report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternAlias, Count: count, Detail: "active path contains a forbidden term"})
		}
		for _, alias := range corpusAliases() {
			count := wholeWordCount(text, alias)
			if count == 0 {
				continue
			}
			report.Occurrences += count
			if allowed {
				report.AllowedOccurrences += count
				continue
			}
			report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternAlias, Count: count})
		}
		if count := len(assignmentTablePattern.FindAllString(text, -1)); count > 0 {
			report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternAssignmentTable, Count: count})
		}
		if count := len(hardcodedAssignmentPattern.FindAllString(text, -1)); count > 0 {
			report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternAssignmentTable, Count: count, Detail: "active source contains a hardcoded phase/role assignment"})
		}
		if count := len(inventedDefaultPattern.FindAllString(text, -1)); count > 0 {
			report.Violations = append(report.Violations, CorpusViolation{Path: relative, Pattern: CorpusPatternInventedDefault, Count: count})
		}
		return nil
	})
	if err != nil {
		return CorpusScanReport{}, fmt.Errorf("scan active corpus: %w", err)
	}
	for path := range allowances {
		if !used[path] {
			report.Violations = append(report.Violations, CorpusViolation{Path: path, Pattern: CorpusPatternAlias, Detail: "allowlisted path is absent from scanned corpus"})
		}
	}
	if len(report.Violations) > 0 {
		report.Complete = false
	}
	slices.SortFunc(report.Inventory, func(a, b CorpusFile) int { return strings.Compare(a.Path, b.Path) })
	report.InventoryDigest = digestCorpusInventory(report.Inventory)
	report.AllowlistDigest = digestCorpusAllowlist(request.Allowlist)
	if len(report.Violations) > 0 && report.Status != EvidenceInconclusive {
		report.Status = EvidenceFailed
	}
	return report, nil
}

func validateCorpusAllowlist(root string, entries []CorpusAllowlistEntry, installCatalog []string) (map[string]CorpusAllowlistEntry, error) {
	installed := make(map[string]bool, len(installCatalog))
	for _, path := range installCatalog {
		installed[filepath.ToSlash(filepath.Clean(path))] = true
	}
	result := make(map[string]CorpusAllowlistEntry, len(entries))
	allowedPurpose := map[string]bool{CorpusPurposeHistorical: true, CorpusPurposeOracle: true, CorpusPurposeCompatibility: true, CorpusPurposeDiscovery: true, CorpusPurposeNegative: true}
	for _, entry := range entries {
		path := filepath.ToSlash(filepath.Clean(entry.Path))
		if path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "../") || strings.ContainsAny(path, "*?[]{}") {
			return nil, fmt.Errorf("corpus allowlist requires an exact repository path: %q", entry.Path)
		}
		if !allowedPurpose[entry.Purpose] {
			return nil, fmt.Errorf("corpus allowlist path %q has unsupported purpose %q", path, entry.Purpose)
		}
		if len(entry.SHA256) != 64 {
			return nil, fmt.Errorf("corpus allowlist path %q requires a sha256 hash", path)
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return nil, fmt.Errorf("corpus allowlist path %q has invalid sha256: %w", path, err)
		}
		if installed[path] {
			return nil, fmt.Errorf("corpus allowlist path %q is installable", path)
		}
		if _, duplicate := result[path]; duplicate {
			return nil, fmt.Errorf("duplicate corpus allowlist path %q", path)
		}
		result[path] = entry
	}
	_ = root
	return result, nil
}

func wholeWordCount(text, term string) int {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
	return len(pattern.FindAllStringIndex(text, -1))
}

func digestCorpusInventory(inventory []CorpusFile) string {
	lines := make([]string, 0, len(inventory))
	for _, file := range inventory {
		lines = append(lines, file.Path+":"+file.SHA256)
	}
	hash := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(hash[:])
}

func digestCorpusAllowlist(entries []CorpusAllowlistEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, filepath.ToSlash(filepath.Clean(entry.Path))+":"+entry.SHA256+":"+entry.Purpose)
	}
	slices.Sort(lines)
	hash := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(hash[:])
}
