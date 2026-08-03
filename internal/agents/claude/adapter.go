package claude

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

const (
	claudeDirectChildCapability capability.CapabilityID = "delegation/direct-child"
	claudeAgentTeamsCapability  capability.CapabilityID = "tasks/dependencies"
	qualifiedClaudeVersion                              = "2.1.199"
)

var (
	errUnqualifiedClaudeVersion = errors.New("installed Claude Code version is not qualified")
	claudeVersionPattern        = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)
)

type probeRunner func(context.Context, string, ...string) ([]byte, error)

// Adapter implements the agents.Adapter interface for Claude Code.
type Adapter struct {
	lookPath func(string) (string, error)
	runProbe probeRunner
	now      func() time.Time
}

// NewAdapter creates a new Claude Code adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		runProbe: runProbeCommand,
		now:      time.Now,
	}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID    { return model.AgentClaudeCode }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

// --- Detection ---

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)

	binaryPath, err := a.lookPath("claude")
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

// --- Config paths ---

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "CLAUDE.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "settings.json")
}

// --- Config strategies ---

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyMarkdownSections
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategySeparateMCPFiles
}

// --- MCP ---

func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string {
	return filepath.Join(homeDir, ".claude", "mcp", serverName+".json")
}

// --- Capabilities ---

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) CommandsDir(_ string) string { return "" }

// --- Sub-agent capabilities ---

func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

// CapabilityFacts returns the conservative qualification snapshot for Claude
// Code. The stable direct-child primitive supports the portable-flat profile;
// experimental agent teams remain unavailable to native profiles unless the
// operator explicitly opts in through profile selection.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	freshUntil := observedAt.Add(90 * 24 * time.Hour)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedClaudeVersion),
		MaximumTested: ir.MustParseVersion(qualifiedClaudeVersion),
	}

	return []capability.CapabilityFact{
		claudeCapabilityFact(
			claudeDirectChildCapability,
			"qualification/claude-code/2.1.199/direct-child/2026-07-26",
			"probe/claude-code/direct-child",
			"sha256:3d50ad89d3c3a2b40cd68e22aed722946e9c80e131081730a74d8c6cfc1fbf7b",
			versions,
			observedAt,
			freshUntil,
			0.95,
			false,
		),
		claudeCapabilityFact(
			claudeAgentTeamsCapability,
			"qualification/claude-code/2.1.199/agent-teams/2026-07-26",
			"probe/claude-code/agent-teams",
			"sha256:5cbfd5f521975821a6a0347b2695a3657333b72e1777c3fe8f6f61de82e14a29",
			versions,
			observedAt,
			freshUntil,
			0.85,
			true,
		),
	}
}

// CapabilityProber exposes only installed-version qualification. It does not
// launch agents, inspect sessions, or manage any runtime-owned state.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = runProbeCommand
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return &claudeVersionProber{run: runner, now: clock}
}

type claudeVersionProber struct {
	run probeRunner
	now func() time.Time
}

func (p *claudeVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "claude" || request.Base.RuntimeID != "claude-code" || request.Base.AdapterID != "cortex-ia/claude" {
		return capability.ProbeResult{}, fmt.Errorf("%w: fact does not belong to the Claude adapter", errUnqualifiedClaudeVersion)
	}

	output, err := p.run(ctx, "claude", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("probe Claude Code version: %w", err)
	}
	match := claudeVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("%w: version output did not contain semantic version", errUnqualifiedClaudeVersion)
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil || !versionInRange(version, request.Base.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("%w: %s", errUnqualifiedClaudeVersion, match[1])
	}

	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	record := capability.ProbeRecord{
		ID:             "probe/claude-code/version",
		Method:         capability.ProbeCommand,
		Command:        "claude --version",
		Result:         "qualified-version:" + version.String(),
		Timestamp:      p.now().UTC(),
		EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
	}
	return capability.ProbeResult{Record: record, Refined: request.Base}, nil
}

func claudeCapabilityFact(
	id capability.CapabilityID,
	evidenceRef string,
	probeID ir.SemanticID,
	digest string,
	versions ir.VersionRange,
	observedAt time.Time,
	freshUntil time.Time,
	confidence capability.Confidence,
	experimental bool,
) capability.CapabilityFact {
	return capability.CapabilityFact{
		ID:              id,
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          "claude",
		RuntimeID:       "claude-code",
		AdapterID:       "cortex-ia/claude",
		RuntimeVersions: versions,
		EvidenceClass:   capability.EvidenceExecutableProbe,
		EvidenceRef:     evidenceRef,
		ObservedAt:      observedAt,
		FreshUntil:      freshUntil,
		Confidence:      confidence,
		Experimental:    experimental,
		Current:         true,
		Probe: &capability.ProbeRecord{
			ID:             probeID,
			Method:         capability.ProbeCommand,
			Command:        "claude --version",
			Result:         "qualified-version:" + qualifiedClaudeVersion,
			Timestamp:      observedAt,
			EvidenceDigest: digest,
		},
		Enforcement: capability.EnforcementRuntime,
	}
}

func runProbeCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func versionInRange(version ir.Version, versions ir.VersionRange) bool {
	return compareVersion(version, versions.Minimum) >= 0 && compareVersion(version, versions.MaximumTested) <= 0
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

// --- Auto-install ---

func (a *Adapter) SupportsAutoInstall() bool { return true }
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string {
	return [][]string{{"npm", "install", "-g", "@anthropic-ai/claude-code"}}
}
