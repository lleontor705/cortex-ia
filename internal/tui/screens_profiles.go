package tui

// Profile management: Profile Create screen.
// Profiles list is now part of the Maintenance screen (Profiles tab).

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// loadProfilesFromDisk refreshes m.Profiles from the persisted file.
func (m *Model) loadProfilesFromDisk() {
	profiles, err := state.LoadProfiles(m.HomeDir)
	if err != nil {
		m.ProfileErr = err
		m.Profiles = []model.Profile{}
		return
	}
	if profiles == nil {
		profiles = []model.Profile{}
	}
	m.Profiles = profiles
	m.ProfileErr = nil
}

// saveProfilesToDisk writes m.Profiles to the persisted file.
func (m *Model) saveProfilesToDisk() {
	m.ProfileErr = state.SaveProfiles(m.HomeDir, m.Profiles)
}

// --- Profile Create ---

func (m Model) updateProfileCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.ProfileInput.Value()
			if val != "" {
				// Check for duplicate names
				duplicate := false
				for _, p := range m.Profiles {
					if p.Name == val {
						duplicate = true
						break
					}
				}
				if duplicate {
					m.ProfileErr = fmt.Errorf("profile %q already exists", val)
				} else {
					m.Profiles = append(m.Profiles, model.Profile{Name: val})
					m.saveProfilesToDisk()
					m.ProfileInput.SetValue("")
				}
			}
			m.ProfileInput.Blur()
			m.setScreen(ScreenMaintenance)
			return m, nil
		case "esc":
			m.ProfileInput.SetValue("")
			m.ProfileInput.Blur()
			m.setScreen(ScreenMaintenance)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.ProfileInput, cmd = m.ProfileInput.Update(msg)
	return m, cmd
}

func (m Model) viewProfileCreate() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Profile"))
	sb.WriteString("\n\n")
	sb.WriteString("Profile name:\n")
	sb.WriteString(m.ProfileInput.View())
	sb.WriteString("\n")
	return sb.String()
}

// Profile deletion is handled by the dialog system in tui.go (DialogProfileDelete).
