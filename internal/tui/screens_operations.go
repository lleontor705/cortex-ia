package tui

// Operation screens: Upgrade, Sync, Upgrade+Sync, Model Config.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
	"github.com/lleontor705/cortex-ia/internal/update"
)

// --- Upgrade ---

func (m Model) updateUpgrade(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "r":
			if !m.OperationRunning {
				m.OperationRunning = true
				m.UpdateCheckDone = false
				return m, tea.Batch(func() tea.Msg {
					result := update.Check(m.Version)
					return UpdateCheckResultMsg{Results: []update.CheckResult{result}}
				})
			}
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewUpgrade() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Check for Updates"))
	sb.WriteString("\n\n")

	if m.OperationRunning {
		fmt.Fprintf(&sb, "%s Checking for updates...\n", m.Spinner.View())
	} else if !m.UpdateCheckDone {
		sb.WriteString("Press r to check for updates.\n")
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
		} else {
			sb.WriteString(styles.StatusWarn.Render("Update available"))
			sb.WriteString("\n\n")
			fmt.Fprintf(&sb, "  Current: %s\n", r.CurrentVersion)
			fmt.Fprintf(&sb, "  Latest:  %s\n", r.LatestRelease.TagName)
			if r.LatestRelease.HTMLURL != "" {
				fmt.Fprintf(&sb, "  Release: %s\n", r.LatestRelease.HTMLURL)
			}
			sb.WriteString("\n")
			sb.WriteString("To upgrade, run:\n")
			sb.WriteString(styles.Subtitle.Render("  curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash"))
			sb.WriteString("\n")
		}
	}
	// help rendered centrally
	return sb.String()
}

// --- Sync ---

func (m Model) updateSync(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.SyncFn != nil && !m.OperationRunning {
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
				// Move to previous profile (or clear)
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
				// Move to next profile (or clear)
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
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewSync() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Sync Configuration"))
	sb.WriteString("\n\n")

	// Profile selector
	sb.WriteString(styles.Subtitle.Render("Profile:"))
	sb.WriteString("\n")
	// "(none)" option
	noneMarker := "( )"
	noneCursor := "  "
	if m.SelectedProfile == "" {
		noneMarker = styles.Selected.Render("(*)")
		noneCursor = styles.Cursor.Render("> ")
	}
	fmt.Fprintf(&sb, "%s%s %s\n", noneCursor, noneMarker, styles.Description.Render("(none)"))
	// Profile options
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
	// help rendered centrally
	return sb.String()
}

// --- Upgrade + Sync ---

func (m Model) updateUpgradeSync(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if !m.OperationRunning {
				m.OperationRunning = true
				m.UpgradeSyncPhase = "checking"
				return m, tea.Batch(func() tea.Msg {
					result := update.Check(m.Version)
					return UpdateCheckResultMsg{Results: []update.CheckResult{result}}
				})
			}
		case "esc":
			if !m.OperationRunning {
				m.UpgradeSyncPhase = ""
				m.setScreen(ScreenWelcome)
			}
		}
	}
	return m, nil
}

func (m Model) viewUpgradeSync() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Upgrade + Sync"))
	sb.WriteString("\n\n")

	switch {
	case m.UpgradeSyncPhase == "" && !m.OperationRunning:
		sb.WriteString("This will check for updates and then sync your configuration.\n\n")
		if m.SelectedProfile != "" {
			fmt.Fprintf(&sb, "Profile: %s\n\n", styles.Subtitle.Render(m.SelectedProfile))
		}
		sb.WriteString("Press Enter to start.\n")

	case m.UpgradeSyncPhase == "checking":
		fmt.Fprintf(&sb, "%s Checking for updates...\n", m.Spinner.View())

	case m.UpgradeSyncPhase == "syncing":
		if len(m.UpdateResults) > 0 {
			r := m.UpdateResults[0]
			switch {
			case r.Error != nil:
				sb.WriteString(styles.StatusWarn.Render("Update check failed"))
				fmt.Fprintf(&sb, " (%v)\n", r.Error)
			case r.UpToDate:
				sb.WriteString(styles.StatusOK.Render("✓ Up to date"))
				fmt.Fprintf(&sb, " (%s)\n", r.CurrentVersion)
			default:
				sb.WriteString(styles.StatusWarn.Render("Update available"))
				fmt.Fprintf(&sb, " (%s → %s)\n", r.CurrentVersion, r.LatestRelease.TagName)
			}
		}
		fmt.Fprintf(&sb, "\n%s Syncing configuration...\n", m.Spinner.View())

	case m.UpgradeSyncPhase == "done":
		if len(m.UpdateResults) > 0 {
			r := m.UpdateResults[0]
			switch {
			case r.Error != nil:
				sb.WriteString(styles.StatusWarn.Render("Update check failed"))
				fmt.Fprintf(&sb, " (%v)\n", r.Error)
			case r.UpToDate:
				sb.WriteString(styles.StatusOK.Render("✓ Up to date"))
				fmt.Fprintf(&sb, " (%s)\n", r.CurrentVersion)
			default:
				sb.WriteString(styles.StatusWarn.Render("Update available"))
				fmt.Fprintf(&sb, " (%s → %s)\n", r.CurrentVersion, r.LatestRelease.TagName)
			}
		}
		sb.WriteString("\n")
		if m.SyncErr != nil {
			sb.WriteString(styles.StatusFail.Render("Sync failed"))
			fmt.Fprintf(&sb, "\nError: %v\n", m.SyncErr)
		} else {
			sb.WriteString(styles.StatusOK.Render("✓ Sync complete"))
			fmt.Fprintf(&sb, "\nFiles changed: %d\n", m.SyncFilesChanged)
		}
	}

	// help rendered centrally
	return sb.String()
}

// --- Model Config ---

func (m Model) updateModelConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.ModelConfigMode = true
			m.setScreen(ScreenClaudeModelPicker)
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewModelConfig() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Configure Models"))
	sb.WriteString("\n\n")
	sb.WriteString("Adjust the AI model used for each SDD phase.\n\n")
	sb.WriteString("Current preset: " + styles.Subtitle.Render(string(m.ModelPreset)) + "\n")
	// help rendered centrally
	return sb.String()
}
