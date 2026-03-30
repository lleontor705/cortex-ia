package catalog

import "github.com/lleontor705/cortex-ia/internal/model"

// ComponentInfo describes an installable component.
type ComponentInfo struct {
	ID          model.ComponentID
	Name        string
	Description string
	Deps        []model.ComponentID
}

// AllComponents returns all available components in dependency order.
func AllComponents() []ComponentInfo {
	return []ComponentInfo{
		{ID: model.ComponentCortex, Name: "Cortex Memory", Description: "Persistent cross-session memory with knowledge graph (19 MCP tools)", Deps: nil},
		{ID: model.ComponentCLIOrch, Name: "CLI Orchestrator", Description: "Multi-CLI routing with circuit breaker and fallback (4 MCP tools)", Deps: nil},
		{ID: model.ComponentMailbox, Name: "Agent Mailbox", Description: "Inter-agent messaging, threads, broadcast (9 MCP tools)", Deps: nil},
		{ID: model.ComponentForgeSpec, Name: "ForgeSpec", Description: "SDD contract validation, task board, file reservation (15 MCP tools)", Deps: nil},
		{ID: model.ComponentContext7, Name: "Context7", Description: "Live framework and library documentation via MCP", Deps: nil},
		{ID: model.ComponentConventions, Name: "Conventions", Description: "Shared cortex conventions and memory protocol", Deps: []model.ComponentID{model.ComponentCortex}},
		{ID: model.ComponentSDD, Name: "SDD Workflow", Description: "Full 9-phase Spec-Driven Development with orchestrator + 19 skills", Deps: []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec, model.ComponentMailbox, model.ComponentConventions}},
		{ID: model.ComponentSkills, Name: "Extra Skills", Description: "Additional utility skills (non-SDD)", Deps: nil},
	}
}

// ComponentMap returns components indexed by ID.
func ComponentMap() map[model.ComponentID]ComponentInfo {
	m := make(map[model.ComponentID]ComponentInfo)
	for _, c := range AllComponents() {
		m[c.ID] = c
	}
	return m
}

// ComponentsForPreset returns the component IDs for a given preset.
func ComponentsForPreset(preset model.PresetID) []model.ComponentID {
	switch preset {
	case model.PresetFull:
		ids := make([]model.ComponentID, 0)
		for _, c := range AllComponents() {
			ids = append(ids, c.ID)
		}
		return ids
	case model.PresetMinimal:
		return []model.ComponentID{
			model.ComponentCortex,
			model.ComponentForgeSpec,
			model.ComponentContext7,
			model.ComponentSDD,
			// SDD auto-pulls mailbox and conventions via deps
		}
	default:
		return nil
	}
}

// ResolveDeps expands a component list to include all transitive dependencies.
// Returns components in dependency order (deps before dependents).
func ResolveDeps(selected []model.ComponentID) []model.ComponentID {
	cmap := ComponentMap()
	visited := make(map[model.ComponentID]bool)
	var result []model.ComponentID

	var visit func(id model.ComponentID)
	visit = func(id model.ComponentID) {
		if visited[id] {
			return
		}
		visited[id] = true
		info, ok := cmap[id]
		if !ok {
			return
		}
		for _, dep := range info.Deps {
			visit(dep)
		}
		result = append(result, id)
	}

	for _, id := range selected {
		visit(id)
	}
	return result
}
