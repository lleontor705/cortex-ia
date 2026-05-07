package agentbuilder

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// builtinSkills returns the set of built-in skill names so conflict detection
// stays in sync with the cortex-ia catalog.
func builtinSkills() map[string]struct{} {
	skills := catalog.AllSDDSkillIDs()
	m := make(map[string]struct{}, len(skills))
	for _, id := range skills {
		m[string(id)] = struct{}{}
	}
	// Plus the judgment-day skill which lives in assets but isn't in AllSDDSkillIDs yet.
	m[string(model.SkillJudgmentDay)] = struct{}{}
	return m
}

// LoadRegistry reads the registry JSON from path. Missing file ⇒ empty registry.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{Version: 1, Agents: []RegistryEntry{}}, nil
		}
		return nil, err
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.Agents == nil {
		reg.Agents = []RegistryEntry{}
	}
	return &reg, nil
}

// SaveRegistry writes reg to path as indented JSON. Caller must ensure the
// parent directory exists (use state.AgentBuilderRegistryPath which does).
func SaveRegistry(path string, reg *Registry) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Add appends entry to the registry.
func (r *Registry) Add(entry RegistryEntry) {
	r.Agents = append(r.Agents, entry)
}

// FindByName returns the first RegistryEntry whose Name matches, or nil.
func (r *Registry) FindByName(name string) *RegistryEntry {
	for i := range r.Agents {
		if r.Agents[i].Name == name {
			return &r.Agents[i]
		}
	}
	return nil
}

// RemoveByName removes the first entry matching name. Returns true on hit.
func (r *Registry) RemoveByName(name string) bool {
	for i, entry := range r.Agents {
		if entry.Name == name {
			r.Agents = append(r.Agents[:i], r.Agents[i+1:]...)
			return true
		}
	}
	return false
}

// HasConflictWithBuiltin reports whether name collides with a known cortex-ia
// built-in skill (SDD or judgment-day). Custom skills must not shadow these.
func HasConflictWithBuiltin(name string) bool {
	_, ok := builtinSkills()[name]
	return ok
}
