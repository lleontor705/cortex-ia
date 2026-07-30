package conformance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type AllowanceClassification string

const (
	AllowanceTombstone       AllowanceClassification = "tombstone-decode"
	AllowanceMigration       AllowanceClassification = "migration"
	AllowanceRollback        AllowanceClassification = "rollback"
	AllowanceHistorical      AllowanceClassification = "historical"
	AllowanceOperatorCleanup AllowanceClassification = "operator-cleanup"
)

type LegacyAllowance struct {
	Path           string
	Pattern        string
	Reason         string
	Owner          string
	Classification AllowanceClassification
	ReviewBy       time.Time
}

func ValidateLegacyAllowances(allowances []LegacyAllowance, now time.Time) error {
	seen := make(map[string]struct{}, len(allowances))
	for index, allowance := range allowances {
		path := filepath.ToSlash(filepath.Clean(allowance.Path))
		if path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "../") {
			return fmt.Errorf("legacy allowance %d has invalid repository path %q", index, allowance.Path)
		}
		if strings.TrimSpace(allowance.Pattern) == "" || strings.TrimSpace(allowance.Reason) == "" || strings.TrimSpace(allowance.Owner) == "" {
			return fmt.Errorf("legacy allowance %q requires pattern, reason, and owner", path)
		}
		switch allowance.Classification {
		case AllowanceTombstone, AllowanceMigration, AllowanceRollback, AllowanceHistorical, AllowanceOperatorCleanup:
		default:
			return fmt.Errorf("legacy allowance %q has unsupported classification %q", path, allowance.Classification)
		}
		if allowance.ReviewBy.IsZero() || !allowance.ReviewBy.After(now) {
			return fmt.Errorf("legacy allowance %q is expired or lacks review_by", path)
		}
		key := path + "\x00" + allowance.Pattern
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate legacy allowance for %q pattern %q", path, allowance.Pattern)
		}
		seen[key] = struct{}{}
	}
	return nil
}
