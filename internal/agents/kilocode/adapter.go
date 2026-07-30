// Package kilocode provides the adapter for Kilocode (Kilo).
package kilocode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

var LookPathOverride = exec.LookPath

const qualifiedKiloVersion = "7.4.16"

var kiloVersionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

type probeRunner func(context.Context, string, ...string) ([]byte, error)

type statResult struct {
	isDir bool
	err   error
}

type Adapter struct {
	lookPath func(string) (string, error)
	statPath func(string) statResult
	runProbe probeRunner
	now      func() time.Time
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: LookPathOverride,
		statPath: defaultStat,
		runProbe: runProbeCommand,
		now:      time.Now,
	}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID    { return model.AgentKilocode }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

// --- Detection ---

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := ConfigPath(homeDir)

	binaryPath, err := a.lookPath("kilo")
	installed := err == nil

	stat := a.statPath(configPath)
	if stat.err != nil {
		if os.IsNotExist(stat.err) {
			return installed, binaryPath, configPath, false, nil
		}
		return false, "", "", false, stat.err
	}

	return installed, binaryPath, configPath, stat.isDir, nil
}

// --- Installation ---

func (a *Adapter) SupportsAutoInstall() bool { return true }

func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string {
	return [][]string{{"npm", "install", "-g", "@kilocode/kilo@latest"}}
}

// --- Config paths ---

func (a *Adapter) GlobalConfigDir(homeDir string) string { return ConfigPath(homeDir) }
func (a *Adapter) SystemPromptDir(homeDir string) string { return ConfigPath(homeDir) }
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "AGENTS.md")
}
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "skills")
}
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "opencode.json")
}

// --- Config strategies ---

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMergeIntoSettings
}

// --- MCP ---

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(ConfigPath(homeDir), "opencode.json")
}

// --- Capabilities ---

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return true }
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "commands")
}

// --- Sub-agent capabilities ---

func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "agents")
}

// CapabilityFacts returns the conservative qualification snapshot for the
// currently tested Kilo CLI release. Kilo documents direct-child subagents,
// but documentation does not prove runtime enforcement, so the fact remains
// prompt-enforced. Nested delegation is explicitly unsupported.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedKiloVersion),
		MaximumTested: ir.MustParseVersion(qualifiedKiloVersion),
	}
	identity := capability.CapabilityFact{
		Target:          "kilocode",
		RuntimeID:       "kilo-cli",
		AdapterID:       "cortex-ia/kilocode",
		RuntimeVersions: versions,
		Current:         true,
	}

	direct := identity
	direct.ID = "delegation/direct-child"
	direct.Mode = capability.CapabilityAvailable
	direct.Cardinality = capability.CardinalityMany
	direct.EvidenceClass = capability.EvidenceDocumentation
	direct.EvidenceRef = "https://kilo.ai/docs/customize/custom-subagents"
	direct.ObservedAt = observedAt
	direct.FreshUntil = observedAt.Add(90 * 24 * time.Hour)
	direct.Confidence = 0.9
	direct.Enforcement = capability.EnforcementPrompt

	nested := identity
	nested.ID = "delegation/nested"
	nested.Mode = capability.CapabilityAbsent
	nested.Cardinality = capability.CardinalityNone
	nested.Enforcement = capability.EnforcementNone

	return []capability.CapabilityFact{direct, nested}
}

// CapabilityProber verifies only the installed Kilo CLI version. It cannot
// launch agents, schedule work, inspect sessions, or manage runtime state.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = runProbeCommand
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return kiloVersionProber{run: runner, now: clock}
}

type kiloVersionProber struct {
	run probeRunner
	now func() time.Time
}

func (p kiloVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "kilocode" || request.Base.RuntimeID != "kilo-cli" || request.Base.AdapterID != "cortex-ia/kilocode" {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: unsupported capability identity %q", request.Base.ID)
	}
	if request.Base.Mode == capability.CapabilityAbsent {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: capability %q is explicitly unsupported", request.Base.ID)
	}

	output, err := p.run(ctx, "kilo", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: execute version command: %w", err)
	}
	match := kiloVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: output did not contain a semantic version")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: invalid semantic version: %w", err)
	}
	if compareVersion(version, request.Authority.RuntimeVersions.Minimum) < 0 || compareVersion(version, request.Authority.RuntimeVersions.MaximumTested) > 0 {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: runtime version %s is outside supported interval %s", version, request.Authority.RuntimeVersions.String())
	}

	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	result := capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/kilocode/version",
			Method:         capability.ProbeCommand,
			Command:        "kilo --version",
			Result:         "qualified-version:" + version.String(),
			Timestamp:      p.now().UTC(),
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("kilocode capability probe: refinement blocked: %w", err)
	}
	return result, nil
}

func runProbeCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
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

func defaultStat(path string) statResult {
	info, err := os.Stat(path)
	if err != nil {
		return statResult{err: err}
	}
	return statResult{isDir: info.IsDir()}
}
