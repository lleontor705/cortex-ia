package catalog

import "github.com/lleontor705/cortex-ia/internal/model"

// DisableClass classifies whether a component may be disabled and, when it
// may not, the protection category (design D4). The zero value is the
// fail-closed protected default, so components without an explicit catalog
// descriptor can never be disabled.
type DisableClass uint8

const (
	// ProtectedUnclassified is the fail-closed default for components
	// without an explicit descriptor, including unknown IDs.
	ProtectedUnclassified DisableClass = iota
	// Optional marks explicit catalog entries that may be disabled.
	Optional
	// ProtectedAuthority protects core authority components (Cortex, ForgeSpec).
	ProtectedAuthority
	// ProtectedWorkflow protects the SDD workflow component.
	ProtectedWorkflow
	// ProtectedRequired protects transitive dependencies of retained selections.
	ProtectedRequired
)

// Protected reports whether the class forbids disabling the component.
func (c DisableClass) Protected() bool { return c != Optional }

// String returns the canonical class name for category-identifying
// diagnostics.
func (c DisableClass) String() string {
	switch c {
	case Optional:
		return "optional"
	case ProtectedAuthority:
		return "protected-authority"
	case ProtectedWorkflow:
		return "protected-workflow"
	case ProtectedRequired:
		return "protected-required"
	default:
		return "protected-unclassified"
	}
}

// ComponentInfo describes an installable component.
type ComponentInfo struct {
	ID          model.ComponentID
	Name        string
	Description string
	Deps        []model.ComponentID
	// Disable is the explicit disable-class descriptor. Omitting it keeps
	// the component protected (fail-closed).
	Disable DisableClass
}

// AllComponents returns all available components in dependency order.
func AllComponents() []ComponentInfo {
	return []ComponentInfo{
		{ID: model.ComponentCortex, Name: "Cortex Memory", Description: "Persistent cross-session memory with knowledge graph (19 MCP tools)", Deps: nil, Disable: ProtectedAuthority},
		{ID: model.ComponentForgeSpec, Name: "ForgeSpec", Description: "SDD contract validation, task board, file reservation (15 MCP tools)", Deps: nil, Disable: ProtectedAuthority},
		{ID: model.ComponentContext7, Name: "Context7", Description: "Live framework and library documentation via MCP", Deps: nil, Disable: Optional},
		{ID: model.ComponentConventions, Name: "Conventions", Description: "Shared cortex conventions and memory protocol", Deps: []model.ComponentID{model.ComponentCortex}, Disable: Optional},
		{ID: model.ComponentSDD, Name: "SDD Workflow", Description: "Full 9-phase Spec-Driven Development with orchestrator + 19 skills", Deps: []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec}, Disable: ProtectedWorkflow},
		{ID: model.ComponentSkills, Name: "Extra Skills", Description: "Additional utility skills (non-SDD)", Deps: nil, Disable: Optional},
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
			// SDD auto-pulls mailbox and conventions via deps.
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

// DisableClasses returns the effective disable classification for every
// catalog component given the retained selection. Transitive dependencies of
// retained components resolve as ProtectedRequired because disabling them
// would break a retained selection; every other component keeps its explicit
// descriptor. IDs absent from the returned map are unclassified and protected
// (fail-closed default).
func DisableClasses(retained []model.ComponentID) map[model.ComponentID]DisableClass {
	classes := make(map[model.ComponentID]DisableClass)
	for _, c := range AllComponents() {
		classes[c.ID] = c.Disable
	}
	retainedSet := make(map[model.ComponentID]bool, len(retained))
	for _, id := range retained {
		retainedSet[id] = true
	}
	for _, id := range ResolveDeps(retained) {
		if !retainedSet[id] {
			classes[id] = ProtectedRequired
		}
	}
	return classes
}
