package tui

import (
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// loadProfilesFromDisk refreshes compatibility-only profile state. It is not a
// public TUI profile route; stale selections are rejected at the TUI boundary.
func (m *Model) loadProfilesFromDisk() {
	if profiles, err := state.LoadProfiles(m.HomeDir); err == nil {
		if profiles == nil {
			profiles = []model.Profile{}
		}
		m.Profiles = profiles
	}
}
