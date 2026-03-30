package vscode

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/lleontor705/cortex-ia/internal/model"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Agent() model.AgentID   { return model.AgentVSCodeCopilot }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", configPath, false, nil
		}
		return false, "", "", false, err
	}
	return true, "", configPath, info.IsDir(), nil
}

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".copilot")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "prompts")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "prompts", "cortex-ia.instructions.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".copilot", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(vscodeUserDir(homeDir), "mcp.json")
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

// vscodeUserDir returns the platform-specific VS Code User directory.
func vscodeUserDir(homeDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Code", "User")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "Code", "User")
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "Code", "User")
	default:
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			return filepath.Join(xdg, "Code", "User")
		}
		return filepath.Join(homeDir, ".config", "Code", "User")
	}
}
