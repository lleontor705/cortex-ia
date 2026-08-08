package conformance

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// retainedAssetRoots is the source-of-truth inventory for assets that can be
// installed by the typed workflow. Historical/utility skills are intentionally
// excluded: they are not part of an installed phase bundle.
var retainedAssetRoots = []string{
	"internal/assets/generic/sdd-orchestrator-root-index.md",
	"internal/assets/generic/sdd-root",
	"internal/assets/generic/profiles",
	"internal/assets/skills/_shared/sdd-phase-contract.md",
	"internal/assets/skills/_shared/cortex-convention.md",
	"internal/assets/skills/_shared/cortex-advanced.md",
	"internal/assets/skills/bootstrap/SKILL.md",
	"internal/assets/skills/investigate/SKILL.md",
	"internal/assets/skills/draft-proposal/SKILL.md",
	"internal/assets/skills/write-specs/SKILL.md",
	"internal/assets/skills/architect/SKILL.md",
	"internal/assets/skills/decompose/SKILL.md",
	"internal/assets/skills/implement/SKILL.md",
	"internal/assets/skills/validate/SKILL.md",
	"internal/assets/skills/finalize/SKILL.md",
	"internal/assets/opencode/commands",
	"internal/assets/roles/testdata",
}

var staleToolPatterns = map[string]*regexp.Regexp{
	"agent-mailbox": regexp.MustCompile(`(?i)agent[- ]mailbox`),
	"a2a":           regexp.MustCompile(`(?i)\ba2a(?:[_-]|\b)`),
	"team-lead":     regexp.MustCompile(`(?i)\bteam[- ]lead\b`),
	"mailbox":       regexp.MustCompile(`(?i)\bmailbox\b`),
	"removed-tool":  regexp.MustCompile(`(?i)\b(?:msg|resource|dlq)_(?:send|request|broadcast|acquire|release|check|list|retry|purge)`),
	"stale-model":   regexp.MustCompile(`(?i)\b(?:gpt-4|claude-3|gemini-pro)\b`),
	"legacy-cortex": regexp.MustCompile(`(?i)\bmem_[a-z_]+\b`),
}

func TestRetainedInstalledSourcesHaveNoStaleTools(t *testing.T) {
	root := repositoryRoot(t)
	files := retainedAssetFiles(t, root)
	if len(files) == 0 {
		t.Fatal("retained asset inventory is empty")
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read retained asset %s: %v", path, err)
		}
		text := string(data)
		for name, pattern := range staleToolPatterns {
			if match := pattern.FindString(text); match != "" {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s contains stale %s reference %q", filepath.ToSlash(rel), name, match)
			}
		}
	}
}

func TestStaleToolScannerDetectsEveryForbiddenClass(t *testing.T) {
	fixture := "agent-mailbox a2a_submit_task team-lead Mailbox msg_send gpt-4 mem_search"
	for name, pattern := range staleToolPatterns {
		if !pattern.MatchString(fixture) {
			t.Errorf("scanner pattern %q does not detect its forbidden class", name)
		}
	}
}

func retainedAssetFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, rel := range retainedAssetRoots {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("retained asset %s is missing: %v", rel, err)
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(file string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				return nil
			}
			files = append(files, file)
			return nil
		})
		if err != nil {
			t.Fatalf("walk retained asset %s: %v", rel, err)
		}
	}
	return files
}
