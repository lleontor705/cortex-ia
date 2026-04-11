package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/system"
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
		{ID: "skills-present", Name: "Skill files present", Severity: SeverityWarning, Fn: checkSkillsPresent},
		{ID: "convention-present", Name: "Cortex convention file", Severity: SeverityWarning, Fn: checkConventionPresent},
		{ID: "state-lock-consistent", Name: "State and lock consistent", Severity: SeverityWarning, Fn: checkStateLockConsistent},
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
		if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
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

func checkSkillsPresent(ctx *Context) error {
	skillsDir := state.SharedSkillsDir(ctx.HomeDir)
	expected := []string{"bootstrap", "implement", "validate", "architect", "investigate"}
	var missing []string
	for _, id := range expected {
		path := filepath.Join(skillsDir, id, "SKILL.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("cortex-convention.md not found at %s", path)
	}
	return nil
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
