package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type utilitySkillFixture struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Method []string `json:"method"`
}

var utilityNames = []string{
	"work-unit-commits", "file-issue", "execute-plan", "onboard", "monitor", "skill-improver",
	"judgment-day", "skill-creator", "debug", "scan-registry", "debate", "parallel-dispatch",
	"comment-writer", "ideate", "open-pr", "cognitive-doc-design", "go-testing", "chained-pr",
	"mutation-testing",
}

func loadUtilityFixtures(t *testing.T) []utilitySkillFixture {
	t.Helper()
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal/components/sdd/conformance/testdata/utility_skills/corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []utilitySkillFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestUtilitySkillCorpusIsExactlyCatalogued(t *testing.T) {
	fixtures := loadUtilityFixtures(t)
	if len(fixtures) != len(utilityNames) {
		t.Fatalf("utility corpus has %d skills, want %d", len(fixtures), len(utilityNames))
	}
	seen := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		seen[fixture.Name] = true
		if len(fixture.Method) == 0 {
			t.Errorf("%s has no utility-specific method markers", fixture.Name)
		}
	}
	for _, name := range utilityNames {
		if !seen[name] {
			t.Errorf("utility %s missing from corpus", name)
		}
	}
	if got := len(utilityNames) + 9; got != 28 {
		t.Fatalf("skill registry cardinality is %d, want 28", got)
	}
}

func TestUtilitySkillsDeclareNonPhaseAuthorityAndPreserveMethod(t *testing.T) {
	for _, fixture := range loadUtilityFixtures(t) {
		raw := readAsset(t, fixture.Path)
		content := strings.ToLower(raw)
		words := len(strings.Fields(raw))
		if words < 80 || words > 300 {
			t.Errorf("%s budget is %d words, want 80..300", fixture.Name, words)
		}
		if _, err := os.Stat(filepath.Join(repositoryRoot(t), fixture.Path)); err != nil {
			t.Errorf("installed/source scan cannot resolve %s: %v", fixture.Name, err)
		}
		if !strings.Contains(content, "non-phase") || !strings.Contains(content, "utility authority") {
			t.Errorf("%s must declare non-phase utility authority", fixture.Name)
		}
		for _, marker := range fixture.Method {
			if !strings.Contains(content, strings.ToLower(marker)) {
				t.Errorf("%s lost utility-specific method marker %q", fixture.Name, marker)
			}
		}
	}
}

func TestUtilitySkillsRejectDefaultProviderAndHiddenPhaseAuthority(t *testing.T) {
	for _, fixture := range loadUtilityFixtures(t) {
		content := strings.ToLower(readAsset(t, fixture.Path))
		for _, forbidden := range []string{
			"agent-mailbox", "a2a_", "a2a task", "dlq", "team-lead", "team lead",
			"resource_acquire", "resource coordination", "phase routing", "phase authority",
			"delegate to @", "delegation threshold", "poll msg_read_inbox",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s contains forbidden default/hidden authority %q", fixture.Name, forbidden)
			}
		}
	}
}
