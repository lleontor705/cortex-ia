package cursor

import (
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/model"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Agent() model.AgentID   { return model.AgentCursor }
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
	return filepath.Join(homeDir, ".cursor")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "rules")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "rules", "cortex-ia.mdc")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".cursor", "mcp.json")
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "agents")
}
