package skillregistry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test helpers ---

// createTestSkill writes a SKILL.md with the given frontmatter into dir.
func createTestSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	skillDir := filepath.Join(dir, "SKILL.md")
	parent := filepath.Dir(skillDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", parent, err)
	}
	content := "---\nname: " + name + "\ndescription: >\n  " + description + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(skillDir, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

// --- Scan() tests ---

func TestScan_Deterministic(t *testing.T) {
	out1, err := Scan()
	if err != nil {
		t.Fatalf("first Scan() failed: %v", err)
	}
	out2, err := Scan()
	if err != nil {
		t.Fatalf("second Scan() failed: %v", err)
	}
	if len(out1.Skills) != len(out2.Skills) {
		t.Fatalf("skill count differs between calls: %d vs %d", len(out1.Skills), len(out2.Skills))
	}
	for i := range out1.Skills {
		if out1.Skills[i] != out2.Skills[i] {
			t.Errorf("skill[%d] differs:\n  first:  %+v\n  second: %+v", i, out1.Skills[i], out2.Skills[i])
		}
	}
}

func TestScan_FindsEmbeddedSkills(t *testing.T) {
	out, err := Scan()
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}
	found := make(map[string]bool)
	for _, s := range out.Skills {
		found[s.Name] = true
	}
	expected := []string{"bootstrap", "judgment-day", "implement", "validate", "architect"}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected embedded skill %q not found in scan results", name)
		}
	}
}

func TestScan_SortedByName(t *testing.T) {
	out, err := Scan()
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}
	for i := 1; i < len(out.Skills); i++ {
		if out.Skills[i-1].Name > out.Skills[i].Name {
			t.Errorf("skills not sorted by name: %q > %q at index %d",
				out.Skills[i-1].Name, out.Skills[i].Name, i)
		}
	}
}

// --- Scan() with filesystem tiers ---

func TestScanSources_ProjectAndCommunity(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create a project-level skill.
	createTestSkill(t, filepath.Join(tmpDir, "skills", "proj-skill"),
		"proj-skill", "A project skill. Trigger: when testing project skills.")

	// Create a community skill.
	createTestSkill(t, filepath.Join(homeDir, ".cortex-ia", "skills-community", "comm-skill"),
		"comm-skill", "A community skill. Trigger: when testing community skills.")

	out, err := scanSources(tmpDir, homeDir)
	if err != nil {
		t.Fatalf("scanSources failed: %v", err)
	}

	found := make(map[string]string) // name → category
	for _, s := range out.Skills {
		found[s.Name] = s.Category
	}
	if cat, ok := found["proj-skill"]; !ok || cat != "project" {
		t.Errorf("project skill not found or wrong category: %q", cat)
	}
	if cat, ok := found["comm-skill"]; !ok || cat != "community" {
		t.Errorf("community skill not found or wrong category: %q", cat)
	}
}

func TestScanSources_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Same skill name in both project and community.
	createTestSkill(t, filepath.Join(tmpDir, "skills", "shared-name"),
		"shared-name", "Project version. Trigger: project.")
	createTestSkill(t, filepath.Join(homeDir, ".cortex-ia", "skills-community", "shared-name"),
		"shared-name", "Community version. Trigger: community.")

	out, err := scanSources(tmpDir, homeDir)
	if err != nil {
		t.Fatalf("scanSources failed: %v", err)
	}

	count := 0
	var category string
	for _, s := range out.Skills {
		if s.Name == "shared-name" {
			count++
			category = s.Category
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for shared-name, got %d", count)
	}
	if category != "project" {
		t.Errorf("expected project to win dedup, got %q", category)
	}
}

// --- ScanCached tests ---

func TestScanCached_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, ".sdd", ".skill-registry-cache")

	createTestSkill(t, filepath.Join(tmpDir, "skills", "cached-skill"),
		"cached-skill", "A cached skill. Trigger: testing cache hit.")

	// First call generates and writes cache.
	out1, err := scanCachedAt(cachePath, tmpDir, homeDir, time.Hour)
	if err != nil {
		t.Fatalf("first scanCachedAt failed: %v", err)
	}

	// Verify cache file was created.
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("cache file was not created on first call")
	}

	// Second call with identical content → cache hit, same result.
	out2, err := scanCachedAt(cachePath, tmpDir, homeDir, time.Hour)
	if err != nil {
		t.Fatalf("second scanCachedAt failed: %v", err)
	}

	if len(out1.Skills) != len(out2.Skills) {
		t.Errorf("skill count differs on cache hit: %d vs %d", len(out1.Skills), len(out2.Skills))
	}
}

func TestScanCached_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, ".sdd", ".skill-registry-cache")

	skillDir := filepath.Join(tmpDir, "skills", "modifiable-skill")
	createTestSkill(t, skillDir, "modifiable-skill",
		"Original description. Trigger: original trigger.")

	// First call.
	out1, err := scanCachedAt(cachePath, tmpDir, homeDir, time.Hour)
	if err != nil {
		t.Fatalf("first scanCachedAt failed: %v", err)
	}

	// Modify the skill file.
	createTestSkill(t, skillDir, "modifiable-skill",
		"Modified description. Trigger: modified trigger.")

	// Second call → hash mismatch → cache miss → rescan.
	out2, err := scanCachedAt(cachePath, tmpDir, homeDir, time.Hour)
	if err != nil {
		t.Fatalf("second scanCachedAt failed: %v", err)
	}

	// Verify the regenerated output reflects the modification.
	var trigger string
	for _, s := range out2.Skills {
		if s.Name == "modifiable-skill" {
			trigger = s.Trigger
		}
	}
	if !strings.Contains(trigger, "modified") {
		t.Errorf("cache miss did not regenerate: expected trigger containing 'modified', got %q", trigger)
	}

	// The first output should NOT have "modified".
	var trigger1 string
	for _, s := range out1.Skills {
		if s.Name == "modifiable-skill" {
			trigger1 = s.Trigger
		}
	}
	if strings.Contains(trigger1, "modified") {
		t.Errorf("first output unexpectedly contains 'modified': %q", trigger1)
	}
}

