package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type phaseSkillFixture struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	MaxWords    int      `json:"max_words"`
	Required    []string `json:"required"`
	Forbidden   []string `json:"forbidden"`
	DecisionIDs []string `json:"decision_ids"`
}

func TestPhaseSkillCorpusConformsToOwnedStructure(t *testing.T) {
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal/components/sdd/conformance/testdata/phase_skills/corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []phaseSkillFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 9 {
		t.Fatalf("fixture corpus has %d skills, want 9", len(fixtures))
	}
	for _, fixture := range fixtures {
		content := readAsset(t, fixture.Path)
		words := len(strings.Fields(content))
		if words < 600 || words > fixture.MaxWords {
			t.Errorf("%s word count %d outside 600..%d", fixture.Name, words, fixture.MaxWords)
		}
		for _, marker := range fixture.Required {
			if !strings.Contains(strings.ToLower(content), strings.ToLower(marker)) {
				t.Errorf("%s missing required marker %q", fixture.Name, marker)
			}
		}
		for _, marker := range fixture.Forbidden {
			if strings.Contains(strings.ToLower(content), strings.ToLower(marker)) {
				t.Errorf("%s contains forbidden default policy %q", fixture.Name, marker)
			}
		}
		for _, id := range fixture.DecisionIDs {
			if !strings.Contains(content, id) {
				t.Errorf("%s missing executable decision ID %q", fixture.Name, id)
			}
		}
	}
}

func TestPhaseSkillsSeparateStatusAndVerdictVocabulary(t *testing.T) {
	for _, name := range []string{"implement", "validate", "finalize"} {
		content := strings.ToLower(readAsset(t, "internal/assets/skills/"+name+"/SKILL.md"))
		if !strings.Contains(content, "phase status") || !strings.Contains(content, "verification verdict") {
			t.Errorf("%s must identify phase status and verification verdict separately", name)
		}
		if strings.Contains(content, "status = verdict") || strings.Contains(content, "verdict is status") {
			t.Errorf("%s collapses status and verdict vocabulary", name)
		}
	}
}

func TestPhaseSkillOwnershipManifestClassifiesRemovedParagraphs(t *testing.T) {
	root := repositoryRoot(t)
	path := filepath.Join(root, "internal/assets/skills/ownership-manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Skills map[string]struct {
			Removed []struct {
				Classification string `json:"classification"`
				Owner          string `json:"owner"`
			} `json:"removed"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Skills) != 27 {
		t.Fatalf("ownership manifest has %d skills, want registry total 27", len(manifest.Skills))
	}
	for _, skill := range []string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize"} {
		if _, ok := manifest.Skills[skill]; !ok {
			t.Errorf("phase skill %s missing from ownership manifest", skill)
		}
	}
	allowed := map[string]bool{"moved": true, "generated": true, "phase-specific retained": true, "utility-specific retained": true, "quarantined": true, "obsolete": true}
	for skill, entry := range manifest.Skills {
		if len(entry.Removed) == 0 {
			t.Errorf("%s has no removed paragraph classification", skill)
		}
		for _, removed := range entry.Removed {
			if !allowed[removed.Classification] || strings.TrimSpace(removed.Owner) == "" {
				t.Errorf("%s has incomplete ownership record: %#v", skill, removed)
			}
		}
	}
}

func TestPhaseSkillCorpusRejectsInvalidFixture(t *testing.T) {
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal/components/sdd/conformance/testdata/phase_skills/invalid_missing_example.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := strings.ToLower(string(data))
	if strings.Contains(content, "valid example") || strings.Contains(content, "invalid example") {
		t.Fatal("invalid fixture unexpectedly contains both example markers")
	}
}
