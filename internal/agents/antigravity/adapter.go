package antigravity

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type Adapter struct {
	now func() time.Time
}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Agent() model.AgentID    { return model.AgentAntigravity }
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
	return filepath.Join(homeDir, ".gemini", "antigravity")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "antigravity")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "antigravity", "instructions.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "antigravity", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "antigravity", "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyAppendToFile
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".gemini", "antigravity", "mcp_config.json")
}

func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

// CapabilityFacts describes Antigravity runtime features without granting this
// configuration adapter execution authority. Official documentation establishes
// that Antigravity 2.x can invoke direct and nested subagents, but documentation
// alone cannot prove runtime enforcement. Both facts therefore remain
// prompt-enforced and experimental.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion("2.0.0"),
		MaximumTested: ir.MustParseVersion("2.4.2"),
	}
	base := capability.CapabilityFact{
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          "antigravity",
		RuntimeID:       "antigravity-2",
		AdapterID:       "cortex-ia/antigravity",
		RuntimeVersions: versions,
		EvidenceClass:   capability.EvidenceDocumentation,
		ObservedAt:      observedAt,
		FreshUntil:      observedAt.AddDate(0, 3, 0),
		Confidence:      0.85,
		Experimental:    true,
		Current:         true,
		Enforcement:     capability.EnforcementPrompt,
	}

	directChild := base
	directChild.ID = "delegation/direct-child"
	directChild.EvidenceRef = "https://antigravity.google/docs/subagents"

	nested := base
	nested.ID = "delegation/nested"
	nested.EvidenceRef = "https://antigravity.google/docs/subagents#inter-agent-communication-nesting-limits"

	return []capability.CapabilityFact{directChild, nested}
}

// CapabilityProber validates the documented Antigravity agent-configuration
// contract. It never launches an agent, inspects a session, or upgrades prompt
// evidence to runtime enforcement.
func (a *Adapter) CapabilityProber() capability.Prober {
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return antigravityConfigurationProber{now: clock}
}

type antigravityConfigurationProber struct {
	now func() time.Time
}

func (p antigravityConfigurationProber) Probe(_ context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "antigravity" || request.Base.RuntimeID != "antigravity-2" || request.Base.AdapterID != "cortex-ia/antigravity" ||
		(request.Base.ID != "delegation/direct-child" && request.Base.ID != "delegation/nested") || request.Authority.CapabilityID != request.Base.ID {
		return capability.ProbeResult{}, fmt.Errorf("unsupported Antigravity capability %q", request.Base.ID)
	}
	if request.Base.Experimental && !request.Authority.ExperimentalOptIn {
		return capability.ProbeResult{}, fmt.Errorf("antigravity experimental capability %q requires explicit opt-in", request.Base.ID)
	}
	for _, enforcement := range request.Authority.Enforcement {
		if enforcement == capability.EnforcementRuntime {
			return capability.ProbeResult{}, fmt.Errorf("antigravity probe is configuration-only and cannot claim runtime authority")
		}
	}

	refined := request.Base
	timestamp := p.now().UTC()
	evidence := "antigravity-agent-configuration/v1:" + string(request.Base.ID)
	digest := sha256.Sum256([]byte(evidence))
	result := capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/antigravity-agent-configuration",
			Method:         capability.ProbeProtocol,
			Protocol:       "antigravity-agent-configuration/v1",
			Result:         "configuration-supported;runtime-unverified",
			Timestamp:      timestamp,
			EvidenceDigest: fmt.Sprintf("sha256:%x", digest),
		},
		Refined: refined,
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("antigravity configuration probe refinement blocked: %w", err)
	}
	return result, nil
}

// --- Auto-install ---

func (a *Adapter) SupportsAutoInstall() bool                           { return false }
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }
