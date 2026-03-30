package codex

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/model"
)

type Adapter struct {
	lookPath func(string) (string, error)
}

func NewAdapter() *Adapter {
	return &Adapter{lookPath: exec.LookPath}
}

func (a *Adapter) Agent() model.AgentID   { return model.AgentCodex }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)
	binaryPath, err := a.lookPath("codex")
	installed := err == nil

	info, statErr := os.Stat(configPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return installed, binaryPath, configPath, false, nil
		}
		return false, "", "", false, statErr
	}
	return installed, binaryPath, configPath, info.IsDir(), nil
}

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "agents.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "config.toml")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyTOMLFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".codex", "config.toml")
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }
