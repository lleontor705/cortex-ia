// Package qwen provides Qwen Code CLI agent integration.
package qwen

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

const qualifiedQwenVersion = "0.14.1"

var qwenVersionPattern = regexp.MustCompile(`(?i)\b(?:qwen(?:-code)?\s+)?v?(\d+\.\d+\.\d+)\b`)

type probeRunner func(context.Context, string, ...string) ([]byte, error)

type statResult struct {
	isDir bool
	err   error
}

// Adapter implements agents.Adapter for Qwen Code.
//
// Qwen Code uses ~/.qwen/ as its configuration directory with settings.json
// for MCP server configuration and QWEN.md as the global instructions file.
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

func (a *Adapter) Agent() model.AgentID    { return model.AgentQwenCode }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

// --- Detection ---

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := filepath.Join(homeDir, ".qwen")

	binaryPath, err := a.lookPath("qwen")
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
	return [][]string{{"npm", "install", "-g", "@qwen-code/qwen-code@latest"}}
}

// --- Config paths ---

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "QWEN.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "settings.json")
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
	return filepath.Join(homeDir, ".qwen", "settings.json")
}

// --- Capabilities ---

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return true }
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "commands")
}

// --- Sub-agent capabilities ---

func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

// CapabilityFacts returns the conservative Qwen Code qualification snapshot.
// Tagged official documentation proves direct-child subagents at 0.14.1, but
// not runtime enforcement or nesting, so native use remains experimental and
// prompt-enforced while nested delegation remains unavailable.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedQwenVersion),
		MaximumTested: ir.MustParseVersion(qualifiedQwenVersion),
	}
	identity := capability.CapabilityFact{
		Target:          "qwen",
		RuntimeID:       "qwen-code-cli",
		AdapterID:       "cortex-ia/qwen",
		RuntimeVersions: versions,
		Current:         true,
	}

	direct := identity
	direct.ID = "delegation/direct-child"
	direct.Mode = capability.CapabilityAvailable
	direct.Cardinality = capability.CardinalityMany
	direct.EvidenceClass = capability.EvidenceDocumentation
	direct.EvidenceRef = "https://github.com/QwenLM/qwen-code/blob/v0.14.1/docs/users/features/sub-agents.md"
	direct.ObservedAt = observedAt
	direct.FreshUntil = observedAt.Add(90 * 24 * time.Hour)
	direct.Confidence = 0.85
	direct.Experimental = true
	direct.Enforcement = capability.EnforcementPrompt

	nested := identity
	nested.ID = "delegation/nested"
	nested.Mode = capability.CapabilityAbsent
	nested.Cardinality = capability.CardinalityNone
	nested.Enforcement = capability.EnforcementNone

	return []capability.CapabilityFact{direct, nested}
}

// CapabilityProber performs read-only installed-version qualification. It
// cannot launch agents, schedule work, inspect sessions, or claim runtime
// enforcement for the documentation-backed capability.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = runProbeCommand
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return qwenVersionProber{run: runner, now: clock}
}

type qwenVersionProber struct {
	run probeRunner
	now func() time.Time
}

func (p qwenVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "qwen" || request.Base.RuntimeID != "qwen-code-cli" || request.Base.AdapterID != "cortex-ia/qwen" ||
		request.Authority.CapabilityID != request.Base.ID {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: unsupported capability identity %q", request.Base.ID)
	}
	if request.Base.Mode == capability.CapabilityAbsent {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: capability %q is explicitly unsupported", request.Base.ID)
	}
	if request.Base.ID != "delegation/direct-child" {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: unsupported capability identity %q", request.Base.ID)
	}
	if request.Base.Experimental && !request.Authority.ExperimentalOptIn {
		return capability.ProbeResult{}, fmt.Errorf("qwen experimental capability %q requires explicit opt-in", request.Base.ID)
	}

	output, err := p.run(ctx, "qwen", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: execute version command: %w", err)
	}
	match := qwenVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: output did not contain a semantic version")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: invalid semantic version: %w", err)
	}
	if compareVersion(version, request.Base.RuntimeVersions.Minimum) < 0 || compareVersion(version, request.Base.RuntimeVersions.MaximumTested) > 0 {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: runtime version %s is outside qualified interval %s", version, request.Base.RuntimeVersions.String())
	}

	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	result := capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/qwen/version",
			Method:         capability.ProbeCommand,
			Command:        "qwen --version",
			Result:         "qualified-version:" + version.String(),
			Timestamp:      p.now().UTC(),
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("qwen capability probe: refinement blocked: %w", err)
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
