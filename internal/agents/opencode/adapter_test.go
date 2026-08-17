package opencode_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// customSkill builds one honest typed custom-skill record for layout
// probes. The skillcore type carries identity and content only: agents,
// tools, permissions, and bindings are not fields a declaration could ever
// grant (design D8, typed_contracts.skillcore).
func customSkill(id model.SkillID, content string) skillcore.Skill {
	digest := sha256.Sum256([]byte(content))
	return skillcore.Skill{
		ID:            id,
		Content:       []byte(content),
		ContentSHA256: hex.EncodeToString(digest[:]),
		Origin:        skillcore.OriginCustom,
	}
}

// layoutProvider asserts the opencode adapter declares the skill-layout
// surface and returns it for probing.
func layoutProvider(t *testing.T) agents.SkillLayoutProvider {
	t.Helper()
	var adapter any = opencode.NewAdapter()
	provider, ok := adapter.(agents.SkillLayoutProvider)
	if !ok {
		t.Fatal("*opencode.Adapter does not implement agents.SkillLayoutProvider (missing method SkillDestinations)")
	}
	return provider
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	relative, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("derive %q relative to %q: %v", target, base, err)
	}
	return relative
}

// TestAdapter_CustomSkillDestination proves AC-ADAPT-1: the opencode
// adapter declares its custom-skill host representation as the native
// per-skill directory form .config/opencode/skills/<id>/SKILL.md — a
// home-relative, slash-separated, deterministic Markdown destination
// beneath the adapter's SkillsDir that OpenCode natively discovers.
func TestAdapter_CustomSkillDestination(t *testing.T) {
	provider := layoutProvider(t)
	adapter := opencode.NewAdapter()
	home := t.TempDir()

	skillsRoot := filepath.ToSlash(mustRel(t, home, adapter.SkillsDir(home)))

	for _, skill := range []skillcore.Skill{
		customSkill("demo-skill", "# demo\n"),
		customSkill("a1-tidy-id", "# tidy\n"),
	} {
		destinations := provider.SkillDestinations(skill)
		if len(destinations) != 1 {
			t.Fatalf("skill %q: got %d destinations %q, want exactly one", skill.ID, len(destinations), destinations)
		}
		got := destinations[0]
		want := skillsRoot + "/" + string(skill.ID) + "/SKILL.md"
		if got != want {
			t.Fatalf("skill %q: destination %q, want native form %q", skill.ID, got, want)
		}
		if filepath.IsAbs(got) {
			t.Fatalf("skill %q: destination %q must stay home-relative", skill.ID, got)
		}
		if filepath.ToSlash(got) != got || path.Clean(got) != got {
			t.Fatalf("skill %q: destination %q must be clean slash-form", skill.ID, got)
		}
		if !strings.HasPrefix(got, skillsRoot+"/") {
			t.Fatalf("skill %q: destination %q escapes the skills root %q", skill.ID, got, skillsRoot)
		}
		if !opencode.NativeLayout().IsNativePath(got) {
			t.Fatalf("skill %q: destination %q is not an OpenCode native skill path", skill.ID, got)
		}
		if repeat := provider.SkillDestinations(skill); !slices.Equal(repeat, destinations) {
			t.Fatalf("skill %q: destinations unstable: %q then %q", skill.ID, destinations, repeat)
		}
	}
}

// TestAdapter_TemporaryHomeIsolated proves AC-ADAPT-2: destination
// declaration is pure planning input. It performs no filesystem access, so
// exercising it creates no directory or file in any home — distinct
// temporary homes stay empty and uncontaminated, and because the
// destination is only a relative string joined to a home later by the
// pipeline, no write can reach a real home during planning.
func TestAdapter_TemporaryHomeIsolated(t *testing.T) {
	provider := layoutProvider(t)
	adapter := opencode.NewAdapter()
	homeA := t.TempDir()
	homeB := t.TempDir()

	skill := customSkill("isolated-skill", "# isolated\n")

	var destinations []string
	for range 10 {
		destinations = provider.SkillDestinations(skill)
		_ = adapter.SkillsDir(homeA)
		_ = adapter.GlobalConfigDir(homeB)
	}

	for _, home := range []string{homeA, homeB} {
		for _, destination := range destinations {
			if target := filepath.Join(home, filepath.FromSlash(destination)); !absent(t, target) {
				t.Fatalf("planning created or touched %q in temporary home %q", destination, home)
			}
		}
		entries, err := os.ReadDir(home)
		if err != nil {
			t.Fatalf("read temporary home %q: %v", home, err)
		}
		if len(entries) != 0 {
			t.Fatalf("temporary home %q contaminated during planning: %v", home, entries)
		}
	}
}

func absent(t *testing.T, target string) bool {
	t.Helper()
	_, err := os.Stat(target)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		t.Fatalf("stat %q: %v", target, err)
	}
	return false
}

// TestAdapter_SkillDestinationsFailClosed proves an invalid skill ID can
// never become a path segment: every ID outside the strict lowercase ASCII
// grammar — including traversal, separator, case, and escape spellings —
// fails closed to no destinations instead of declaring an unsafe path.
func TestAdapter_SkillDestinationsFailClosed(t *testing.T) {
	provider := layoutProvider(t)
	for _, id := range []string{
		"",
		".",
		"..",
		"../escape",
		"skills/../../escape",
		`back\slash`,
		"UPPERCASE",
		"Mixed-Case",
		"trailing-hyphen-",
		"-leading-hyphen",
		"double--hyphen",
		"with.dot",
		"with space",
		"tab\tid",
		"newline\nid",
		"~home",
		"/absolute",
		`C:\drive`,
	} {
		if destinations := provider.SkillDestinations(customSkill(model.SkillID(id), "# body\n")); len(destinations) != 0 {
			t.Fatalf("invalid skill ID %q declared destinations %q; want fail-closed none", id, destinations)
		}
	}
}

// TestAdapter_SkillDestinationsAreDataAssetsOnly proves AC-ADAPT-3: a
// declared layout is host representation only. Every destination is a plain
// SKILL.md data asset beneath SkillsRoot — never a command, subagent,
// config, or workflow overlay — so a skill declaration cannot grant host
// authority surfaces.
func TestAdapter_SkillDestinationsAreDataAssetsOnly(t *testing.T) {
	provider := layoutProvider(t)
	layout := opencode.NativeLayout()

	for _, skill := range []skillcore.Skill{
		customSkill("asset-only", "# asset\n"),
		customSkill("another-skill", "# another\n"),
	} {
		for _, destination := range provider.SkillDestinations(skill) {
			if !strings.HasPrefix(destination, layout.SkillsRoot+"/") {
				t.Fatalf("skill %q destination %q must stay beneath the skills root %q", skill.ID, destination, layout.SkillsRoot)
			}
			if base := path.Base(destination); base != "SKILL.md" {
				t.Fatalf("skill %q destination %q must be a SKILL.md data asset, got base %q", skill.ID, destination, base)
			}
			for _, forbidden := range []string{
				layout.CommandsRoot,
				layout.AgentsRoot,
				layout.WorkflowRoot,
				path.Join(layout.ConfigRoot, "opencode.json"),
				path.Join(layout.ConfigRoot, "opencode.jsonc"),
				path.Join(layout.ConfigRoot, "AGENTS.md"),
			} {
				if destination == forbidden || strings.HasPrefix(destination, forbidden+"/") {
					t.Fatalf("skill %q destination %q grants host authority surface %q", skill.ID, destination, forbidden)
				}
			}
		}
	}
}
