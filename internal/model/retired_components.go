package model

import "fmt"

// RetiredComponentInfo describes an identifier retained solely for bounded
// legacy decoding and rollback. Retired components are never current choices.
type RetiredComponentInfo struct {
	ID          ComponentID
	RetiredIn   string
	Guidance    string
	Selectable  bool
	Installable bool
}

var retiredComponents = map[ComponentID]RetiredComponentInfo{
	ComponentMailbox: {
		ID:        ComponentMailbox,
		RetiredIn: "redesign-agent-workflows",
		Guidance:  "remove the managed registration during migration or configure a separately qualified external provider",
	},
}

// RetiredComponent returns decode-only metadata for a retired identifier.
func RetiredComponent(id ComponentID) (RetiredComponentInfo, bool) {
	component, ok := retiredComponents[id]
	return component, ok
}

// RetiredComponentError reports an attempt to use a tombstone as current.
type RetiredComponentError struct {
	Component ComponentID
	Guidance  string
}

func (e *RetiredComponentError) Error() string {
	return fmt.Sprintf("component %q is retired and cannot be selected or installed; %s", e.Component, e.Guidance)
}

// ValidateCurrentComponents rejects decode-only tombstones at the shared
// selection boundary used by CLI, configuration, TUI, presets, and repair.
func ValidateCurrentComponents(components []ComponentID) error {
	for _, id := range components {
		if retired, ok := RetiredComponent(id); ok {
			return &RetiredComponentError{Component: id, Guidance: retired.Guidance}
		}
	}
	return nil
}
