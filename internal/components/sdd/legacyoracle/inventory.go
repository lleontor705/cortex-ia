// Package legacyoracle contains the temporary, read-only migration oracle for
// the pre-typed SDD injector. It describes legacy responsibilities; it never
// writes files and is not a production installation path.
package legacyoracle

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

type Classification string

const (
	Retain  Classification = "retain"
	Replace Classification = "replace"
	Retire  Classification = "retire"
)

// Asset records one observable responsibility of the legacy injector. The
// TypedEquivalent is a semantic path, not a filesystem mutation instruction.
type Asset struct {
	ID               string
	SourcePath       string
	Behavior         string
	Classification   Classification
	TypedEquivalent  string
	RetirementReason string
}

type Inventory struct {
	Assets []Asset
}

// Build returns a deterministic inventory of all legacy injector inputs and
// behaviors. The inventory is intentionally pure: it only reads embedded
// assets and returns data suitable for parity tests and migration evidence.
func Build() (Inventory, error) {
	var entries []Asset
	add := func(id, source, behavior, typed string, class Classification) {
		entries = append(entries, Asset{ID: id, SourcePath: source, Behavior: behavior, TypedEquivalent: typed, Classification: class})
	}

	add("orchestrator/root", "generic/sdd-orchestrator.md", "shared progressive-disclosure orchestrator prompt", "root/index", Replace)
	add("orchestrator/reference", "generic/sdd-orchestrator-reference.md", "just-in-time operational reference modules", "root/modules", Replace)
	add("orchestrator/single", "generic/sdd-orchestrator-single.md", "single-agent system prompt lowering", "role-stubs", Replace)

	for _, skill := range []string{
		"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize",
		"debate", "debug", "execute-plan", "ideate", "monitor", "open-pr", "file-issue", "parallel-dispatch", "scan-registry",
		"judgment-day", "onboard", "chained-pr", "cognitive-doc-design", "comment-writer", "go-testing", "skill-creator", "skill-improver", "work-unit-commits",
	} {
		class := Retire
		typed := ""
		reason := "utility skill is not a retained phase-install asset; it remains available through the project skill registry"
		if slices.Contains([]string{"bootstrap", "investigate", "draft-proposal", "write-specs", "architect", "decompose", "implement", "validate", "finalize"}, skill) {
			class, typed, reason = Retain, "skill/"+skill, ""
		}
		add("skill/"+skill, "skills/"+skill+"/SKILL.md", "install canonical skill content", typed, class)
		if reason != "" {
			entries[len(entries)-1].RetirementReason = reason
		}
	}

	commands, err := assets.ListDir("opencode/commands")
	if err != nil {
		return Inventory{}, fmt.Errorf("read legacy command inventory: %w", err)
	}
	for _, command := range commands {
		if command.IsDir() || !strings.HasSuffix(command.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(command.Name(), ".md")
		add("command/"+name, "opencode/commands/"+command.Name(), "install slash command", "", Retire)
		entries[len(entries)-1].RetirementReason = "slash commands are not emitted by the typed portable workflow; phase skills are the retained entry point"
	}

	add("role/sub-agents", "inject_roles.go", "render one role stub per canonical phase", "role-stubs", Replace)
	add("permissions/role-matrix", "inject_roles.go", "lower role-specific permissions", "permissions", Replace)
	add("models/role-routing", "inject_roles.go", "lower role model/step/temperature routing", "model-routes", Replace)
	add("install/rollback", "inject.go", "write shared assets and restore prior managed state", "typed-plan-receipt-rollback", Replace)
	add("legacy/team-lead", "inject_roles.go", "remove obsolete team-lead role and assets", "", Retire)
	entries[len(entries)-1].RetirementReason = "team-lead coordination is forbidden; ForgeSpec owns task readiness and CAS"

	slices.SortFunc(entries, func(a, b Asset) int { return strings.Compare(a.ID, b.ID) })
	return Inventory{Assets: entries}, nil
}

func (i Inventory) Validate() error {
	if len(i.Assets) == 0 {
		return fmt.Errorf("legacy inventory is empty")
	}
	seen := make(map[string]struct{}, len(i.Assets))
	for _, asset := range i.Assets {
		if asset.ID == "" || asset.SourcePath == "" || asset.Behavior == "" {
			return fmt.Errorf("legacy inventory contains incomplete asset %q", asset.ID)
		}
		if _, ok := seen[asset.ID]; ok {
			return fmt.Errorf("legacy inventory contains duplicate asset %q", asset.ID)
		}
		seen[asset.ID] = struct{}{}
		switch asset.Classification {
		case Retain, Replace:
			if asset.TypedEquivalent == "" {
				return fmt.Errorf("retained/replaced asset %q lacks typed equivalent", asset.ID)
			}
		case Retire:
			if strings.TrimSpace(asset.RetirementReason) == "" {
				return fmt.Errorf("retired asset %q lacks retirement reason", asset.ID)
			}
		default:
			return fmt.Errorf("asset %q has unknown classification %q", asset.ID, asset.Classification)
		}
	}
	return nil
}
