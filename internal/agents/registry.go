package agents

import "github.com/lleontor705/cortex-ia/internal/model"

// Registry holds all registered agent adapters.
type Registry struct {
	adapters map[model.AgentID]Adapter
	order    []model.AgentID
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[model.AgentID]Adapter),
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(a Adapter) {
	id := a.Agent()
	if _, exists := r.adapters[id]; !exists {
		r.order = append(r.order, id)
	}
	r.adapters[id] = a
}

// Get returns an adapter by agent ID.
func (r *Registry) Get(id model.AgentID) (Adapter, error) {
	a, ok := r.adapters[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return a, nil
}

// All returns all adapters in registration order.
func (r *Registry) All() []Adapter {
	result := make([]Adapter, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.adapters[id])
	}
	return result
}

// IDs returns all registered agent IDs in order.
func (r *Registry) IDs() []model.AgentID {
	return append([]model.AgentID(nil), r.order...)
}
