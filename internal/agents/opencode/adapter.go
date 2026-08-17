package opencode

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type Adapter struct {
	lookPath   func(string) (string, error)
	runCommand func(context.Context, string, ...string) ([]byte, error)
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		runCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
	}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentOpenCode }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)
	binaryPath, err := a.lookPath("opencode")
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
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().ConfigRoot))
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().ConfigRoot))
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().ConfigRoot), "AGENTS.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().SkillsRoot))
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return GlobalConfigPath(homeDir)
}

// GlobalConfigPath follows OpenCode's global load precedence: JSONC is loaded
// after JSON and therefore owns conflicting keys when both files exist.
func GlobalConfigPath(homeDir string) string {
	dir := filepath.Join(homeDir, filepath.FromSlash(NativeLayout().ConfigRoot))
	jsonc := filepath.Join(dir, "opencode.jsonc")
	if _, err := os.Stat(jsonc); err == nil || !os.IsNotExist(err) {
		return jsonc
	}
	return filepath.Join(dir, "opencode.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMergeIntoSettings
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return GlobalConfigPath(homeDir)
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return true }
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().CommandsRoot))
}

func (a *Adapter) SupportsTaskDelegation() bool { return true }
func (a *Adapter) SupportsSubAgents() bool      { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(NativeLayout().AgentsRoot))
}

// customSkillIDPattern guards the skill ID used as the single path segment
// of a declared destination. It lexically mirrors the registry policy
// owner's strict lowercase ASCII grammar (one or more [a-z0-9] segments
// joined by single hyphens) because this package must not import the
// registry package; it is a containment guard, never a second grammar
// authority.
var customSkillIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// SkillDestinations implements agents.SkillLayoutProvider for OpenCode. It
// declares the host representation of one verified custom skill as the
// native per-skill directory form .config/opencode/skills/<id>/SKILL.md —
// a plain Markdown data asset OpenCode discovers natively.
//
// The declaration is pure: it reads no files, creates no directories, and
// mutates no state, so planning may call it freely before any write.
// Destinations are home-relative, slash-separated, deterministic, and
// always beneath the adapter's SkillsDir. An ID outside the strict grammar
// fails closed with no destinations rather than an unsafe path. Declaring a
// layout grants no registry, command, subagent, config, tool, permission,
// or binding authority (design D8).
func (a *Adapter) SkillDestinations(skill skillcore.Skill) []string {
	id := string(skill.ID)
	if !customSkillIDPattern.MatchString(id) {
		return nil
	}
	return []string{path.Join(NativeLayout().SkillsRoot, id, "SKILL.md")}
}

var (
	openCodeMinimumQualified = ir.MustParseVersion("1.18.0")
	openCodeMaximumTested    = ir.MustParseVersion("1.18.15")
	openCodeObservedAt       = time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)
	openCodeFreshUntil       = time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC)
	openCodeVersionPattern   = regexp.MustCompile(`(?m)opencode version:\s*v?(\d+\.\d+\.\d+)`)
)

const (
	openCodeTarget               capability.TargetID     = "opencode"
	openCodeAdapterID                                    = "cortex-ia/opencode"
	directChildCapabilityID      capability.CapabilityID = "delegation/direct-child"
	nestedDelegationCapabilityID capability.CapabilityID = "delegation/nested"
)

