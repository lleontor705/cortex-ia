package conformance

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPublicDocumentationHasNoCurrentMailboxOrTeamLeadClaims(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	paths := []string{"README.md", "AGENTS.md", ".sdd/skill-registry.md", ".atl/skill-registry.md", "llms.txt", "llms-full.txt"}
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			relative, _ := filepath.Rel(root, path)
			paths = append(paths, relative)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	currentSurface := regexp.MustCompile(`(?i)(team-lead|\bmsg_|\ba2a_|\bresource_|\bdlq_)`)
	classification := regexp.MustCompile(`(?i)(retired|historical|legacy|removed|unsupported|unbound|operator-controlled|never auto)`)
	all := strings.Builder{}
	for _, relative := range paths {
		file, err := os.Open(filepath.Join(root, relative))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			all.WriteString(text)
			all.WriteByte('\n')
			if currentSurface.MatchString(text) && !classification.MatchString(text) {
				t.Errorf("%s:%d has unclassified current surface: %s", filepath.ToSlash(relative), line, text)
			}
		}
		_ = file.Close()
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
	for _, required := range []string{
		"legacy-sequential", "direct-v1", "unsupported and unbound", "never automatically",
		"operator-controlled", "ForgeSpec", "Cortex", "runtime-native",
	} {
		if !strings.Contains(all.String(), required) {
			t.Errorf("public documentation missing required truth %q", required)
		}
	}
}
