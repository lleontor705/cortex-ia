package vscode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	return &Adapter{now: time.Now, runProbe: runVSCodeProbe}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentVSCodeCopilot }
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
	return filepath.Join(homeDir, ".copilot")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "prompts")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "prompts", "cortex-ia.instructions.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".copilot", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(vscodeUserDir(homeDir), "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(vscodeUserDir(homeDir), "mcp.json")
}

func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) CommandsDir(_ string) string { return "" }

// SupportsTaskDelegation returns false because VS Code Copilot does not support
// sub-agents. There is no SubAgentsDir and no task() tool — giving it the
// multi-agent prompt would instruct delegation that has no target.
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

const qualifiedVSCodeVersion = "1.115.0"

var vscodeVersionPattern = regexp.MustCompile(`(?m)^(\d+\.\d+\.\d+)\s*$`)

// CapabilityFacts returns a conservative, version-bounded snapshot. Current
// documentation describes direct-child subagents, but a version response does
// not prove that the preview feature is enabled. The fact therefore remains
// experimental and prompt-enforced, and nesting remains explicitly absent.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedVSCodeVersion),
		MaximumTested: ir.MustParseVersion(qualifiedVSCodeVersion),
	}
	identity := capability.CapabilityFact{
		Target:          "vscode",
		RuntimeID:       "vscode-copilot",
		AdapterID:       "cortex-ia/vscode",
		RuntimeVersions: versions,
		Current:         true,
	}

	direct := identity
	direct.ID = "delegation/direct-child"
	direct.Mode = capability.CapabilityAvailable
	direct.Cardinality = capability.CardinalityMany
	direct.EvidenceClass = capability.EvidenceDocumentation
	direct.EvidenceRef = "https://code.visualstudio.com/docs/agents/subagents"
	direct.ObservedAt = observedAt
	direct.FreshUntil = observedAt.Add(90 * 24 * time.Hour)
	direct.Confidence = 0.8
	direct.Experimental = true
	direct.Enforcement = capability.EnforcementPrompt

	nested := identity
	nested.ID = "delegation/nested"
	nested.Mode = capability.CapabilityAbsent
	nested.Cardinality = capability.CardinalityNone
	nested.Enforcement = capability.EnforcementNone

	return []capability.CapabilityFact{direct, nested}
}

// CapabilityProber verifies only the installed VS Code version. It cannot
// launch agents, infer preview-feature enablement, or upgrade prompt evidence
// to runtime enforcement.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = runVSCodeProbe
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return vscodeVersionProber{run: runner, now: clock}
}

type vscodeVersionProber struct {
	run probeRunner
	now func() time.Time
}

func (p vscodeVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "vscode" || request.Base.RuntimeID != "vscode-copilot" || request.Base.AdapterID != "cortex-ia/vscode" {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: unsupported capability identity %q", request.Base.ID)
	}
	if request.Base.Mode == capability.CapabilityAbsent {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: capability %q is explicitly unsupported", request.Base.ID)
	}

	output, err := p.run(ctx, "code", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: execute version command: %w", err)
	}
	match := vscodeVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: output did not contain a semantic version")
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: invalid semantic version: %w", err)
	}
	if compareVSCodeVersion(version, request.Authority.RuntimeVersions.Minimum) < 0 || compareVSCodeVersion(version, request.Authority.RuntimeVersions.MaximumTested) > 0 {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: runtime version %s is outside supported interval %s", version, request.Authority.RuntimeVersions.String())
	}

	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	result := capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/vscode/version",
			Method:         capability.ProbeCommand,
			Command:        "code --version",
			Result:         "qualified-version:" + version.String(),
			Timestamp:      p.now().UTC(),
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		return capability.ProbeResult{}, fmt.Errorf("VS Code capability probe: refinement blocked: %w", err)
	}
	return result, nil
}

func runVSCodeProbe(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func compareVSCodeVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}

// --- Auto-install ---

func (a *Adapter) SupportsAutoInstall() bool                           { return false }
func (a *Adapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

// vscodeUserDir returns the platform-specific VS Code User directory.
func vscodeUserDir(homeDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Code", "User")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "Code", "User")
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "Code", "User")
	default:
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			return filepath.Join(xdg, "Code", "User")
		}
		return filepath.Join(homeDir, ".config", "Code", "User")
	}
}
