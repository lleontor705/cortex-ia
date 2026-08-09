package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/system"
	"gopkg.in/yaml.v3"
)

// Severity indicates how critical a check failure is.
type Severity int

const (
	SeverityError   Severity = iota // installation broken
	SeverityWarning                 // degraded but functional
)

// Check is a single health check.
type Check struct {
	ID       string
	Name     string
	Severity Severity
	Fn       func(ctx *Context) error
}

// Context carries shared state for all checks.
type Context struct {
	HomeDir  string
	State    state.State
	Lock     state.Lockfile
	Registry *agents.Registry
}

// Result is the outcome of running one check.
type Result struct {
	CheckID  string
	Name     string
	Passed   bool
	Message  string
	Severity Severity
}

// Report aggregates all check results.
type Report struct {
	Results []Result
	Passed  int
	Failed  int
	Warned  int
}

// HasErrors returns true if any error-severity checks failed.
func (r Report) HasErrors() bool { return r.Failed > 0 }

// Run executes all checks and returns a report.
func Run(ctx *Context, checks []Check) Report {
	var r Report
	for _, c := range checks {
		err := c.Fn(ctx)
		res := Result{CheckID: c.ID, Name: c.Name, Severity: c.Severity}
		if err != nil {
			res.Passed = false
			res.Message = err.Error()
			if c.Severity == SeverityError {
				r.Failed++
			} else {
				r.Warned++
			}
		} else {
			res.Passed = true
			r.Passed++
		}
		r.Results = append(r.Results, res)
	}
	return r
}

// DefaultChecks returns the standard set of health checks.
func DefaultChecks() []Check {
	return []Check{
		{ID: "install-status", Name: "Install completed cleanly", Severity: SeverityError, Fn: checkInstallStatus},
		{ID: "files-exist", Name: "Tracked files present", Severity: SeverityError, Fn: checkFilesExist},
		{ID: "cortex-binary", Name: "Cortex MCP binary", Severity: SeverityWarning, Fn: checkCortexBinary},
		{ID: "node-npx", Name: "Node.js and npx available", Severity: SeverityWarning, Fn: checkNodeNpx},
		{ID: "forgespec-binary", Name: "ForgeSpec MCP wrapper", Severity: SeverityWarning, Fn: checkForgeSpecBinary},
		{ID: "skills-present", Name: "Skill files present", Severity: SeverityWarning, Fn: checkSkillsPresent},
		{ID: "convention-present", Name: "Cortex convention file", Severity: SeverityWarning, Fn: checkConventionPresent},
		{ID: "state-lock-consistent", Name: "State and lock consistent", Severity: SeverityWarning, Fn: checkStateLockConsistent},
		{ID: "critical-lock-inventory", Name: "Critical artifacts tracked", Severity: SeverityError, Fn: checkCriticalLockInventory},
		{ID: "orchestrator-frontmatter", Name: "Orchestrator skill frontmatter", Severity: SeverityError, Fn: checkOrchestratorFrontmatter},
		{ID: "opencode-composition", Name: "OpenCode SDD composition", Severity: SeverityError, Fn: checkOpenCodeComposition},
		{ID: "forgespec-version", Name: "Qualified ForgeSpec version", Severity: SeverityError, Fn: checkForgeSpecOpenCodeConfig},
	}
}

