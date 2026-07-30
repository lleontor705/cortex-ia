package windsurf

import (
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Agent() model.AgentID   { return model.AgentWindsurf }
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
	return filepath.Join(homeDir, ".codeium", "windsurf")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".codeium", "windsurf", "memories")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".codeium", "windsurf", "memories", "global_rules.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".codeium", "windsurf", "skills")
}

// SettingsPath returns empty because Windsurf uses MCPConfigFile for all MCP
// configuration and does not support settings-merge strategy. Windsurf's
// editor settings live in the platform-specific VS Code-like User directory,
// not in ~/.codeium/windsurf/, so there is no home-relative settings file
// to merge into.
func (a *Adapter) SettingsPath(_ string) string {
	return ""
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyAppendToFile
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json")
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

// CapabilityFacts reports only evidence-backed Windsurf workflow capabilities.
// Windsurf 2.0 documents direct delegation to Devin, but rollout and admin
// controls vary and no stable installed-runtime probe currently qualifies that
// feature. The fact therefore remains experimental and advisory, which keeps
// profile selection on the portable sequential fallback even after opt-in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	windsurf2 := ir.VersionRange{
		Minimum:       ir.MustParseVersion("2.0.0"),
		MaximumTested: ir.MustParseVersion("2.0.0"),
	}

	return []capability.CapabilityFact{
		{
			ID:              "delegation/direct-child",
			Mode:            capability.CapabilityAvailable,
			Cardinality:     capability.CardinalityMany,
			Target:          "windsurf",
			RuntimeID:       "windsurf-cascade",
			AdapterID:       string(model.AgentWindsurf),
			RuntimeVersions: windsurf2,
			EvidenceClass:   capability.EvidenceDocumentation,
			EvidenceRef:     "https://docs.windsurf.com/windsurf/devin.md",
			ObservedAt:      observedAt,
			FreshUntil:      observedAt.AddDate(0, 3, 0),
			Confidence:      0.8,
			Experimental:    true,
			Current:         true,
			Enforcement:     capability.EnforcementPrompt,
		},
		{
			ID:              "delegation/nested",
			Mode:            capability.CapabilityAbsent,
			Cardinality:     capability.CardinalityNone,
			Target:          "windsurf",
			RuntimeID:       "windsurf-cascade",
			AdapterID:       string(model.AgentWindsurf),
			RuntimeVersions: windsurf2,
			Current:         true,
			Enforcement:     capability.EnforcementNone,
		},
	}
}

// CapabilityProber returns nil because Windsurf does not currently expose a
// stable installed schema or executable probe that proves delegation semantics.
// Documentation alone must never upgrade the selected workflow profile.
func (a *Adapter) CapabilityProber() capability.Prober { return nil }

// --- Auto-install ---

func (a *Adapter) SupportsAutoInstall() bool                          { return false }
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }
