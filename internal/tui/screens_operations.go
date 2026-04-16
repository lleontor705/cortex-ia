package tui

// Maintenance screen: unified Upgrade, Sync, and Profiles in a single tabbed view.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
	"github.com/lleontor705/cortex-ia/internal/update"
)

// renderTabBar renders a horizontal tab bar with the active tab highlighted.
func renderTabBar(labels []string, activeIdx int) string {
	var sb strings.Builder
	for i, label := range labels {
		if i > 0 {
			sb.WriteString(styles.Description.Render(" │ "))
		}
		if i == activeIdx {
			sb.WriteString(styles.Subtitle.Render("[" + label + "]"))
		} else {
			sb.WriteString(styles.Description.Render(" " + label + " "))
		}
	}
	return sb.String()
}

// --- Maintenance screen (tabbed: Upgrade | Sync | Profiles) ---

func (m Model) updateMaintenance(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", "right", "l":
			m.MaintenanceTab = (m.MaintenanceTab + 1) % 2
			m.Cursor = 0
			return m, nil
		case "shift+tab", "left", "h":
			m.MaintenanceTab = (m.MaintenanceTab + 1) % 2 // wraps backward (2 tabs)
			m.Cursor = 0
			return m, nil
		case "esc":
			m.setScreen(ScreenWelcome)
			return m, nil
		}

		// Delegate to the active tab
		switch m.MaintenanceTab {
		case MaintenanceTabUpgrade:
			return m.updateMaintenanceUpgrade(msg)
		case MaintenanceTabSync:
			return m.updateMaintenanceSync(msg)
		}
	}
	return m, nil
}

func (m Model) viewMaintenance() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Maintenance"))
	sb.WriteString("\n\n")

	sb.WriteString(renderTabBar([]string{"Upgrade", "Sync"}, int(m.MaintenanceTab)))
	sb.WriteString("\n\n")

	switch m.MaintenanceTab {
	case MaintenanceTabUpgrade:
		sb.WriteString(m.viewMaintenanceUpgrade())
	case MaintenanceTabSync:
		sb.WriteString(m.viewMaintenanceSync())
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Tab switch • Esc back"))
	return sb.String()
}

// --- Upgrade tab ---

