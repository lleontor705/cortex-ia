package tui

// Profile management screens: Profiles list, Profile Create, Profile Delete.

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
	if profiles, err := state.LoadProfiles(m.HomeDir); err == nil {
		if profiles == nil {
			profiles = []model.Profile{}
		}
		m.Profiles = profiles
	}
}

// saveProfilesToDisk writes m.Profiles to the persisted file.
func (m *Model) saveProfilesToDisk() {
	m.ProfileErr = state.SaveProfiles(m.HomeDir, m.Profiles)
}

// --- Profiles ---

func (m Model) updateProfiles(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Profiles)-1 {
				m.Cursor++
			}
		case "c":
			m.ProfileErr = nil
			m.ProfileInput.SetValue("")
			m.ProfileInput.Focus()
			m.setScreen(ScreenProfileCreate)
		case "d":
			if m.Cursor < len(m.Profiles) && len(m.Profiles) > 0 {
				m.ProfileErr = nil
				m.ActiveDialog = Dialog{
					Type:    DialogProfileDelete,
					Title:   "Delete Profile",
					Message: "Delete profile " + m.Profiles[m.Cursor].Name + "?",
					Warning: "This action cannot be undone.",
				}
			}
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewProfiles() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Manage Profiles"))
	sb.WriteString("\n\n")

	if m.ProfileErr != nil {
		sb.WriteString(styles.StatusWarn.Render(fmt.Sprintf("Save failed: %v", m.ProfileErr)))
		sb.WriteString("\n\n")
	}

	if len(m.Profiles) == 0 {
		sb.WriteString(styles.Description.Render("No profiles found."))
		sb.WriteString("\n")
	} else {
		for i, p := range m.Profiles {
			cursor := "  "
			if i == m.Cursor {
				cursor = styles.Cursor.Render("> ")
			}
			fmt.Fprintf(&sb, "%s%s\n", cursor, p.Name)
		}
	}

	return sb.String()
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
			m.setScreen(ScreenProfiles)
			return m, nil
		case "esc":
			m.ProfileInput.SetValue("")
			m.ProfileInput.Blur()
			m.setScreen(ScreenProfiles)
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
