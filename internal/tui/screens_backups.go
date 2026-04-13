package tui

// Backup management screens: Backups list, Restore Confirm/Result,
// Delete Confirm/Result, Rename Backup.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// --- Backups screen ---

func (m Model) updateBackups(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Backups)-1 {
				m.Cursor++
			}
		case "r":
			if m.Cursor < len(m.Backups) {
				m.SelectedBackup = m.Backups[m.Cursor]
				m.ActiveDialog = Dialog{
					Type:    DialogRestoreConfirm,
					Title:   "Confirm Restore",
					Message: "Restore backup " + m.SelectedBackup.ID + "?",
					Warning: "This will overwrite current configuration files.",
				}
			}
		case "d":
			if m.Cursor < len(m.Backups) {
				m.SelectedBackup = m.Backups[m.Cursor]
				m.ActiveDialog = Dialog{
					Type:    DialogDeleteConfirm,
					Title:   "Confirm Delete",
					Message: "Delete backup " + m.SelectedBackup.ID + "?",
					Warning: "This action cannot be undone.",
				}
			}
		case "n":
			if m.Cursor < len(m.Backups) {
				m.SelectedBackup = m.Backups[m.Cursor]
				m.BackupRenameInput.SetValue(m.SelectedBackup.Description)
				m.BackupRenameInput.Focus()
				m.RenameErr = nil
				m.setScreen(ScreenRenameBackup)
			}
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewBackups() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Manage Backups"))
	sb.WriteString("\n\n")

	if m.RenameErr != nil {
		sb.WriteString(styles.StatusWarn.Render(fmt.Sprintf("Rename failed: %v", m.RenameErr)))
		sb.WriteString("\n\n")
	}

	if len(m.Backups) == 0 {
		sb.WriteString(styles.Description.Render("No backups found."))
		sb.WriteString("\n")
	} else {
		for i, bk := range m.Backups {
			cursor := "  "
			if i == m.Cursor {
				cursor = styles.Cursor.Render("> ")
			}
			desc := bk.Description
			if desc == "" {
				desc = bk.ID
			}
			fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render(bk.ID), styles.Description.Render(desc))
		}
	}

	if len(m.BackupWarnings) > 0 {
		sb.WriteString("\n")
		for _, w := range m.BackupWarnings {
			fmt.Fprintf(&sb, "%s\n", styles.StatusWarn.Render("⚠ "+w))
		}
	}

	return sb.String()
}

// --- Restore Confirm ---

func (m Model) updateRestoreConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "enter":
			m.OperationRunning = true
			return m, func() tea.Msg {
				if m.RestoreFn != nil {
					err := m.RestoreFn(m.SelectedBackup)
					return BackupRestoreMsg{Err: err}
				}
				return BackupRestoreMsg{Err: fmt.Errorf("restore not available")}
			}
		case "n", "esc":
			m.setScreen(ScreenBackups)
		}
	}
	return m, nil
}

func (m Model) viewRestoreConfirm() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Confirm Restore"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Restore backup %s?\n", styles.Subtitle.Render(m.SelectedBackup.ID))
	sb.WriteString(styles.StatusWarn.Render("This will overwrite current configuration files."))
	sb.WriteString("\n")
	return sb.String()
}

// --- Restore Result ---

func (m Model) updateRestoreResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "esc":
			m.setScreen(ScreenBackups)
		}
	}
	return m, nil
}

func (m Model) viewRestoreResult() string {
	var sb strings.Builder
	if m.RestoreErr != nil {
		sb.WriteString(styles.StatusFail.Render("Restore Failed"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.RestoreErr)
	} else {
		sb.WriteString(styles.StatusOK.Render("✓ Restore Complete"))
		sb.WriteString("\n\n")
		sb.WriteString("Configuration files have been restored.\n")
	}
	return sb.String()
}

// --- Delete Confirm ---

func (m Model) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "enter":
			m.OperationRunning = true
			return m, func() tea.Msg {
				if m.DeleteBackupFn != nil {
					err := m.DeleteBackupFn(m.SelectedBackup)
					return BackupDeleteMsg{Err: err}
				}
				return BackupDeleteMsg{Err: fmt.Errorf("delete not available")}
			}
		case "n", "esc":
			m.setScreen(ScreenBackups)
		}
	}
	return m, nil
}

func (m Model) viewDeleteConfirm() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Confirm Delete"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Delete backup %s?\n", styles.Subtitle.Render(m.SelectedBackup.ID))
	sb.WriteString(styles.StatusFail.Render("This action cannot be undone."))
	sb.WriteString("\n")
	return sb.String()
}

// --- Delete Result ---

func (m Model) updateDeleteResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "esc":
			m.setScreen(ScreenBackups)
		}
	}
	return m, nil
}

func (m Model) viewDeleteResult() string {
	var sb strings.Builder
	if m.DeleteErr != nil {
		sb.WriteString(styles.StatusFail.Render("Delete Failed"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.DeleteErr)
	} else {
		sb.WriteString(styles.StatusOK.Render("✓ Backup Deleted"))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// --- Rename Backup ---

func (m Model) updateRenameBackup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.BackupRenameInput.Value()
			if m.RenameBackupFn != nil && val != "" {
				if err := m.RenameBackupFn(m.SelectedBackup, val); err != nil {
					m.RenameErr = err
				} else {
					m.RenameErr = nil
				}
				if m.ListBackupsFn != nil {
					m.Backups, m.BackupWarnings = m.ListBackupsFn()
				}
			}
			m.BackupRenameInput.Blur()
			m.setScreen(ScreenBackups)
			return m, nil
		case "esc":
			m.BackupRenameInput.Blur()
			m.setScreen(ScreenBackups)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.BackupRenameInput, cmd = m.BackupRenameInput.Update(msg)
	return m, cmd
}

func (m Model) viewRenameBackup() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Rename Backup"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Backup: %s\n\n", styles.Subtitle.Render(m.SelectedBackup.ID))
	sb.WriteString("Description:\n")
	sb.WriteString(m.BackupRenameInput.View())
	sb.WriteString("\n")
	return sb.String()
}
