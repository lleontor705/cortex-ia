package agents_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// customSkill builds an honest typed custom-skill record for layout probes.
func customSkill(t *testing.T, id model.SkillID, content string) registry.Skill {
	t.Helper()
	digest := sha256.Sum256([]byte(content))
	return registry.Skill{
		ID:            id,
		Content:       []byte(content),
		ContentSHA256: hex.EncodeToString(digest[:]),
		Origin:        registry.OriginCustom,
	}
}

// TestFactory_NonCollidingLayout validates that every adapter registered by
// the default factory declares, through SkillLayoutProvider, relative,
// contained, deterministic, and mutually non-colliding custom-skill
// destinations (spec AC-ADAPT-1; design D8: adapters own host representation
// only, never registry authority).
func TestFactory_NonCollidingLayout(t *testing.T) {
	registered := agents.NewDefaultRegistry().All()
	if len(registered) == 0 {
		t.Fatal("default factory registered no adapters; skill layout contract has no grounding")
	}

	seenAgents := make(map[model.AgentID]bool, len(registered))
	globalOwners := make(map[string]model.AgentID) // destination -> owning agent
	for _, adapter := range registered {
		agentID := adapter.Agent()
		if agentID == "" {
			t.Fatalf("registered adapter %T reported an empty agent ID", adapter)
		}
		if seenAgents[agentID] {
			t.Fatalf("agent %q is registered more than once", agentID)
		}
		seenAgents[agentID] = true

		t.Run(string(agentID), func(t *testing.T) {
			provider, ok := adapter.(agents.SkillLayoutProvider)
			if !ok {
				t.Skipf("adapter %q does not implement SkillLayoutProvider yet; validation activates when the adapter declares its custom-skill layout", agentID)
			}
			validateSkillLayout(t, adapter, provider, globalOwners)
		})
	}
}

// validateSkillLayout exercises one adapter's declared layout with distinct
// typed custom skills and asserts every destination is valid, stable, unique
// within the adapter, and unclaimed by another adapter.
func validateSkillLayout(t *testing.T, adapter agents.Adapter, provider agents.SkillLayoutProvider, globalOwners map[string]model.AgentID) {
	t.Helper()

	home := t.TempDir()
	root, err := filepath.Rel(home, adapter.SkillsDir(home))
	if err != nil {
		t.Fatalf("derive skills root relative to home: %v", err)
	}
	skillsRoot := filepath.ToSlash(root)

	probes := []registry.Skill{
		customSkill(t, "probe-alpha", "# alpha\n"),
		customSkill(t, "probe-beta", "# beta\n"),
		customSkill(t, "probe-gamma", "# gamma\n"),
	}

	owners := make(map[string]model.SkillID) // destination -> owning skill
	for _, skill := range probes {
		first := provider.SkillDestinations(skill)
		if len(first) == 0 {
			t.Fatalf("adapter %q declared no destinations for custom skill %q", adapter.Agent(), skill.ID)
		}
		repeat := provider.SkillDestinations(skill)
		if !slices.Equal(first, repeat) {
			t.Fatalf("adapter %q returned unstable destinations for custom skill %q: %q then %q", adapter.Agent(), skill.ID, first, repeat)
		}

		for _, destination := range first {
			assertRelativeDestination(t, adapter.Agent(), skillsRoot, skill.ID, destination)

			if owner, collides := owners[destination]; collides {
				t.Fatalf("adapter %q collides on destination %q between custom skills %q and %q", adapter.Agent(), destination, owner, skill.ID)
			}
			owners[destination] = skill.ID

			if ownerAgent, collides := globalOwners[destination]; collides {
				t.Fatalf("destination %q is declared by both %q and %q", destination, ownerAgent, adapter.Agent())
			}
			globalOwners[destination] = adapter.Agent()
		}
	}
}

// assertRelativeDestination enforces the declared layout contract: relative
// slash-form paths only, lexically clean, no traversal, contained under the
// adapter's skills root, and Markdown data-asset form.
func assertRelativeDestination(t *testing.T, agentID model.AgentID, skillsRoot string, skillID model.SkillID, destination string) {
	t.Helper()
	if destination == "" {
		t.Fatalf("adapter %q returned an empty destination for custom skill %q", agentID, skillID)
	}
	if filepath.IsAbs(destination) {
		t.Fatalf("adapter %q returned absolute destination %q for custom skill %q; destinations must be relative", agentID, destination, skillID)
	}
	if filepath.ToSlash(destination) != destination {
		t.Fatalf("adapter %q returned %q for custom skill %q; destinations must use forward slashes", agentID, destination, skillID)
	}
	if path.Clean(destination) != destination {
		t.Fatalf("adapter %q returned unclean destination %q for custom skill %q", agentID, destination, skillID)
	}
	for _, segment := range strings.Split(destination, "/") {
		if segment == ".." {
			t.Fatalf("adapter %q returned traversing destination %q for custom skill %q", agentID, destination, skillID)
		}
	}
	if !strings.HasPrefix(destination, skillsRoot+"/") {
		t.Fatalf("adapter %q declared %q for custom skill %q outside its skills root %q", agentID, destination, skillID, skillsRoot)
	}
	if path.Ext(destination) != ".md" {
		t.Fatalf("adapter %q declared %q for custom skill %q; custom skills lower to Markdown data assets only", agentID, destination, skillID)
	}
}
