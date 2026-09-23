package delegation

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	workloadAdvisoryDetail = "WORKLOAD_ADVISORY"
	tsPythonDeletionWeight = 0.2
	workloadChurnTimeout   = 15 * time.Second

	strictPlainSourceCap    = 350
	strictScriptSourceCap   = 250
	strictTestCap           = 600
	flexiblePlainSourceCap  = 700
	flexibleScriptSourceCap = 500
	flexibleTestCap         = 1200
)

var (
	ErrWorkloadSourceBudgetExceeded = errors.New("WORKLOAD_SOURCE_BUDGET_EXCEEDED")
	ErrWorkloadTestBudgetExceeded   = errors.New("WORKLOAD_TEST_BUDGET_EXCEEDED")
	ErrWorkloadBudgetUnverifiable   = errors.New("WORKLOAD_BUDGET_UNVERIFIABLE")
)

// workloadChurn is the classified churn of a task workspace. plainSource holds
// Go/Rust/Java/C# and unclassified source at 1x; scriptSource holds TS/Python/JS
// source whose deletions are discounted; test holds every test/tooling file.
type workloadChurn struct {
	plainSource  int
	scriptSource int
	test         int
}

type workloadCaps struct {
	plainSource  int
	scriptSource int
	test         int
}

func workloadCapsFor(policy WorkloadPolicy) (workloadCaps, bool) {
	switch policy {
	case WorkloadPolicyStrict:
		return workloadCaps{plainSource: strictPlainSourceCap, scriptSource: strictScriptSourceCap, test: strictTestCap}, true
	case WorkloadPolicyFlexible:
		return workloadCaps{plainSource: flexiblePlainSourceCap, scriptSource: flexibleScriptSourceCap, test: flexibleTestCap}, true
	default:
		return workloadCaps{}, false
	}
}

func (c workloadChurn) budgetError(caps workloadCaps) error {
	if c.plainSource > caps.plainSource || c.scriptSource > caps.scriptSource {
		return fmt.Errorf("%w: source churn %d exceeds cap (plain<=%d, ts/python<=%d)",
			ErrWorkloadSourceBudgetExceeded, c.plainSource+c.scriptSource, caps.plainSource, caps.scriptSource)
	}
	if c.test > caps.test {
		return fmt.Errorf("%w: test churn %d exceeds cap %d", ErrWorkloadTestBudgetExceeded, c.test, caps.test)
	}
	return nil
}

// parseNumStat classifies `git diff HEAD --numstat` output. Binary entries report
// '-' for both counts and are skipped without contributing to either bucket.
func parseNumStat(raw string) workloadChurn {
	var churn workloadChurn
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 3)
		if len(fields) != 3 {
			continue
		}
		added, addErr := strconv.Atoi(strings.TrimSpace(fields[0]))
		deleted, delErr := strconv.Atoi(strings.TrimSpace(fields[1]))
		if addErr != nil || delErr != nil {
			continue
		}
		churn.add(numStatPath(fields[2]), added, deleted)
	}
	return churn
}

func (c *workloadChurn) add(path string, added, deleted int) {
	path = strings.ToLower(numStatPath(path))
	if path == "" || isDeclarativeWorkloadPath(path) {
		return
	}
	if isTestOrToolingPath(path) {
		c.test += weightedWorkloadLines(path, added, deleted)
		return
	}
	if isScriptSourcePath(path) {
		c.scriptSource += weightedWorkloadLines(path, added, deleted)
		return
	}
	c.plainSource += weightedWorkloadLines(path, added, deleted)
}

func weightedWorkloadLines(path string, added, deleted int) int {
	if isScriptSourcePath(path) {
		return added + int(math.Round(float64(deleted)*tsPythonDeletionWeight))
	}
	return added + deleted
}

func isScriptSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py":
		return true
	default:
		return false
	}
}

func isDeclarativeWorkloadPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".yaml", ".yml", ".toml", ".sql", ".lock", ".sum":
		return true
	}
	switch strings.ToLower(filepath.Base(path)) {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "cargo.lock", "composer.lock", "poetry.lock":
		return true
	default:
		return false
	}
}

// numStatPath normalizes rename entries ("old => new", optionally braced) to the
// destination path so classification keys off the file that survived the diff.
func numStatPath(raw string) string {
	path := strings.TrimSpace(raw)
	if idx := strings.LastIndex(path, "=>"); idx >= 0 {
		path = strings.TrimSpace(path[idx+2:])
	}
	return filepath.ToSlash(strings.Trim(strings.TrimSpace(path), `"{}`))
}

// workloadNumStatCommand is the single workspace-churn seam so tests can prove an
// unbounded policy never pays for the git computation.
var workloadNumStatCommand = func(ctx context.Context, workspace string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", "-C", workspace, "diff", "HEAD", "--numstat")
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("read workspace churn: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func readWorkloadChurn(ctx context.Context, workspace string) (workloadChurn, error) {
	if strings.TrimSpace(workspace) == "" {
		return workloadChurn{}, errors.New("task workspace is required to compute churn")
	}
	churnCtx, cancel := context.WithTimeout(ctx, workloadChurnTimeout)
	defer cancel()
	out, err := workloadNumStatCommand(churnCtx, workspace)
	if err != nil {
		return workloadChurn{}, err
	}
	return parseNumStat(string(out)), nil
}

// resolveWorkloadBudget applies the task policy to its workspace churn. Strict
// tasks fail closed on an exceeded cap or an unverifiable workspace; flexible
// tasks never block and report an advisory; unbounded tasks skip churn entirely.
func resolveWorkloadBudget(ctx context.Context, policy WorkloadPolicy, workspace string) (bool, error) {
	caps, enforced := workloadCapsFor(policy)
	if !enforced {
		return false, nil
	}
	churn, churnErr := readWorkloadChurn(ctx, workspace)
	if churnErr != nil {
		if policy == WorkloadPolicyStrict {
			return false, fmt.Errorf("%w: %v", ErrWorkloadBudgetUnverifiable, churnErr)
		}
		return false, nil
	}
	if budgetErr := churn.budgetError(caps); budgetErr != nil {
		if policy == WorkloadPolicyStrict {
			return false, budgetErr
		}
		return true, nil
	}
	return false, nil
}
