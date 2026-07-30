// Package kimi provides Kimi Code CLI agent integration.
//
// Kimi Code uses ~/.kimi/ as the global config dir for all configuration
// including skills, system prompt, MCP, and sub-agents.
package kimi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

var LookPathOverride = exec.LookPath

type statResult struct {
	isDir bool
	err   error
}

// Adapter implements agents.Adapter for Kimi Code CLI.
type Adapter struct {
	lookPath    func(string) (string, error)
	statPath    func(string) statResult
	pathExists  func(string) bool
	userHomeDir func() (string, error)
	runInfo     func(context.Context, string, ...string) ([]byte, error)
	now         func() time.Time
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath:    LookPathOverride,
		statPath:    defaultStat,
		pathExists:  defaultPathExists,
		userHomeDir: os.UserHomeDir,
		runInfo:     runCommand,
		now:         time.Now,
	}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID    { return model.AgentKimi }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

// --- Detection ---

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := ConfigPath(homeDir)

	binaryPath, err := a.findKimi()
	installed := err == nil && binaryPath != ""

	stat := a.statPath(configPath)
	if stat.err != nil {
		if os.IsNotExist(stat.err) {
			return installed, binaryPath, configPath, false, nil
		}
		return false, "", "", false, stat.err
	}

	return installed, binaryPath, configPath, stat.isDir, nil
}

// findKimi searches for kimi in PATH and official fallback locations.
func (a *Adapter) findKimi() (string, error) {
	if path, err := a.lookPath("kimi"); err == nil {
		return path, nil
	}

	home, err := a.userHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("kimi not found in PATH and home directory is unavailable")
	}

	fallbacks := []string{
		filepath.Join(home, ".local", "bin", binaryName()),
		filepath.Join(home, "bin", binaryName()),
	}
	if runtime.GOOS == "windows" {
		fallbacks = append(fallbacks,
			filepath.Join(home, "AppData", "Local", "Microsoft", "WinGet", "Links", "kimi.exe"),
			filepath.Join(home, "AppData", "Roaming", "uv", "bin", "kimi.exe"),
		)
	}

	for _, fb := range fallbacks {
		if a.pathExists(fb) {
			return fb, nil
		}
	}

	return "", fmt.Errorf("kimi not found in PATH or official install locations")
}

// --- Installation ---

func (a *Adapter) SupportsAutoInstall() bool { return true }

// InstallCommands installs Kimi via Astral's `uv` package manager (avoids
// the upstream pipe-to-shell bootstrap script).
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string {
	return [][]string{{"uv", "tool", "install", "kimi-cli"}}
}

// --- Config paths ---

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".kimi")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".kimi")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".kimi", "KIMI.md")
}

// SkillsDir returns the agent-specific skills directory under ~/.kimi/skills,
// following the same convention as other agents (each owns its own skills dir).
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".kimi", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".kimi", "config.toml")
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
	return filepath.Join(homeDir, ".kimi", "mcp.json")
}

// --- Capabilities ---

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) CommandsDir(_ string) string { return "" }

// --- Sub-agent support ---

func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".kimi", "agents")
}

// CapabilityFacts returns conservative, version-bounded facts for Kimi CLI
// 1.49.0. Documentation proves availability but not runtime enforcement, so
// executable version qualification refreshes provenance without silently
// upgrading enforcement. Background parallel delegation is deliberately
// experimental and cannot participate in native selection without opt-in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 16, 10, 23, 22, 0, time.UTC)
	freshUntil := observedAt.Add(90 * 24 * time.Hour)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion("1.49.0"),
		MaximumTested: ir.MustParseVersion("1.49.0"),
	}
	available := func(id capability.CapabilityID, experimental bool) capability.CapabilityFact {
		return capability.CapabilityFact{
			ID:              id,
			Mode:            capability.CapabilityAvailable,
			Cardinality:     capability.CardinalityMany,
			Target:          "kimi",
			RuntimeID:       "kimi-cli",
			AdapterID:       "cortex-ia/kimi",
			RuntimeVersions: versions,
			EvidenceClass:   capability.EvidenceDocumentation,
			EvidenceRef:     "https://github.com/MoonshotAI/kimi-cli/blob/1.49.0/docs/en/customization/agents.md",
			ObservedAt:      observedAt,
			FreshUntil:      freshUntil,
			Confidence:      0.85,
			Experimental:    experimental,
			Current:         true,
			Enforcement:     capability.EnforcementPrompt,
		}
	}

	return []capability.CapabilityFact{
		available("delegation/direct-child", false),
		available("delegation/background-parallel", true),
		{
			ID:              "delegation/nested",
			Mode:            capability.CapabilityAbsent,
			Cardinality:     capability.CardinalityNone,
			Target:          "kimi",
			RuntimeID:       "kimi-cli",
			AdapterID:       "cortex-ia/kimi",
			RuntimeVersions: versions,
			Experimental:    false,
			Current:         true,
			Enforcement:     capability.EnforcementNone,
		},
	}
}

// CapabilityProber exposes installation inspection only. It cannot launch,
// schedule, resume, or otherwise control Kimi runtime sessions.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runInfo
	if runner == nil {
		runner = runCommand
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return kimiInfoProber{run: runner, now: clock}
}

type kimiInfoProber struct {
	run func(context.Context, string, ...string) ([]byte, error)
	now func() time.Time
}

type kimiInfo struct {
	Version           string   `json:"kimi_cli_version"`
	AgentSpecVersions []string `json:"agent_spec_versions"`
	WireProtocol      string   `json:"wire_protocol_version"`
}

func (p kimiInfoProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "kimi" || request.Base.RuntimeID != "kimi-cli" || request.Base.AdapterID != "cortex-ia/kimi" {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: unsupported capability identity %q", request.Base.ID)
	}
	if request.Base.Mode == capability.CapabilityAbsent {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: capability %q is explicitly unsupported", request.Base.ID)
	}
	if request.Base.Experimental && !request.Authority.ExperimentalOptIn {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: capability %q requires explicit opt-in", request.Base.ID)
	}

	output, err := p.run(ctx, "kimi", "info", "--json")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: execute info: %w", err)
	}
	var info kimiInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: invalid info JSON: %w", err)
	}
	version, err := ir.ParseVersion(strings.TrimSpace(info.Version))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: invalid runtime version: %w", err)
	}
	if !versionInRange(version, request.Authority.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: runtime version %s is outside supported interval %s", version, request.Authority.RuntimeVersions.String())
	}
	if !slices.Contains(info.AgentSpecVersions, "1") {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: agent spec version 1 is unsupported by runtime %s", version)
	}

	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	recordedAt := p.now().UTC()
	digest := sha256.Sum256(output)
	result := capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/kimi-info",
			Method:         capability.ProbeCommand,
			Command:        "kimi info --json",
			Result:         fmt.Sprintf("version=%s;agent_spec=1;wire=%s", version, info.WireProtocol),
			Timestamp:      recordedAt,
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kimi capability probe: refinement blocked: %w", err)
	}
	return result, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func versionInRange(version ir.Version, interval ir.VersionRange) bool {
	return compareVersion(version, interval.Minimum) >= 0 && compareVersion(version, interval.MaximumTested) <= 0
}

func compareVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}

// --- Helpers ---

func defaultStat(path string) statResult {
	info, err := os.Stat(path)
	if err != nil {
		return statResult{err: err}
	}
	return statResult{isDir: info.IsDir()}
}

func defaultPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ConfigPath returns the configuration directory path.
func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".kimi")
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "kimi.exe"
	}
	return "kimi"
}