// CapabilityFacts returns evidence-backed facts for the tested OpenCode
// interval. Direct-child delegation qualifies the portable-flat profile.
// Nested delegation is non-default and remains an explicit native opt-in.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	runtimeVersions := ir.VersionRange{
		Minimum:       openCodeMinimumQualified,
		MaximumTested: openCodeMaximumTested,
	}
	base := capability.CapabilityFact{
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          openCodeTarget,
		RuntimeID:       string(openCodeTarget),
		AdapterID:       openCodeAdapterID,
		RuntimeVersions: runtimeVersions,
		EvidenceClass:   capability.EvidenceExecutableProbe,
		ObservedAt:      openCodeObservedAt,
		FreshUntil:      openCodeFreshUntil,
		Confidence:      0.95,
		Current:         true,
		Enforcement:     capability.EnforcementRuntime,
	}

	directChild := base
	directChild.ID = directChildCapabilityID
	directChild.EvidenceRef = "qualification/opencode/1.18.15/direct-child/2026-08-09"
	directChild.Probe = qualificationProbeRecord("probe/opencode/direct-child", openCodeObservedAt)

	nested := base
	nested.ID = nestedDelegationCapabilityID
	nested.Experimental = true
	nested.EvidenceRef = "qualification/opencode/1.18.15/nested-delegation/2026-08-09"
	nested.Probe = qualificationProbeRecord("probe/opencode/nested-delegation", openCodeObservedAt)

	return []capability.CapabilityFact{directChild, nested}
}

func qualificationProbeRecord(id ir.SemanticID, observedAt time.Time) *capability.ProbeRecord {
	return &capability.ProbeRecord{
		ID:             id,
		Method:         capability.ProbeCommand,
		Command:        "opencode debug info",
		Result:         "qualified-version:1.18.15;available:many",
		Timestamp:      observedAt,
		EvidenceDigest: "sha256:e20cf5c0bdddeff7f795fb4ce5393db4e02a4e6977f4991bef775be46d123104",
	}
}

func (a *Adapter) CapabilityProber() capability.Prober {
	return openCodeProber{runCommand: a.runCommand}
}

type openCodeProber struct {
	runCommand func(context.Context, string, ...string) ([]byte, error)
}

func (p openCodeProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Authority.CapabilityID != request.Base.ID {
		return capability.ProbeResult{}, fmt.Errorf("OpenCode probe capability %q is outside declared authority %q", request.Base.ID, request.Authority.CapabilityID)
	}
	if request.Base.Target != openCodeTarget || request.Base.RuntimeID != string(openCodeTarget) || request.Base.AdapterID != openCodeAdapterID {
		return capability.ProbeResult{}, fmt.Errorf("OpenCode probe cannot inspect foreign capability identity")
	}
	if request.Base.ID != directChildCapabilityID && request.Base.ID != nestedDelegationCapabilityID {
		return capability.ProbeResult{}, fmt.Errorf("OpenCode probe does not qualify capability %q", request.Base.ID)
	}
	if request.Base.Experimental && !request.Authority.ExperimentalOptIn {
		return capability.ProbeResult{}, fmt.Errorf("OpenCode experimental native capability %q requires explicit opt-in", request.Base.ID)
	}

	output, err := p.runCommand(ctx, "opencode", "debug", "info")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("run read-only OpenCode capability probe: %w", err)
	}
	match := openCodeVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("read-only OpenCode probe did not report a semantic version")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("parse OpenCode probe version: %w", err)
	}
	if !versionInRange(version, request.Base.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("OpenCode version %s is outside qualified range %s", version.String(), request.Base.RuntimeVersions.String())
	}

	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	digest := sha256.Sum256(output)
	now := time.Now().UTC()
	return capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/opencode/debug-info",
			Method:         capability.ProbeCommand,
			Command:        "opencode debug info",
			Result:         "available:" + string(refined.Cardinality),
			Timestamp:      now,
			EvidenceDigest: fmt.Sprintf("sha256:%x", digest),
		},
		Refined: refined,
	}, nil
}

func versionInRange(version ir.Version, versionRange ir.VersionRange) bool {
	return compareOpenCodeVersion(version, versionRange.Minimum) >= 0 && compareOpenCodeVersion(version, versionRange.MaximumTested) <= 0
}

func compareOpenCodeVersion(left, right ir.Version) int {
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
func (a *Adapter) InstallCommands(profile system.PlatformProfile) [][]string {
	if profile.PackageManager == "brew" {
		return [][]string{{"brew", "install", "opencode-ai/tap/opencode"}}
	}
	return [][]string{{"npm", "install", "-g", "opencode-ai"}}
}
