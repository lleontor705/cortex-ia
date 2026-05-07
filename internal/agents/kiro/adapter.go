// Package kiro provides Kiro IDE agent integration.
package kiro

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type Adapter struct {
	lookPath func(string) (string, error)
	statPath func(string) (os.FileInfo, error)
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		statPath: os.Stat,
	}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID    { return model.AgentKiroIDE }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

// --- Detection ---

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := filepath.Join(homeDir, ".kiro")

	binaryPath, err := a.lookPath("kiro")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, "", configPath, false, nil
		}
		return false, "", configPath, false, err
	}

	info, statErr := a.statPath(configPath)
	configFound := statErr == nil && info.IsDir()

	return true, binaryPath, configPath, configFound, nil
}

// --- Installation ---

func (a *Adapter) SupportsAutoInstall() bool { return false }

// InstallCommands returns nil because Kiro IDE is a desktop app installed
// via official downloads or package managers — not auto-installable.
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

// --- Config paths ---
//
// Kiro IDE (VS Code fork) uses a split-root layout:
//   - Steering/skills/agents/MCP: ~/.kiro/ (home-based, all platforms)
//   - Settings: macOS: ~/Library/Application Support/Kiro/User/
//               Linux: ~/.config/kiro/user/ (respects XDG_CONFIG_HOME)
//               Windows: %APPDATA%/kiro/User/

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return a.kiroConfigDir(homeDir)
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro", "steering")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.SystemPromptDir(homeDir), "cortex-ia.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(a.kiroConfigDir(homeDir), "settings.json")
}

// --- Sub-agent support (Kiro native agents in ~/.kiro/agents/) ---

func (a *Adapter) SupportsSubAgents() bool { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro", "agents")
}

// --- Config strategies ---

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

// --- MCP ---

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".kiro", "settings", "mcp.json")
}

func (a *Adapter) kiroConfigDir(homeDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Kiro", "User")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		return filepath.Join(appData, "kiro", "User")
	default:
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			xdgConfigHome = filepath.Join(homeDir, ".config")
		}
		return filepath.Join(xdgConfigHome, "kiro", "user")
	}
}

// --- Capabilities ---

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }

// --- Sub-agent capabilities ---

func (a *Adapter) SupportsTaskDelegation() bool { return true }
