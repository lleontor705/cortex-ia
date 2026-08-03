package codex

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

type Adapter struct {
	lookPath func(string) (string, error)
	runProbe func(context.Context, string, ...string) ([]byte, error)
	now      func() time.Time
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		runProbe: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		now: time.Now,
	}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentCodex }
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

func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

const (
	codexDirectChild      capability.CapabilityID = "delegation/direct-child"
	codexParallel         capability.CapabilityID = "delegation/parallel"
	qualifiedCodexVersion                         = "0.145.0"
)

var (
	errUnsupportedCodexCapability = errors.New("unsupported Codex capability probe")
	codexVersionPattern           = regexp.MustCompile(`(?i)\bcodex(?:-cli)?\s+v?(\d+\.\d+\.\d+)\b`)
)

// CapabilityFacts returns the qualified Codex CLI capability snapshot. The
// direct-child primitive supports portable-flat workflows. Parallel delegation
// remains experimental and therefore cannot select native-advanced without an
// explicit operator opt-in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	freshUntil := observedAt.Add(90 * 24 * time.Hour)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedCodexVersion),
		MaximumTested: ir.MustParseVersion(qualifiedCodexVersion),
	}

	return []capability.CapabilityFact{
		codexCapabilityFact(
			codexDirectChild,
			"qualification/codex-cli/0.145.0/direct-child/2026-07-26",
			"probe/codex-cli/direct-child",
			"sha256:417ffb4cda5fb3295daf6453db145bc0e40fb58573c145816278a3043685a15d",
			versions,
			observedAt,
			freshUntil,
			0.9,
			false,
		),
		codexCapabilityFact(
			codexParallel,
			"qualification/codex-cli/0.145.0/parallel-subagents/2026-07-26",
			"probe/codex-cli/parallel-subagents",
			"sha256:90f689abaf26378d9caa0faf61c73e06ad78048a8d131dfa68559da552b46a77",
			versions,
			observedAt,
			freshUntil,
			0.85,
			true,
		),
	}
}

// CapabilityProber performs read-only installed-version qualification. It
// cannot launch agents, inspect sessions, or mutate runtime-owned state.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return codexVersionProber{run: runner, now: clock}
}

type codexVersionProber struct {
	run func(context.Context, string, ...string) ([]byte, error)
	now func() time.Time
}

func (p codexVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.ID != codexDirectChild && request.Base.ID != codexParallel {
		return capability.ProbeResult{}, fmt.Errorf("%w: %s", errUnsupportedCodexCapability, request.Base.ID)
	}
	if request.Authority.CapabilityID != request.Base.ID {
		return capability.ProbeResult{}, fmt.Errorf("%w: capability is outside declared authority", errUnsupportedCodexCapability)
	}
	if request.Base.Target != "codex" || request.Base.RuntimeID != "codex-cli" || request.Base.AdapterID != "cortex-ia/codex" {
		return capability.ProbeResult{}, fmt.Errorf("%w: foreign capability identity", errUnsupportedCodexCapability)
	}
	if request.Base.Experimental && !request.Authority.ExperimentalOptIn {
		return capability.ProbeResult{}, fmt.Errorf("codex experimental native capability %q requires explicit opt-in", request.Base.ID)
	}

	output, err := p.run(ctx, "codex", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("probe Codex CLI version: %w", err)
	}
	match := codexVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("codex CLI probe did not report a semantic version")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("parse Codex CLI probe version: %w", err)
	}
	if !codexVersionInRange(version, request.Base.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("codex CLI version %s is outside qualified range %s", version.String(), request.Base.RuntimeVersions.String())
	}

	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	return capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/codex-cli/version",
			Method:         capability.ProbeCommand,
			Command:        "codex --version",
			Result:         "qualified-version:" + version.String(),
			Timestamp:      p.now().UTC(),
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}, nil
}

func codexCapabilityFact(
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
		Target:          "codex",
		RuntimeID:       "codex-cli",
		AdapterID:       "cortex-ia/codex",
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
			Command:        "codex --version",
			Result:         "qualified-version:" + qualifiedCodexVersion,
			Timestamp:      observedAt,
			EvidenceDigest: digest,
		},
		Enforcement: capability.EnforcementRuntime,
	}
}

func codexVersionInRange(version ir.Version, versions ir.VersionRange) bool {
	return compareCodexVersion(versions.Minimum, version) <= 0 && compareCodexVersion(version, versions.MaximumTested) <= 0
}

func compareCodexVersion(left, right ir.Version) int {
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
	return [][]string{{"npm", "install", "-g", "@openai/codex"}}
}
