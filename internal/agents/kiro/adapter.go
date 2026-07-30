// Package kiro provides Kiro IDE agent integration.
package kiro

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type Adapter struct {
	lookPath   func(string) (string, error)
	statPath   func(string) (os.FileInfo, error)
	runCommand func(context.Context, string, ...string) ([]byte, error)
	now        func() time.Time
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath:   exec.LookPath,
		statPath:   os.Stat,
		runCommand: runCommand,
		now:        time.Now,
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
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) CommandsDir(_ string) string { return "" }

// --- Sub-agent capabilities ---

func (a *Adapter) SupportsTaskDelegation() bool { return true }

// CapabilityFacts returns conservative, evidence-backed Kiro capability
// metadata. The installed agent schema establishes the configuration surface,
// but runtime delegation remains experimental and advisory until the explicit
// executable probe succeeds and the operator opts in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC)
	schemaResult := "custom agents expose delegated subagent configuration"
	schemaDigest := sha256.Sum256([]byte(schemaResult))
	return []capability.CapabilityFact{{
		ID:              "delegation/direct-child",
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          "kiro",
		RuntimeID:       "kiro",
		AdapterID:       string(model.AgentKiroIDE),
		RuntimeVersions: ir.VersionRange{Minimum: ir.MustParseVersion("1.0.0"), MaximumTested: ir.MustParseVersion("1.99.99")},
		EvidenceClass:   capability.EvidenceInstalledSchema,
		EvidenceRef:     "https://kiro.dev/docs/chat/subagents/",
		ObservedAt:      observedAt,
		FreshUntil:      time.Date(2027, time.March, 5, 0, 0, 0, 0, time.UTC),
		Confidence:      0.85,
		Experimental:    true,
		Current:         true,
		Probe: &capability.ProbeRecord{
			ID:             "probe/kiro-agent-schema",
			Method:         capability.ProbeProtocol,
			Protocol:       "kiro-agent-frontmatter-schema/v1",
			Result:         schemaResult,
			Timestamp:      observedAt,
			EvidenceDigest: fmt.Sprintf("sha256:%x", schemaDigest),
		},
		Enforcement: capability.EnforcementPrompt,
	}}
}

// CapabilityProber exposes only qualification evidence. It does not launch,
// schedule, or otherwise manage agents.
func (a *Adapter) CapabilityProber() capability.Prober {
	lookPath := a.lookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	runner := a.runCommand
	if runner == nil {
		runner = runCommand
	}
	now := a.now
	if now == nil {
		now = time.Now
	}
	return &kiroCapabilityProber{lookPath: lookPath, runCommand: runner, now: now}
}

type kiroCapabilityProber struct {
	lookPath   func(string) (string, error)
	runCommand func(context.Context, string, ...string) ([]byte, error)
	now        func() time.Time
}

func (p *kiroCapabilityProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.ID != "delegation/direct-child" {
		return capability.ProbeResult{}, fmt.Errorf("unsupported Kiro capability probe %q", request.Base.ID)
	}
	binary, err := p.lookPath("kiro")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("locate Kiro executable: %w", err)
	}
	output, err := p.runCommand(ctx, binary, "--help")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("probe Kiro subagent support: %w", err)
	}
	normalized := strings.ToLower(strings.TrimSpace(string(output)))
	if !strings.Contains(normalized, "subagent") && !strings.Contains(normalized, "sub-agent") {
		return capability.ProbeResult{}, fmt.Errorf("kiro help does not advertise subagent support")
	}

	refined := request.Base
	refined.Mode = capability.CapabilityAvailable
	refined.Cardinality = capability.CardinalityMany
	refined.Enforcement = capability.EnforcementRuntime
	digest := sha256.Sum256([]byte(normalized))
	return capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/kiro-subagent-help",
			Method:         capability.ProbeCommand,
			Command:        "kiro --help",
			Result:         "subagent capability advertised",
			Timestamp:      p.now().UTC(),
			EvidenceDigest: fmt.Sprintf("sha256:%x", digest),
		},
		Refined: refined,
	}, nil
}

func runCommand(ctx context.Context, binary string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, binary, args...).Output()
}