func (m Model) updateMaintenanceUpgrade(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "r", "enter":
			if !m.OperationRunning {
				m.OperationRunning = true
				m.UpdateCheckDone = false
				return m, tea.Batch(func() tea.Msg {
					result := update.Check(m.Version)
					return UpdateCheckResultMsg{Results: []update.CheckResult{result}}
				})
			}
		case "x":
			// Repair ecosystem
			if m.RepairFn != nil && !m.OperationRunning {
				m.OperationRunning = true
				return m, func() tea.Msg {
					result, err := m.RepairFn()
					return RepairDoneMsg{Result: result, Err: err}
				}
			}
		case "u":
			// Auto-upgrade
			if m.UpdateCheckDone && !m.OperationRunning && len(m.UpdateResults) > 0 {
				r := m.UpdateResults[0]
				if r.Error == nil && !r.UpToDate && m.UpgradeFn != nil {
					m.OperationRunning = true
					results := m.UpdateResults
					return m, func() tea.Msg {
						err := m.UpgradeFn(results)
						return UpgradeDoneMsg{Err: err}
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) viewMaintenanceUpgrade() string {
	var sb strings.Builder

	if m.UpgradeErr != nil {
		sb.WriteString(styles.StatusFail.Render("Upgrade failed"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.UpgradeErr)
		sb.WriteString("\n")
		sb.WriteString("To upgrade manually, run:\n")
		sb.WriteString(styles.Subtitle.Render("  curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash"))
		sb.WriteString("\n")
	} else if m.OperationRunning {
		fmt.Fprintf(&sb, "%s Checking for updates...\n", m.Spinner.View())
	} else if !m.UpdateCheckDone {
		sb.WriteString("Press Enter to check for updates.\n")
	} else if len(m.UpdateResults) > 0 {
		r := m.UpdateResults[0]
		if r.Error != nil {
			sb.WriteString(styles.StatusFail.Render("Check failed"))
			sb.WriteString("\n")
			fmt.Fprintf(&sb, "Error: %v\n", r.Error)
		} else if r.UpToDate {
			sb.WriteString(styles.StatusOK.Render("✓ Up to date"))
			sb.WriteString("\n\n")
			fmt.Fprintf(&sb, "cortex-ia %s is the latest version.\n", r.CurrentVersion)
			if m.RepairFn != nil {
				sb.WriteString("\n")
				sb.WriteString(styles.Description.Render("Press x to repair/re-sync ecosystem files"))
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString(styles.StatusWarn.Render("Update available"))
			sb.WriteString("\n\n")
			fmt.Fprintf(&sb, "  Current: %s\n", r.CurrentVersion)
			fmt.Fprintf(&sb, "  Latest:  %s\n", r.LatestRelease.TagName)
			if r.LatestRelease.HTMLURL != "" {
				fmt.Fprintf(&sb, "  Release: %s\n", r.LatestRelease.HTMLURL)
			}
			sb.WriteString("\n")
			if m.UpgradeFn != nil {
				sb.WriteString(styles.Subtitle.Render("Press u to upgrade automatically"))
				sb.WriteString("\n")
			} else {
				sb.WriteString("To upgrade, run:\n")
				sb.WriteString(styles.Subtitle.Render("  curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash"))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// --- Sync tab ---

func (m Model) updateMaintenanceSync(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "c":
			// Create profile inline
			m.ProfileErr = nil
			m.ProfileInput.SetValue("")
			m.ProfileInput.Focus()
			m.setScreen(ScreenProfileCreate)
			return m, nil
		case "d":
			// Delete selected profile
			if m.SelectedProfile != "" {
				m.ProfileErr = nil
				m.ActiveDialog = Dialog{
					Type:    DialogProfileDelete,
					Title:   "Delete Profile",
					Message: "Delete profile " + m.SelectedProfile + "?",
					Warning: "This action cannot be undone.",
				}
			}
			return m, nil
		case "enter":
			if m.SyncFn == nil {
				m.SyncErr = fmt.Errorf("sync not available")
				return m, nil
			}
			if !m.OperationRunning {
				m.OperationRunning = true
				profileName := m.SelectedProfile
				return m, tea.Batch(func() tea.Msg {
					changed, err := m.SyncFn(profileName)
					return SyncDoneMsg{FilesChanged: changed, Err: err}
				})
			}
		case "up", "k":
			if !m.OperationRunning {
				if len(m.Profiles) == 0 {
					m.loadProfilesFromDisk()
				}
				if m.SelectedProfile == "" && len(m.Profiles) > 0 {
					m.SelectedProfile = m.Profiles[len(m.Profiles)-1].Name
				} else {
					for i, p := range m.Profiles {
						if p.Name == m.SelectedProfile {
							if i > 0 {
								m.SelectedProfile = m.Profiles[i-1].Name
							} else {
								m.SelectedProfile = ""
							}
							break
						}
					}
				}
			}
		case "down", "j":
			if !m.OperationRunning {
				if len(m.Profiles) == 0 {
					m.loadProfilesFromDisk()
				}
				if m.SelectedProfile == "" && len(m.Profiles) > 0 {
					m.SelectedProfile = m.Profiles[0].Name
				} else {
					for i, p := range m.Profiles {
						if p.Name == m.SelectedProfile {
							if i+1 < len(m.Profiles) {
								m.SelectedProfile = m.Profiles[i+1].Name
							} else {
								m.SelectedProfile = ""
							}
							break
						}
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) viewMaintenanceSync() string {
	var sb strings.Builder

	// Profile selector
	sb.WriteString(styles.Subtitle.Render("Profile:"))
	sb.WriteString("\n")
	noneMarker := "( )"
	noneCursor := "  "
	if m.SelectedProfile == "" {
		noneMarker = styles.Selected.Render("(*)")
		noneCursor = styles.Cursor.Render("> ")
	}
	fmt.Fprintf(&sb, "%s%s %s\n", noneCursor, noneMarker, styles.Description.Render("(none)"))
	for _, p := range m.Profiles {
		marker := "( )"
		cursor := "  "
		if p.Name == m.SelectedProfile {
			marker = styles.Selected.Render("(*)")
			cursor = styles.Cursor.Render("> ")
		}
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, marker, p.Name)
	}
	sb.WriteString("\n")

	if m.OperationRunning {
		fmt.Fprintf(&sb, "%s Syncing managed files...\n", m.Spinner.View())
	} else if m.SyncErr != nil {
		sb.WriteString(styles.StatusFail.Render("Sync failed"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.SyncErr)
	} else if m.SyncFilesChanged > 0 {
		sb.WriteString(styles.StatusOK.Render("✓ Sync Complete"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Files changed: %d\n", m.SyncFilesChanged)
	} else {
		sb.WriteString("Press Enter to sync managed configuration files.\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Enter sync • c create profile • d delete profile"))
	return sb.String()
}