func TestScanCached_CorruptCache(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, ".sdd", ".skill-registry-cache")

	createTestSkill(t, filepath.Join(tmpDir, "skills", "survivor"),
		"survivor", "Survives corrupt cache. Trigger: corrupt cache.")

	// Write garbage to the cache file.
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("THIS IS NOT VALID JSON{{{!!!"), 0o644); err != nil {
		t.Fatalf("write garbage cache: %v", err)
	}

	// Should NOT crash, should invalidate and regenerate.
	out, err := scanCachedAt(cachePath, tmpDir, homeDir, time.Hour)
	if err != nil {
		t.Fatalf("scanCachedAt with corrupt cache crashed: %v", err)
	}

	// Verify the skill was still found (via rescan).
	found := false
	for _, s := range out.Skills {
		if s.Name == "survivor" {
			found = true
		}
	}
	if !found {
		t.Error("expected skills despite corrupt cache, found none")
	}

	// Verify cache was rewritten with valid data.
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file not rewritten: %v", err)
	}
	var cd cacheData
	if err := json.Unmarshal(data, &cd); err != nil {
		t.Errorf("rewritten cache is not valid JSON: %v", err)
	}
}

// --- FormatMarkdown tests ---

func TestFormatMarkdown_ValidMarkdown(t *testing.T) {
	out := RegistryOutput{
		Skills: []SkillEntry{
			{Name: "beta", Path: "skills/beta/SKILL.md", Trigger: "test beta", Category: "project"},
			{Name: "alpha", Path: "skills/alpha/SKILL.md", Trigger: "test alpha", Category: "embedded"},
		},
		Version: "1.0.0",
	}
	md := FormatMarkdown(out)

	if !strings.Contains(md, "| Name") {
		t.Error("markdown missing table header with 'Name'")
	}
	if !strings.Contains(md, "|---|") {
		t.Error("markdown missing table separator row")
	}
	if !strings.Contains(md, "alpha") {
		t.Error("markdown missing skill 'alpha'")
	}
	if !strings.Contains(md, "beta") {
		t.Error("markdown missing skill 'beta'")
	}

	// Verify sorted (alpha before beta).
	alphaIdx := strings.Index(md, "alpha")
	betaIdx := strings.Index(md, "beta")
	if alphaIdx < 0 || betaIdx < 0 || alphaIdx > betaIdx {
		t.Error("markdown entries not sorted by name")
	}
}

func TestFormatMarkdown_EmptyTrigger(t *testing.T) {
	out := RegistryOutput{
		Skills: []SkillEntry{
			{Name: "no-trigger", Path: "p", Category: "embedded"},
		},
	}
	md := FormatMarkdown(out)
	if !strings.Contains(md, "N/A") {
		t.Error("empty trigger should render as 'N/A'")
	}
}

func TestFormatMarkdown_Deterministic(t *testing.T) {
	out := RegistryOutput{
		Skills: []SkillEntry{
			{Name: "zeta", Path: "z", Trigger: "tz", Category: "a"},
			{Name: "alpha", Path: "a", Trigger: "ta", Category: "b"},
			{Name: "mid", Path: "m", Trigger: "tm", Category: "c"},
		},
	}
	md1 := FormatMarkdown(out)
	md2 := FormatMarkdown(out)
	if md1 != md2 {
		t.Error("FormatMarkdown is not deterministic — same input produced different output")
	}
}

// --- FormatJSON tests ---

func TestFormatJSON_ValidJSON(t *testing.T) {
	out := RegistryOutput{
		Skills: []SkillEntry{
			{Name: "test-skill", Path: "skills/test/SKILL.md", Trigger: "testing", Category: "embedded"},
		},
		Version: "1.0.0",
	}
	jsonStr, err := FormatJSON(out)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("FormatJSON output is not valid JSON: %v", err)
	}
	if parsed["version"] != "1.0.0" {
		t.Errorf("JSON version mismatch: got %v", parsed["version"])
	}
}

func TestFormatJSON_SortedByName(t *testing.T) {
	out := RegistryOutput{
		Skills: []SkillEntry{
			{Name: "zulu", Path: "z", Category: "a"},
			{Name: "alpha", Path: "a", Category: "b"},
		},
	}
	jsonStr, err := FormatJSON(out)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}
	var parsed RegistryOutput
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Skills) < 2 {
		t.Fatalf("expected 2 skills, got %d", len(parsed.Skills))
	}
	if parsed.Skills[0].Name != "alpha" || parsed.Skills[1].Name != "zulu" {
		t.Errorf("JSON skills not sorted: got %q, %q", parsed.Skills[0].Name, parsed.Skills[1].Name)
	}
}
