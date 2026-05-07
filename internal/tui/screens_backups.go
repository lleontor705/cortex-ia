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

// visibleBackups returns indices of backups matching the current filter.
func (m Model) visibleBackups() []int {
	var indices []int
	for i, bk := range m.Backups {
		text := bk.ID + " " + bk.Description
		if m.BackupFilter.Matches(text) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m Model) updateBackups(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When filter is active, delegate to filter input
	if m.BackupFilter.Active {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.BackupFilter.Deactivate()
				return m, nil
			case "enter":
				m.BackupFilter.Deactivate()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.BackupFilter.Input, cmd = m.BackupFilter.Input.Update(msg)
		visible := m.visibleBackups()
		if m.Cursor >= len(visible) {
			m.Cursor = max(len(visible)-1, 0)
		}
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "/"  :
			m.BackupFilter.Activate()
			return m, m.BackupFilter.Input.Focus()
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			visible := m.visibleBackups()
			if m.Cursor < len(visible)-1 {
				m.Cursor++
			}
		case "r":
			visible := m.visibleBackups()
			if m.Cursor < len(visible) {
				m.SelectedBackup = m.Backups[visible[m.Cursor]]
				m.ActiveDialog = Dialog{
					Type:    DialogRestoreConfirm,
					Title:   "Confirm Restore",
					Message: "Restore backup " + m.SelectedBackup.ID + "?",
					Warning: "This will overwrite current configuration files.",
				}
			}
		case "d":
			visible := m.visibleBackups()
			if m.Cursor < len(visible) {
				m.SelectedBackup = m.Backups[visible[m.Cursor]]
				m.ActiveDialog = Dialog{
					Type:    DialogDeleteConfirm,
					Title:   "Confirm Delete",
					Message: "Delete backup " + m.SelectedBackup.ID + "?",
					Warning: "This action cannot be undone.",
				}
			}
		case "n":
			visible := m.visibleBackups()
			if m.Cursor < len(visible) {
				m.SelectedBackup = m.Backups[visible[m.Cursor]]
				m.BackupRenameInput.SetValue(m.SelectedBackup.Description)
				m.BackupRenameInput.Focus()
				m.RenameErr = nil
				m.setScreenKeepCursor(ScreenRenameBackup)
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

	if m.BackupFilter.Active || m.BackupFilter.Query() != "" {
		sb.WriteString(m.BackupFilter.View())
	}

	visible := m.visibleBackups()
	if len(m.Backups) == 0 {
		sb.WriteString(styles.Description.Render("No backups found."))
		sb.WriteString("\n")
	} else if len(visible) == 0 && m.BackupFilter.Query() != "" {
		sb.WriteString(styles.Description.Render("No matching backups."))
		sb.WriteString("\n")
	} else {
		for i, idx := range visible {
			bk := m.Backups[idx]
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

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("r restore • d delete • n rename • / filter • Esc back"))

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
			m.restoreScreen(ScreenBackups)
			return m, nil
		case "esc":
			m.BackupRenameInput.Blur()
			m.restoreScreen(ScreenBackups)
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
