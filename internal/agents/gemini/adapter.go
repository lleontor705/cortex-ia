package gemini

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
	return &Adapter{lookPath: exec.LookPath, runProbe: runGeminiProbe, now: time.Now}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentGeminiCLI }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

func (a *Adapter) Detect(homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)
	binaryPath, err := a.lookPath("gemini")
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
	return filepath.Join(homeDir, ".gemini")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".gemini")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "GEMINI.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".gemini", "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMergeIntoSettings
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".gemini", "settings.json")
}

func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) CommandsDir(_ string) string  { return "" }
func (a *Adapter) SupportsTaskDelegation() bool { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SubAgentsDir(_ string) string { return "" }

const qualifiedGeminiVersion = "0.52.0"

var (
	errUnqualifiedGeminiVersion = errors.New("installed Gemini CLI version is not qualified")
	geminiVersionPattern        = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)
)

// CapabilityFacts describes only the Gemini CLI v0.52.0 agent schema verified
// from the tagged upstream source. Direct-child delegation qualifies the flat
// profile. Tool-scope isolation is runtime-provided but remains experimental,
// so native selection requires an explicit operator opt-in. Gemini subagents
// cannot recursively invoke other subagents.
func (a *Adapter) CapabilityFacts() []capability.CapabilityFact {
	observedAt := time.Date(2026, time.July, 22, 20, 51, 21, 0, time.UTC)
	freshUntil := observedAt.Add(90 * 24 * time.Hour)
	versions := ir.VersionRange{
		Minimum:       ir.MustParseVersion(qualifiedGeminiVersion),
		MaximumTested: ir.MustParseVersion(qualifiedGeminiVersion),
	}

	direct := geminiSchemaFact(
		"delegation/direct-child",
		"https://github.com/google-gemini/gemini-cli/blob/v0.52.0/docs/core/subagents.md#what-are-subagents",
		"agent definitions are exposed as direct-child tools",
		false,
		versions,
		observedAt,
		freshUntil,
	)
	isolation := geminiSchemaFact(
		"isolation/tool-scope",
		"https://github.com/google-gemini/gemini-cli/blob/v0.52.0/docs/core/subagents.md#subagent-tool-isolation",
		"agent frontmatter scopes tools and inline MCP servers",
		true,
		versions,
		observedAt,
		freshUntil,
	)
	nested := capability.CapabilityFact{
		ID:              "delegation/nested",
		Mode:            capability.CapabilityAbsent,
		Cardinality:     capability.CardinalityNone,
		Target:          "gemini",
		RuntimeID:       "gemini-cli",
		AdapterID:       "cortex-ia/gemini",
		RuntimeVersions: versions,
		Current:         true,
		Enforcement:     capability.EnforcementNone,
	}
	return []capability.CapabilityFact{direct, isolation, nested}
}

func geminiSchemaFact(
	id capability.CapabilityID,
	evidenceRef string,
	result string,
	experimental bool,
	versions ir.VersionRange,
	observedAt time.Time,
	freshUntil time.Time,
) capability.CapabilityFact {
	digest := sha256.Sum256([]byte("gemini-cli-agent-schema/v0.52.0:" + string(id) + ":" + result))
	return capability.CapabilityFact{
		ID:              id,
		Mode:            capability.CapabilityAvailable,
		Cardinality:     capability.CardinalityMany,
		Target:          "gemini",
		RuntimeID:       "gemini-cli",
		AdapterID:       "cortex-ia/gemini",
		RuntimeVersions: versions,
		EvidenceClass:   capability.EvidenceInstalledSchema,
		EvidenceRef:     evidenceRef,
		ObservedAt:      observedAt,
		FreshUntil:      freshUntil,
		Confidence:      0.9,
		Experimental:    experimental,
		Current:         true,
		Probe: &capability.ProbeRecord{
			ID:             "probe/gemini-agent-schema",
			Method:         capability.ProbeProtocol,
			Protocol:       "gemini-cli-agent-schema/v0.52.0",
			Result:         result,
			Timestamp:      observedAt,
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Enforcement: capability.EnforcementRuntime,
	}
}

// CapabilityProber qualifies the installed CLI version without launching an
// agent or claiming authority outside the tagged schema interval.
func (a *Adapter) CapabilityProber() capability.Prober {
	runner := a.runProbe
	if runner == nil {
		runner = runGeminiProbe
	}
	clock := a.now
	if clock == nil {
		clock = time.Now
	}
	return geminiVersionProber{run: runner, now: clock}
}

type geminiVersionProber struct {
	run func(context.Context, string, ...string) ([]byte, error)
	now func() time.Time
}

func (p geminiVersionProber) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	if request.Base.Target != "gemini" || request.Base.RuntimeID != "gemini-cli" || request.Base.AdapterID != "cortex-ia/gemini" ||
		(request.Base.ID != "delegation/direct-child" && request.Base.ID != "isolation/tool-scope") || request.Authority.CapabilityID != request.Base.ID {
		return capability.ProbeResult{}, fmt.Errorf("%w: fact does not belong to the Gemini adapter", errUnqualifiedGeminiVersion)
	}
	output, err := p.run(ctx, "gemini", "--version")
	if err != nil {
		return capability.ProbeResult{}, fmt.Errorf("probe Gemini CLI version: %w", err)
	}
	match := geminiVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return capability.ProbeResult{}, fmt.Errorf("%w: version output did not contain semantic version", errUnqualifiedGeminiVersion)
	}
	version, err := ir.ParseVersion(string(match[1]))
	if err != nil || !geminiVersionInRange(version, request.Base.RuntimeVersions) {
		return capability.ProbeResult{}, fmt.Errorf("%w: %s", errUnqualifiedGeminiVersion, match[1])
	}

	digest := sha256.Sum256([]byte(strings.TrimSpace(string(output))))
	refined := request.Base
	refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
	return capability.ProbeResult{
		Record: capability.ProbeRecord{
			ID:             "probe/gemini-version",
			Method:         capability.ProbeCommand,
			Command:        "gemini --version",
			Result:         "qualified-version:" + version.String(),
			Timestamp:      p.now().UTC(),
			EvidenceDigest: "sha256:" + hex.EncodeToString(digest[:]),
		},
		Refined: refined,
	}, nil
}

func runGeminiProbe(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func geminiVersionInRange(version ir.Version, interval ir.VersionRange) bool {
	return compareGeminiVersion(interval.Minimum, version) <= 0 && compareGeminiVersion(version, interval.MaximumTested) <= 0
}

func compareGeminiVersion(left, right ir.Version) int {
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
