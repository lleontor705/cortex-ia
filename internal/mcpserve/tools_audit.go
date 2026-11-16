package mcpserve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const gitDiffTimeout = 10 * time.Second

var (
	testPathPattern   = regexp.MustCompile(`(?i)(_test\.|[._]test\.|[._]spec\.|[/\\]tests?[/\\]|[/\\]fixtures?[/\\])`)
	schemaDocPattern  = regexp.MustCompile(`(?i)(\.md|\.json|\.ya?ml|\.sql)$`)
	maxGitDiffOutputB = 1 << 20
)

type diffFile struct {
	Path     string `json:"path"`
	Added    int    `json:"added"`
	Deleted  int    `json:"deleted"`
	Category string `json:"category"`
}

func workloadAudit(ctx context.Context, s *session, args map[string]any) (string, error) {
	policy := optString(args, "policy")
	if policy == "" {
		policy = "flexible"
	}
	if policy != "strict" && policy != "flexible" && policy != "unbounded" {
		return "", errors.New("policy must be strict, flexible or unbounded")
	}
	baseRef := optString(args, "baseRef")
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if strings.HasPrefix(baseRef, "-") {
		return "", errors.New("baseRef must be a git revision, not an option")
	}
	files, err := s.numstat(ctx, baseRef)
	if err != nil {
		return "", err
	}
	return encodeJSON(auditResult(files, policy))
}

// numstat fails closed like the plugin: an empty list from a failed diff would
// read as a passing budget.
func (s *session) numstat(ctx context.Context, baseRef string) ([]diffFile, error) {
	runCtx, cancel := context.WithTimeout(ctx, gitDiffTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "git", "diff", "--numstat", baseRef, "--")
	cmd.Dir = s.workspace
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("workload audit: git diff failed: %w", err)
	}
	if stdout.Len() > maxGitDiffOutputB {
		return nil, errors.New("workload audit: git diff output exceeds the audit limit")
	}
	var files []diffFile
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		name := strings.TrimSpace(parts[2])
		category := "source"
		switch {
		case testPathPattern.MatchString(name):
			category = "test"
		case schemaDocPattern.MatchString(name):
			category = "schema_doc"
		}
		files = append(files, diffFile{Path: name, Added: added, Deleted: deleted, Category: category})
	}
	return files, nil
}

// auditResult applies the plugin's budget rules. Unbounded caps are reported
// as null, matching JSON.stringify(Infinity).
func auditResult(files []diffFile, policy string) map[string]any {
	var logicAdded, logicDeleted, testAdded, testDeleted int
	for _, f := range files {
		switch f.Category {
		case "test":
			testAdded += f.Added
			testDeleted += f.Deleted
		case "source":
			logicAdded += f.Added
			logicDeleted += f.Deleted
		}
	}
	weightedLogic := int(math.Round(float64(logicAdded) + float64(logicDeleted)*0.2))
	totalTest := testAdded + testDeleted
	maxLogic, maxTest := math.Inf(1), math.Inf(1)
	switch policy {
	case "strict":
		maxLogic, maxTest = 350, 600
	case "flexible":
		maxLogic, maxTest = 700, 1200
	}
	status := "PASS"
	switch policy {
	case "strict":
		if float64(weightedLogic) > maxLogic {
			status = "WORKLOAD_SOURCE_BUDGET_EXCEEDED"
		} else if float64(totalTest) > maxTest {
			status = "WORKLOAD_TEST_BUDGET_EXCEEDED"
		}
	case "flexible":
		if float64(weightedLogic) > maxLogic || float64(totalTest) > maxTest {
			status = "EXCEEDED_ADVISORY"
		}
	}
	if files == nil {
		files = []diffFile{}
	}
	return map[string]any{
		"workload_status": status,
		"policy":          policy,
		"logic_lines":     weightedLogic,
		"test_lines":      totalTest,
		"total_weighted":  weightedLogic + totalTest,
		"max_logic_cap":   capValue(maxLogic),
		"max_test_cap":    capValue(maxTest),
		"files_changed":   len(files),
		"files":           files,
	}
}

func capValue(value float64) any {
	if math.IsInf(value, 1) {
		return nil
	}
	return int(value)
}
