package cursor

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

type probeRunner func(context.Context, string, ...string) ([]byte, error)

type Adapter struct {
	now      func() time.Time
	runProbe probeRunner
}

func NewAdapter() *Adapter {
	return &Adapter{
		now:      time.Now,
		runProbe: runCursorProbe,
	}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentCursor }
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
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) CommandsDir(_ string) string { return "" }

// SupportsTaskDelegation returns true because Cursor has a sub-agent system
// (~/.cursor/agents/) with deployed stubs. The multi-agent orchestrator prompt
// instructs the LLM to delegate to those sub-agents via the IDE's built-in
// agent invocation mechanism.
func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "agents")
}

// --- Auto-install ---

func (a *Adapter) SupportsAutoInstall() bool                           { return false }
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

const (
	cursorDirectChild capability.CapabilityID = "delegation/direct-child"
	cursorParallel    capability.CapabilityID = "delegation/parallel"
)

var (
	errUnsupportedCapability = errors.New("unsupported Cursor capability probe")
	cursorVersionPattern     = regexp.MustCompile(`(?i)cursor\s+v?(\d+\.\d+\.\d+)`)
)

// CapabilityFacts returns the conservative qualification snapshot used by the
// workflow compiler. Parallel delegation remains experimental even when the
// installed Cursor schema qualifies it, so native-advanced still requires an
// explicit operator opt-in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	freshUntil := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion("3.0.0"),
		MaximumTested: ir.MustParseVersion("3.5.0"),
	}

	newFact := func(id capability.CapabilityID, experimental bool, evidenceRef string) capability.CapabilityFact {
		return capability.CapabilityFact{
			ID:              id,
			Mode:            capability.CapabilityAvailable,
			Cardinality:     capability.CardinalityMany,
			Target:          "cursor",
			RuntimeID:       "cursor",
			AdapterID:       "cortex-ia/cursor",
			RuntimeVersions: versions,
			EvidenceClass:   capability.EvidenceInstalledSchema,
			EvidenceRef:     evidenceRef,
			ObservedAt:      observedAt,
			FreshUntil:      freshUntil,
			Confidence:      0.9,
			Experimental:    experimental,
			Current:         true,
			Probe: &capability.ProbeRecord{
				ID:             "probe/cursor-version",
				Method:         capability.ProbeCommand,
				Command:        "cursor --version",
				Result:         "version=3.5.0;available:many",
				Timestamp:      observedAt,
				EvidenceDigest: evidenceRef,
			},
			Enforcement: capability.EnforcementRuntime,
		}
	}

	return []capability.CapabilityFact{
		newFact(cursorDirectChild, false, "sha256:0073e9e7c34d2552d997426108ab0db87a9207e67b75be81125a44220be6a3ea"),
		newFact(cursorParallel, true, "sha256:96ac7117d8153cdbc343e5b14992ddb573080b1eeb0a3649647f6d006ccd811d"),
	}
}

// CapabilityProber exposes only the capability.Prober inspection boundary. It
// can read Cursor's version output but cannot start agents, schedule work, or
// mutate runtime state.
func (a *Adapter) CapabilityProber() capability.Prober {
	now := a.now
	if now == nil {
		now = time.Now
	}
	runner := a.runProbe
	if runner == nil {
		runner = runCursorProbe
	}
	return cursorProber{now: now, run: runner}
}

type cursorProber struct {
	now func() time.Time
	run probeRunner
}

func (p cursorProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.ID != cursorDirectChild && request.Base.ID != cursorParallel {
		return capability.ProbeResult{}, fmt.Errorf("%w: %s", errUnsupportedCapability, request.Base.ID)
	}

	output, err := p.run(ctx, "cursor", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("probe Cursor version: %w", err)
	}
	version, err := parseCursorVersion(output)
	if err != nil {
		return capability.ProbeResult{}, err
	}
	if !versionInRange(version, request.Authority.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("cursor version %s is outside probe authority %s", version, request.Authority.RuntimeVersions.String())
	}

	timestamp := p.now().UTC()
	digestBytes := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	digest := "sha256:" + hex.EncodeToString(digestBytes[:])
	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}

	return capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/cursor-version",
			Method:         capability.ProbeCommand,
			Command:        "cursor --version",
			Result:         "version=" + version.String() + ";available:many",
			Timestamp:      timestamp,
			EvidenceDigest: digest,
		},
		Refined: refined,
	}, nil
}

func runCursorProbe(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func parseCursorVersion(output []byte) (ir.Version, error) {
	match := cursorVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return ir.Version{}, fmt.Errorf("cursor version probe returned unrecognized output")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return ir.Version{}, fmt.Errorf("parse Cursor version: %w", err)
	}
	return version, nil
}

func versionInRange(version ir.Version, versions ir.VersionRange) bool {
	return compareCursorVersion(versions.Minimum, version) <= 0 && compareCursorVersion(version, versions.MaximumTested) <= 0
}

func compareCursorVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}
