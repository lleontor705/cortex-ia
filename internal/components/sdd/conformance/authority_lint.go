package conformance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AuthorityInventory is a narrow, fail-closed scan for copied executable
// policy. Generated references must point at a policy key rather than restate
// numeric retry/confidence rules.
type AuthorityInventory struct {
	Complete          bool
	HasDuplicateOwner bool
	FilesScanned      int
	Violations        []string
}

var copiedPolicyPattern = regexp.MustCompile(`(?i)(retry\s+ceiling\s*:\s*\d+|confidence\s+(?:threshold|table)\s*:)`)

// ScanAuthorityInventory scans a source or generated bundle and rejects
// hand-maintained policy literals outside executable Go authority.
func ScanAuthorityInventory(root string) (AuthorityInventory, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return AuthorityInventory{}, fmt.Errorf("authority inventory root is required")
	}
	evidence := AuthorityInventory{Complete: true}
	goOwners := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		evidence.FilesScanned++
		text := string(content)
		if strings.HasSuffix(path, ".go") && strings.Contains(text, "RetryCeiling") {
			goOwners++
		}
		if copiedPolicyPattern.MatchString(text) {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			evidence.HasDuplicateOwner = true
			evidence.Complete = false
			evidence.Violations = append(evidence.Violations, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return AuthorityInventory{}, fmt.Errorf("scan authority inventory: %w", err)
	}
	if goOwners > 1 {
		evidence.HasDuplicateOwner = true
		evidence.Complete = false
	}
	return evidence, nil
}
