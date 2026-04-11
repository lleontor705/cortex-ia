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
				m.setScreen(ScreenRestoreConfirm)
			}
		case "d":
			if m.Cursor < len(m.Backups) {
				m.SelectedBackup = m.Backups[m.Cursor]
				m.setScreen(ScreenDeleteConfirm)
			}
		case "n":
			if m.Cursor < len(m.Backups) {
				m.SelectedBackup = m.Backups[m.Cursor]
				m.BackupRenameText = m.SelectedBackup.Description
				m.BackupRenamePos = len(m.BackupRenameText)
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

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • r restore • d delete • n rename • Esc back"))
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
	sb.WriteString(styles.Help.Render("\ny to confirm • n/Esc to cancel"))
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
	sb.WriteString(styles.Help.Render("\nPress Enter to continue"))
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
	sb.WriteString(styles.Help.Render("\ny to confirm • n/Esc to cancel"))
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
	sb.WriteString(styles.Help.Render("\nPress Enter to continue"))
	return sb.String()
}

// --- Rename Backup ---

func (m Model) updateRenameBackup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.RenameBackupFn != nil && m.BackupRenameText != "" {
				if err := m.RenameBackupFn(m.SelectedBackup, m.BackupRenameText); err != nil {
					m.RenameErr = err
				} else {
					m.RenameErr = nil
				}
				if m.ListBackupsFn != nil {
					m.Backups, m.BackupWarnings = m.ListBackupsFn()
				}
			}
			m.setScreen(ScreenBackups)
		case "esc":
			m.setScreen(ScreenBackups)
		case "backspace":
			m.BackupRenameText, m.BackupRenamePos = textBackspace(m.BackupRenameText, m.BackupRenamePos)
		case "delete":
			m.BackupRenameText = textDelete(m.BackupRenameText, m.BackupRenamePos)
		case "left":
			if m.BackupRenamePos > 0 {
				m.BackupRenamePos--
			}
		case "right":
			m.BackupRenamePos = clampPos(m.BackupRenameText, m.BackupRenamePos+1)
		case "home", "ctrl+a":
			m.BackupRenamePos = 0
		case "end", "ctrl+e":
			m.BackupRenamePos = len([]rune(m.BackupRenameText))
		default:
			if len(key.String()) == 1 {
				m.BackupRenameText, m.BackupRenamePos = textInsert(m.BackupRenameText, m.BackupRenamePos, key.String())
			}
		}
	}
	return m, nil
}

func (m Model) viewRenameBackup() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Rename Backup"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Backup: %s\n\n", styles.Subtitle.Render(m.SelectedBackup.ID))
	sb.WriteString("Description: ")
	sb.WriteString(styles.Box.Render(textRenderWithCursor(m.BackupRenameText, m.BackupRenamePos)))
	sb.WriteString("\n")
	sb.WriteString(styles.Help.Render("\nEnter to save • ←→ move cursor • Esc to cancel"))
	return sb.String()
}
