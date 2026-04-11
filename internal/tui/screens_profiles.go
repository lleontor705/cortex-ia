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
			m.setScreen(ScreenProfileCreate)
		case "d":
			if m.Cursor < len(m.Profiles) && len(m.Profiles) > 0 {
				m.ProfileErr = nil
				m.setScreen(ScreenProfileDelete)
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

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • c create • d delete • Esc back"))
	return sb.String()
}

// --- Profile Create ---

func (m Model) updateProfileCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.ProfileNameInput != "" {
				m.Profiles = append(m.Profiles, model.Profile{Name: m.ProfileNameInput})
				m.saveProfilesToDisk()
				m.ProfileNameInput = ""
				m.ProfileNamePos = 0
			}
			m.setScreen(ScreenProfiles)
		case "esc":
			m.ProfileNameInput = ""
			m.ProfileNamePos = 0
			m.setScreen(ScreenProfiles)
		case "backspace":
			m.ProfileNameInput, m.ProfileNamePos = textBackspace(m.ProfileNameInput, m.ProfileNamePos)
		case "delete":
			m.ProfileNameInput = textDelete(m.ProfileNameInput, m.ProfileNamePos)
		case "left":
			if m.ProfileNamePos > 0 {
				m.ProfileNamePos--
			}
		case "right":
			m.ProfileNamePos = clampPos(m.ProfileNameInput, m.ProfileNamePos+1)
		case "home", "ctrl+a":
			m.ProfileNamePos = 0
		case "end", "ctrl+e":
			m.ProfileNamePos = len([]rune(m.ProfileNameInput))
		default:
			if len(key.String()) == 1 {
				m.ProfileNameInput, m.ProfileNamePos = textInsert(m.ProfileNameInput, m.ProfileNamePos, key.String())
			}
		}
	}
	return m, nil
}

func (m Model) viewProfileCreate() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Profile"))
	sb.WriteString("\n\n")
	sb.WriteString("Profile name: ")
	sb.WriteString(styles.Box.Render(textRenderWithCursor(m.ProfileNameInput, m.ProfileNamePos)))
	sb.WriteString("\n")
	sb.WriteString(styles.Help.Render("\nEnter to create • ←→ move cursor • Esc to cancel"))
	return sb.String()
}

// --- Profile Delete ---

func (m Model) updateProfileDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "enter":
			if m.Cursor < len(m.Profiles) {
				m.Profiles = append(m.Profiles[:m.Cursor], m.Profiles[m.Cursor+1:]...)
				m.saveProfilesToDisk()
			}
			m.setScreen(ScreenProfiles)
		case "n", "esc":
			m.setScreen(ScreenProfiles)
		}
	}
	return m, nil
}

func (m Model) viewProfileDelete() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Delete Profile"))
	sb.WriteString("\n\n")
	if m.Cursor < len(m.Profiles) {
		fmt.Fprintf(&sb, "Delete profile %s?\n", styles.StatusFail.Render(m.Profiles[m.Cursor].Name))
	}
	sb.WriteString(styles.Help.Render("\ny to confirm • n/Esc to cancel"))
	return sb.String()
}
