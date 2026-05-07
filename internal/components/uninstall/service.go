package uninstall

import (
	"errors"
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// Result reports what happened during an uninstall run.
type Result struct {
	BackupID         string
	ChangedFiles     []string
	RemovedFiles     []string
	RemovedDirs      []string
	SkippedNonEmpty  []string
	AgentsRemoved    []model.AgentID
	ComponentsScoped []model.ComponentID
}

// Selection describes the scope of an uninstall: which agents and which
// components to peel back.
//
// An empty Agents slice means "all installed agents tracked in state".
// An empty Components slice means "all known components".
// All=true is a convenience equivalent to leaving both fields empty AND removing
// the cortex-ia state file at the end.
type Selection struct {
	Agents     []model.AgentID
	Components []model.ComponentID
	All        bool
	DryRun     bool
}

// allManagedComponents is the canonical list the uninstaller knows how to clean.
// Order matters for execution: marker-based components first (so the agent's
// system prompt is left clean), then MCP, then sub-agent / commands / skills dirs.
var allManagedComponents = []model.ComponentID{
	model.ComponentPersona,
	model.ComponentConventions,
	model.ComponentSDD,
	model.ComponentCortex,
	model.ComponentForgeSpec,
	model.ComponentMailbox,
	model.ComponentContext7,
	model.ComponentSkills,
	model.ComponentGGA,
}

// Service plans and executes an uninstall against a registry of agents.
type Service struct {
	homeDir  string
	registry *agents.Registry
}

// NewService returns a Service backed by the default agent registry.
func NewService(homeDir string) *Service {
	return &Service{homeDir: homeDir, registry: agents.NewDefaultRegistry()}
}

// NewServiceWithRegistry lets callers (tests) inject a custom registry.
func NewServiceWithRegistry(homeDir string, reg *agents.Registry) *Service {
	return &Service{homeDir: homeDir, registry: reg}
}

// Plan builds the dedup'd ordered operation list for a Selection without
// executing anything. Useful for --dry-run output and for the pipeline step
// that wants to compute backup paths upfront.
func (s *Service) Plan(sel Selection) ([]operation, error) {
	agentsToClean, err := s.resolveAgents(sel)
	if err != nil {
		return nil, err
	}
	comps := s.resolveComponents(sel)

	plan := make([]operation, 0, len(agentsToClean)*len(comps)*2)
	for _, comp := range comps {
		for _, agentID := range agentsToClean {
			adapter, getErr := s.registry.Get(agentID)
			if getErr != nil {
				continue
			}
			plan = append(plan, componentOperations(s.homeDir, adapter, comp)...)
		}
	}
	return dedupeOperations(plan), nil
}

// Apply runs the planned operations. Errors are aggregated and returned;
// callers wrap Apply in a snapshot+rollback so a partial failure can be undone.
//
// On dry-run, no operation runs and Result.ChangedFiles lists every path the
// plan would have touched.
func (s *Service) Apply(sel Selection) (Result, error) {
	plan, err := s.Plan(sel)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		ComponentsScoped: s.resolveComponents(sel),
	}

	for _, op := range plan {
		if sel.DryRun {
			res.ChangedFiles = append(res.ChangedFiles, op.path)
			continue
		}
		changed, err := applyOperation(op)
		if err != nil {
			return res, fmt.Errorf("uninstall %s/%s: %w", op.agent, op.component, err)
		}
		if !changed {
			if op.typeID == opRemoveIfEmpty {
				res.SkippedNonEmpty = append(res.SkippedNonEmpty, op.path)
			}
			continue
		}
		switch op.typeID {
		case opRemoveFile, opRemoveJSONKey:
			res.RemovedFiles = append(res.RemovedFiles, op.path)
		case opRemoveTree, opRemoveIfEmpty:
			res.RemovedDirs = append(res.RemovedDirs, op.path)
		default:
			res.ChangedFiles = append(res.ChangedFiles, op.path)
		}
	}

	if !sel.DryRun {
		if err := s.updateState(sel, &res); err != nil {
			return res, fmt.Errorf("update state after uninstall: %w", err)
		}
	}
	return res, nil
}

// PathsToBackup returns every file the planned uninstall would touch. Used by
// the pipeline to seed a pre-uninstall snapshot.
func (s *Service) PathsToBackup(sel Selection) ([]string, error) {
	plan, err := s.Plan(sel)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(plan))
	seen := make(map[string]struct{}, len(plan))
	for _, op := range plan {
		if op.path == "" {
			continue
		}
		if _, ok := seen[op.path]; ok {
			continue
		}
		seen[op.path] = struct{}{}
		out = append(out, op.path)
	}
	return out, nil
}

// resolveAgents normalises sel.Agents (empty ⇒ everything in state).
func (s *Service) resolveAgents(sel Selection) ([]model.AgentID, error) {
	if len(sel.Agents) > 0 {
		// Validate that every requested agent is registered.
		out := make([]model.AgentID, 0, len(sel.Agents))
		for _, id := range sel.Agents {
			if _, err := s.registry.Get(id); err != nil {
				return nil, fmt.Errorf("uninstall: unknown agent %q: %w", id, err)
			}
			out = append(out, id)
		}
		return out, nil
	}

	st, err := state.Load(s.homeDir)
	if err != nil {
		return nil, fmt.Errorf("uninstall: load state: %w", err)
	}
	if len(st.InstalledAgents) == 0 {
		return nil, errors.New("uninstall: nothing to do — no agents recorded in state")
	}
	return st.InstalledAgents, nil
}

// resolveComponents normalises sel.Components (empty ⇒ all known).
func (s *Service) resolveComponents(sel Selection) []model.ComponentID {
	if len(sel.Components) > 0 {
		out := make([]model.ComponentID, len(sel.Components))
		copy(out, sel.Components)
		return out
	}
	out := make([]model.ComponentID, len(allManagedComponents))
	copy(out, allManagedComponents)
	return out
}

// updateState rewrites cortex-ia's state.json to remove the agents/components
// that were just uninstalled. When sel.All=true the full state is cleared.
func (s *Service) updateState(sel Selection, res *Result) error {
	if sel.All {
		// Best-effort: ignore "file does not exist" errors.
		st, err := state.Load(s.homeDir)
		if err == nil {
			res.AgentsRemoved = append(res.AgentsRemoved, st.InstalledAgents...)
		}
		// Reset state to empty rather than deleting the file so that subsequent
		// installs don't have to recreate the directory.
		return state.Save(s.homeDir, state.State{})
	}

	st, err := state.Load(s.homeDir)
	if err != nil {
		return err
	}

	// Strip agents.
	if len(sel.Agents) > 0 {
		removed := map[model.AgentID]struct{}{}
		for _, id := range sel.Agents {
			removed[id] = struct{}{}
		}
		kept := st.InstalledAgents[:0]
		for _, id := range st.InstalledAgents {
			if _, drop := removed[id]; drop {
				res.AgentsRemoved = append(res.AgentsRemoved, id)
				continue
			}
			kept = append(kept, id)
		}
		st.InstalledAgents = kept
	}

	// Strip components.
	if len(sel.Components) > 0 {
		removed := map[model.ComponentID]struct{}{}
		for _, id := range sel.Components {
			removed[id] = struct{}{}
		}
		kept := st.Components[:0]
		for _, id := range st.Components {
			if _, drop := removed[id]; drop {
				continue
			}
			kept = append(kept, id)
		}
		st.Components = kept
	}

	return state.Save(s.homeDir, st)
}