func checkInstallStatus(ctx *Context) error {
	status, err := state.LoadInstallStatus(ctx.HomeDir)
	if err != nil {
		return fmt.Errorf("could not read install status: %w", err)
	}
	if status == nil {
		return nil // no marker — clean state
	}
	if status.Status == "in-progress" {
		msg := "previous installation did not complete cleanly (started " + status.StartedAt + ")"
		if status.BackupID != "" {
			msg += "; run 'cortex-ia rollback' to restore backup " + status.BackupID + " or 'cortex-ia repair' to retry"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func checkFilesExist(ctx *Context) error {
	var missing []string
	for _, path := range ctx.Lock.Files {
		fullPath := path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(ctx.HomeDir, fullPath)
		}
		if _, err := os.Stat(fullPath); err != nil && os.IsNotExist(err) {
			missing = append(missing, filepath.Clean(path))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d/%d tracked files missing", len(missing), len(ctx.Lock.Files))
	}
	return nil
}

func checkCortexBinary(ctx *Context) error {
	if _, ok := system.ToolExists("cortex"); !ok {
		return fmt.Errorf("cortex binary not found in PATH")
	}
	return nil
}

func checkNodeNpx(ctx *Context) error {
	var missing []string
	if _, ok := system.ToolExists("node"); !ok {
		missing = append(missing, "node")
	}
	if _, ok := system.ToolExists("npx"); !ok {
		missing = append(missing, "npx")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing: %s (required for MCP servers)", strings.Join(missing, ", "))
	}
	return nil
}

func checkForgeSpecBinary(ctx *Context) error {
	if !componentSelected(ctx, model.ComponentForgeSpec) {
		return nil
	}
	if _, ok := system.ToolExists(forgespeccomp.OpenCodeCommand); !ok {
		return fmt.Errorf("%s wrapper not found in PATH", forgespeccomp.OpenCodeCommand)
	}
	return nil
}

func checkSkillsPresent(ctx *Context) error {
	skillsDir := state.SharedSkillsDir(ctx.HomeDir)
	expected := []string{"bootstrap", "implement", "validate", "architect", "investigate"}
	var missing []string
	for _, id := range expected {
		found := false
		if _, err := os.Stat(filepath.Join(skillsDir, id, "SKILL.md")); err == nil {
			found = true
		}
		if !found {
			for _, a := range ctx.Lock.InstalledAgents {
				if _, err := os.Stat(filepath.Join(ctx.HomeDir, ".config", string(a), "skills", id, "SKILL.md")); err == nil {
					found = true
					break
				}
			}
		}
		if !found {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing skills in %s: %s", skillsDir, strings.Join(missing, ", "))
	}
	return nil
}

func checkConventionPresent(ctx *Context) error {
	path := filepath.Join(state.SharedSkillsDir(ctx.HomeDir), "_shared", "cortex-convention.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if agentSelected(ctx, model.AgentOpenCode) {
		canonical := filepath.Join(ctx.HomeDir, ".cortex-ia", "opencode", "contracts", "_shared", "cortex-convention.md")
		if _, err := os.Stat(canonical); err == nil {
			return nil
		}
	}
	for _, a := range ctx.Lock.InstalledAgents {
		if a == model.AgentOpenCode {
			continue
		}
		if _, err := os.Stat(filepath.Join(ctx.HomeDir, ".config", string(a), "skills", "_shared", "cortex-convention.md")); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cortex-convention.md not found at %s", path)
}

func checkStateLockConsistent(ctx *Context) error {
	if len(ctx.State.InstalledAgents) == 0 && len(ctx.Lock.InstalledAgents) == 0 {
		return nil // both empty is consistent
	}
	stateAgents := make(map[string]bool)
	for _, a := range ctx.State.InstalledAgents {
		stateAgents[string(a)] = true
	}
	lockAgents := make(map[string]bool)
	for _, a := range ctx.Lock.InstalledAgents {
		lockAgents[string(a)] = true
	}
	var diffs []string
	for a := range stateAgents {
		if !lockAgents[a] {
			diffs = append(diffs, fmt.Sprintf("%s in state but not lock", a))
		}
	}
	for a := range lockAgents {
		if !stateAgents[a] {
			diffs = append(diffs, fmt.Sprintf("%s in lock but not state", a))
		}
	}
	if len(diffs) > 0 {
		return fmt.Errorf("state/lock mismatch: %s", strings.Join(diffs, "; "))
	}
	return nil
}

func checkCriticalLockInventory(ctx *Context) error {
	if !agentSelected(ctx, model.AgentOpenCode) {
		return nil
	}

	var critical []string
	configRoot := openCodeConfigRoot(ctx.HomeDir)
	if configPath, ok := existingOpenCodeConfig(configRoot); ok {
		critical = append(critical, configPath)
	}
	if componentSelected(ctx, model.ComponentSDD) {
		critical = appendExisting(critical,
			filepath.Join(ctx.HomeDir, ".cortex-ia", "opencode", "composition.json"),
			filepath.Join(configRoot, "skills", "orchestrator", "SKILL.md"),
		)
	}

	tracked := trackedFileSet(ctx)
	var missing []string
	for _, path := range critical {
		if _, ok := tracked[canonicalPath(path)]; !ok {
			missing = append(missing, displayHomePath(ctx.HomeDir, path))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("critical artifacts missing from lock inventory: %s", strings.Join(missing, ", "))
	}
	return nil
}

func checkOrchestratorFrontmatter(ctx *Context) error {
	if !openCodeSDDSelected(ctx) {
		return nil
	}
	path := filepath.Join(openCodeConfigRoot(ctx.HomeDir), "skills", "orchestrator", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read orchestrator skill: %w", err)
	}
	type frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	var header frontmatter
	if err := decodeFrontmatter(content, &header); err != nil {
		return fmt.Errorf("invalid orchestrator frontmatter: %w", err)
	}
	if header.Name != "orchestrator" || strings.TrimSpace(header.Description) == "" {
		return fmt.Errorf("invalid orchestrator frontmatter: name must be %q and description must be non-empty", "orchestrator")
	}
	return nil
}

func checkOpenCodeComposition(ctx *Context) error {
	if !openCodeSDDSelected(ctx) {
		return nil
	}
	manifestPath := filepath.Join(ctx.HomeDir, ".cortex-ia", "opencode", "composition.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read composition manifest: %w", err)
	}
	type skillBinding struct {
		Path string `json:"path"`
	}
	type compositionManifest struct {
		RootIndex       string         `json:"root_index"`
		Modules         []string       `json:"modules"`
		SkillBindings   []skillBinding `json:"skill_bindings"`
		SharedContract  string         `json:"shared_contract"`
		ProfileOverlay  string         `json:"profile_overlay"`
		QualityTemplate string         `json:"quality_template"`
	}
	var manifest compositionManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return fmt.Errorf("parse composition manifest: %w", err)
	}
	internalReferences := append([]string{manifest.RootIndex, manifest.SharedContract, manifest.ProfileOverlay, manifest.QualityTemplate}, manifest.Modules...)
	for _, reference := range internalReferences {
		if _, err := safeCompositionPath(ctx.HomeDir, reference); err != nil {
			return fmt.Errorf("composition reference %q: %w", reference, err)
		}
		if !strings.HasPrefix(filepath.ToSlash(reference), ".cortex-ia/opencode/") {
			return fmt.Errorf("composition internal reference %q is outside canonical OpenCode state root", reference)
		}
	}
	for _, binding := range manifest.SkillBindings {
		clean := filepath.ToSlash(binding.Path)
		if !strings.HasPrefix(clean, ".config/opencode/skills/") || !strings.HasSuffix(clean, "/SKILL.md") {
			return fmt.Errorf("composition skill reference %q is outside native OpenCode skill root", binding.Path)
		}
	}
	references := []string{manifest.RootIndex, manifest.SharedContract, manifest.ProfileOverlay, manifest.QualityTemplate}
	references = append(references, manifest.Modules...)
	for _, binding := range manifest.SkillBindings {
		references = append(references, binding.Path)
	}
	tracked := trackedFileSet(ctx)
	for _, reference := range references {
		fullPath, err := safeCompositionPath(ctx.HomeDir, reference)
		if err != nil {
			return fmt.Errorf("composition reference %q: %w", reference, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("composition reference %q does not exist", reference)
			}
			return fmt.Errorf("stat composition reference %q: %w", reference, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("composition reference %q is not a regular file", reference)
		}
		if _, ok := tracked[canonicalPath(fullPath)]; !ok {
			return fmt.Errorf("composition reference %q is not registered in lock", reference)
		}
	}
	return nil
}

func checkForgeSpecOpenCodeConfig(ctx *Context) error {
	if !agentSelected(ctx, model.AgentOpenCode) || !componentSelected(ctx, model.ComponentForgeSpec) {
		return nil
	}
	configPath, ok := existingOpenCodeConfig(openCodeConfigRoot(ctx.HomeDir))
	if !ok {
		return fmt.Errorf("OpenCode config not found for selected ForgeSpec component")
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read OpenCode config: %w", err)
	}
	root, err := filemerge.DecodeJSONObject(content)
	if err != nil {
		return fmt.Errorf("parse OpenCode config: %w", err)
	}
	mcp, _ := root["mcp"].(map[string]any)
	forgeSpec, _ := mcp["forgespec"].(map[string]any)
	command, _ := forgeSpec["command"].([]any)
	if len(command) != 1 || command[0] != forgespeccomp.OpenCodeCommand {
		return fmt.Errorf("OpenCode ForgeSpec command must use direct wrapper %s", forgespeccomp.OpenCodeCommand)
	}
	return nil
}

func openCodeSDDSelected(ctx *Context) bool {
	return agentSelected(ctx, model.AgentOpenCode) && componentSelected(ctx, model.ComponentSDD)
}

func agentSelected(ctx *Context, wanted model.AgentID) bool {
	for _, agents := range [][]model.AgentID{ctx.State.InstalledAgents, ctx.Lock.InstalledAgents} {
		for _, agent := range agents {
			if agent == wanted {
				return true
			}
		}
	}
	return false
}

func componentSelected(ctx *Context, wanted model.ComponentID) bool {
	for _, components := range [][]model.ComponentID{ctx.State.Components, ctx.Lock.Components} {
		for _, component := range components {
			if component == wanted {
				return true
			}
		}
	}
	return false
}

func openCodeConfigRoot(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode")
}

func existingOpenCodeConfig(configRoot string) (string, bool) {
	for _, name := range []string{"opencode.jsonc", "opencode.json"} {
		path := filepath.Join(configRoot, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func appendExisting(paths []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
	}
	return paths
}

func trackedFileSet(ctx *Context) map[string]struct{} {
	tracked := make(map[string]struct{}, len(ctx.Lock.Files))
	for _, path := range ctx.Lock.Files {
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.HomeDir, path)
		}
		tracked[canonicalPath(path)] = struct{}{}
	}
	return tracked
}

func canonicalPath(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}

func displayHomePath(homeDir, path string) string {
	relative, err := filepath.Rel(homeDir, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.Clean(path)
}

func decodeFrontmatter(content []byte, target any) error {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return fmt.Errorf("missing opening delimiter")
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return fmt.Errorf("missing closing delimiter")
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), target); err != nil {
		return err
	}
	return nil
}

func safeCompositionPath(root, reference string) (string, error) {
	if strings.TrimSpace(reference) == "" || filepath.IsAbs(reference) || filepath.VolumeName(reference) != "" {
		return "", fmt.Errorf("unsafe relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(reference))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe relative path")
	}
	fullPath := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe relative path")
	}
	return fullPath, nil
}
