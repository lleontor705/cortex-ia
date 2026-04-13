package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines key bindings for the TUI with integrated help text.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Esc      key.Binding
	Quit     key.Binding
	Space    key.Binding
	All      key.Binding
	Help     key.Binding
	Restore key.Binding
	Delete  key.Binding
	Rename  key.Binding
	Create  key.Binding
	Profile key.Binding
	Filter  key.Binding

	// ShowFullHelp toggles the full help view.
	ShowFullHelp bool
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		All: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Restore: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restore"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Rename: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "rename"),
		),
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create"),
		),
		Profile: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "profile"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

// --- Help KeyMap interfaces for different screen contexts ---

// NavigateKeyMap is used for screens with simple up/down/enter/esc navigation.
type NavigateKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Esc   key.Binding
	Help  key.Binding
}

func (k NavigateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Esc}
}

func (k NavigateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Enter, k.Esc, k.Help}}
}

// CheckboxKeyMap is used for screens with checkbox selection.
type CheckboxKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Space  key.Binding
	All    key.Binding
	Filter key.Binding
	Enter  key.Binding
	Esc    key.Binding
	Help   key.Binding
}

func (k CheckboxKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Space, k.Filter, k.Enter, k.Esc}
}

func (k CheckboxKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Space, k.All, k.Filter, k.Enter, k.Esc, k.Help}}
}

// BackupKeyMap is used for the backup management screen.
type BackupKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Restore key.Binding
	Delete  key.Binding
	Rename  key.Binding
	Esc     key.Binding
	Help    key.Binding
}

func (k BackupKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Restore, k.Delete, k.Rename, k.Esc}
}

func (k BackupKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Restore, k.Delete, k.Rename, k.Esc, k.Help}}
}

// InputKeyMap is used for text input screens.
type InputKeyMap struct {
	Enter key.Binding
	Esc   key.Binding
}

func (k InputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Esc}
}

func (k InputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Enter, k.Esc}}
}

// ProfileKeyMap is used for the profile management screen.
type ProfileKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Create key.Binding
	Delete key.Binding
	Esc    key.Binding
	Help   key.Binding
}

func (k ProfileKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Create, k.Delete, k.Esc}
}

func (k ProfileKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Create, k.Delete, k.Esc, k.Help}}
}

// WelcomeKeyMap is used for the welcome screen.
type WelcomeKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Quit  key.Binding
	Help  key.Binding
}

func (k WelcomeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Quit}
}

func (k WelcomeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Enter, k.Quit, k.Help}}
}

// --- Helper to get the appropriate keymap for a screen ---

func (m Model) screenKeyMap() help.KeyMap {
	switch m.Screen {
	case ScreenWelcome:
		return WelcomeKeyMap{Up: m.Keys.Up, Down: m.Keys.Down, Enter: m.Keys.Enter, Quit: m.Keys.Quit, Help: m.Keys.Help}
	case ScreenAgents, ScreenSkillPicker:
		return CheckboxKeyMap{Up: m.Keys.Up, Down: m.Keys.Down, Space: m.Keys.Space, All: m.Keys.All, Filter: m.Keys.Filter, Enter: m.Keys.Enter, Esc: m.Keys.Esc, Help: m.Keys.Help}
	case ScreenBackups:
		return BackupKeyMap{Up: m.Keys.Up, Down: m.Keys.Down, Restore: m.Keys.Restore, Delete: m.Keys.Delete, Rename: m.Keys.Rename, Esc: m.Keys.Esc, Help: m.Keys.Help}
	case ScreenProfiles:
		return ProfileKeyMap{Up: m.Keys.Up, Down: m.Keys.Down, Create: m.Keys.Create, Delete: m.Keys.Delete, Esc: m.Keys.Esc, Help: m.Keys.Help}
	case ScreenAgentBuilderPrompt:
		return InputKeyMap{
			Enter: key.NewBinding(key.WithKeys("ctrl+enter"), key.WithHelp("ctrl+enter", "confirm")),
			Esc:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	case ScreenRenameBackup, ScreenProfileCreate:
		return InputKeyMap{
			Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
			Esc:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	default:
		return NavigateKeyMap{Up: m.Keys.Up, Down: m.Keys.Down, Enter: m.Keys.Enter, Esc: m.Keys.Esc, Help: m.Keys.Help}
	}
}
