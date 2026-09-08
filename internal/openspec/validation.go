// Package openspec validates planning structure. It never approves semantics or work.
package openspec

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type Options struct {
	Workflow string `json:"workflow"`
	Phase    string `json:"phase"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Result struct {
	Valid                  bool         `json:"valid"`
	StructuralOnly         bool         `json:"structural_only"`
	SemanticReviewRequired bool         `json:"semantic_review_required"`
	Workflow               string       `json:"workflow"`
	Phase                  string       `json:"phase"`
	ChangeID               string       `json:"change_id"`
	Requirements           int          `json:"requirements"`
	Scenarios              int          `json:"scenarios"`
	Tasks                  int          `json:"tasks"`
	TaskIDs                []string     `json:"task_ids"`
	Errors                 []Diagnostic `json:"errors"`
}

var requirementID = regexp.MustCompile(`\bREQ-[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]{3}\b`)
var taskLine = regexp.MustCompile(`^[-*] \[[ xX]\]\s+([A-Za-z0-9][A-Za-z0-9_.-]{0,127})(?:\s|$)`)
var comments = regexp.MustCompile(`(?s)<!--.*?-->`)

// ChangeDirectory only resolves active, workspace-owned change directories.
func ChangeDirectory(workspace, target string) (string, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	base := filepath.Join(root, "openspec", "changes")
	if target == "" {
		return "", fmt.Errorf("an explicit OpenSpec change is required")
	}
	if filepath.IsAbs(target) {
		return "", fmt.Errorf("change must be workspace-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(target))
	if !strings.ContainsAny(clean, `/\`) {
		clean = filepath.Join("openspec", "changes", clean)
	}
	candidate := filepath.Join(root, clean)
	rel, err := filepath.Rel(base, candidate)
	if err != nil || rel == "." || strings.ContainsAny(rel, `/\`) || rel == "archive" || strings.HasPrefix(rel, ".") {
		return "", fmt.Errorf("change must identify one active openspec/changes directory")
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve change: %w", err)
	}
	if !samePath(resolved, candidate) {
		return "", fmt.Errorf("OpenSpec change may not traverse symbolic links")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("change is not a directory")
	}
	return resolved, nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// Validate requires only the selected phase and its prerequisites. REMOVED
// requirements require a reason instead of scenarios; semantic review remains external.
func Validate(workspace, target string, opts Options) (Result, error) {
	r := Result{StructuralOnly: true, SemanticReviewRequired: true, Workflow: opts.Workflow, Phase: opts.Phase, Errors: []Diagnostic{}}
	phases := map[string]int{"propose": 0, "spec": 1, "design": 2, "tasks": 3}
	level, fullPhase := phases[opts.Phase]
	validPair := opts.Workflow == "sdd-full" && fullPhase || opts.Workflow == "sdd-lite" && opts.Phase == "integrated" || opts.Workflow == "decision-map" && (opts.Phase == "chart" || opts.Phase == "resolve")
	if !validPair {
		return r, fmt.Errorf("unsupported workflow/phase %q/%q", opts.Workflow, opts.Phase)
	}
	dir, err := ChangeDirectory(workspace, target)
	if err != nil {
		return r, err
	}
	r.ChangeID = filepath.Base(dir)
	add := func(code, path string, line int, message string) {
		r.Errors = append(r.Errors, Diagnostic{code, path, line, message})
	}
	read := func(relative string) string {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			add("artifact_missing", relative, 0, err.Error())
			return ""
		}
		if !samePath(path, resolved) {
			add("artifact_path", relative, 0, "artifact may not traverse symbolic links")
			return ""
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			add("artifact_invalid", relative, 0, "artifact must be a regular file of at most 1 MiB")
			return ""
		}
		file, err := os.Open(path)
		if err != nil {
			add("artifact_read", relative, 0, err.Error())
			return ""
		}
		bytes, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
		closeErr := file.Close()
		if err != nil {
			add("artifact_read", relative, 0, err.Error())
			return ""
		}
		if closeErr != nil {
			add("artifact_read", relative, 0, closeErr.Error())
			return ""
		}
		if len(bytes) > 1024*1024 {
			add("artifact_invalid", relative, 0, "artifact exceeds 1 MiB")
			return ""
		}
		text := comments.ReplaceAllStringFunc(string(bytes), func(s string) string { return strings.Repeat("\n", strings.Count(s, "\n")) })
		meaningful := false
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				meaningful = true
			}
		}
		if !meaningful {
			add("artifact_empty", relative, 0, "artifact has no content beyond headings/comments")
		}
		return text
	}
	if opts.Workflow == "decision-map" {
		text := read("decision-map.md")
		for _, heading := range []string{"Destination", "Decisions so far", "Decision frontier", "Not yet specified", "Out of scope"} {
			if !hasSection(text, heading) {
				add("section_missing", "decision-map.md", 0, "missing section: "+heading)
			}
		}
		for _, line := range strings.Split(text, "\n") {
			if taskLine.MatchString(strings.TrimSpace(line)) {
				add("decision_map_dag", "decision-map.md", 0, "decision-map may not define implementation checklist tasks")
				break
			}
		}
		if _, err := os.Lstat(filepath.Join(dir, "tasks.md")); err == nil {
			add("decision_map_dag", "tasks.md", 0, "decision-map cannot include an implementation DAG artifact")
		} else if !os.IsNotExist(err) {
			add("artifact_read", "tasks.md", 0, err.Error())
		}
		r.Valid = len(r.Errors) == 0
		return r, nil
	}
	documents := map[string]string{}
	taskText, taskPath := "", "tasks.md"
	if opts.Workflow == "sdd-lite" {
		text := read("plan.md")
		for _, heading := range []string{"Intent", "Requirements", "Design", "Tasks"} {
			if !hasSection(text, heading) {
				add("section_missing", "plan.md", 0, "missing section: "+heading)
			}
		}
		documents["plan.md"] = text
		taskText, taskPath = text, "plan.md"
	} else {
		read("proposal.md")
		if level >= 1 {
			specs := filepath.Join(dir, "specs")
			err := filepath.WalkDir(specs, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("symbolic link in specs")
				}
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					if len(documents) >= 100 {
						return fmt.Errorf("at most 100 specification files are allowed")
					}
					rel, _ := filepath.Rel(dir, path)
					documents[filepath.ToSlash(rel)] = read(rel)
				}
				return nil
			})
			if err != nil {
				add("specs_read", "specs", 0, err.Error())
			}
			if len(documents) == 0 {
				add("specs_missing", "specs", 0, "at least one delta specification is required")
			}
		}
		if level >= 2 {
			read("design.md")
		}
		if level >= 3 {
			taskText = read(taskPath)
		}
	}
	requirements := map[string]bool{}
	paths := make([]string, 0, len(documents))
	for p := range documents {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		inspectRequirements(p, documents[p], requirements, &r, add)
	}
	if len(documents) > 0 && len(requirements) == 0 {
		add("requirements_missing", "", 0, "at least one machine-readable Requirement: REQ-DOMAIN-001 is required")
	}
	if opts.Workflow == "sdd-lite" || level >= 3 {
		inspectTasks(taskPath, taskText, requirements, &r, add)
	}
	r.Requirements = len(requirements)
	r.Valid = len(r.Errors) == 0
	return r, nil
}

func hasSection(text, name string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "## "+strings.ToLower(name)) {
			return true
		}
	}
	return false
}

func inspectRequirements(path, text string, ids map[string]bool, r *Result, add func(string, string, int, string)) {
	current, delta := "", ""
	removed := false
	count, scenarioLine := 0, 0
	steps := []string{}
	hasReason := false
	finishScenario := func() {
		if scenarioLine != 0 {
			if strings.Join(steps, ",") != "GIVEN,WHEN,THEN" {
				add("scenario_incomplete", path, scenarioLine, "scenario must contain ordered nonempty GIVEN, WHEN and THEN")
			}
			r.Scenarios++
		}
	}
	finish := func() {
		finishScenario()
		scenarioLine = 0
		steps = nil
		if current != "" {
			if removed && !hasReason {
				add("removal_reason", path, 0, current+" requires a nonempty Reason: for removal")
			}
			if !removed && count < 3 {
				add("scenario_count", path, 0, current+" requires at least three complete scenarios")
			}
		}
	}
	fenced := false
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			finish()
			current = ""
			delta = line
			continue
		}
		if strings.HasPrefix(line, "### Requirement:") {
			finish()
			body := strings.TrimSpace(strings.TrimPrefix(line, "### Requirement:"))
			current = requirementID.FindString(body)
			removed = strings.Contains(delta, "REMOVED")
			count = 0
			hasReason = false
			if current != "" && !strings.HasPrefix(body, current) {
				current = ""
			}
			if current == "" {
				add("requirement_id", path, index+1, "expected Requirement: REQ-DOMAIN-001")
				continue
			}
			if path != "plan.md" && delta != "## ADDED Requirements" && delta != "## MODIFIED Requirements" && delta != "## REMOVED Requirements" {
				add("delta_header", path, index+1, "requirement must be under ADDED, MODIFIED or REMOVED Requirements")
			}
			if ids[current] {
				add("requirement_duplicate", path, index+1, "duplicate "+current)
			}
			ids[current] = true
			continue
		}
		if current == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimPrefix(line, "("), "Reason:") && len(strings.Trim(strings.TrimPrefix(strings.TrimPrefix(line, "("), "Reason:"), " )")) > 0 {
			hasReason = true
		}
		if strings.HasPrefix(line, "#### Scenario:") {
			finishScenario()
			steps = nil
			scenarioLine = index + 1
			count++
			if strings.TrimSpace(strings.TrimPrefix(line, "#### Scenario:")) == "" {
				add("scenario_title", path, index+1, "scenario requires a title")
			}
			continue
		}
		if scenarioLine == 0 {
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "-* "))
		line = strings.ReplaceAll(line, "**", "")
		for _, step := range []string{"GIVEN", "WHEN", "THEN"} {
			if strings.HasPrefix(line, step+" ") && strings.TrimSpace(strings.TrimPrefix(line, step)) != "" {
				steps = append(steps, step)
			}
		}
	}
	finish()
}

func inspectTasks(path, text string, requirements map[string]bool, r *Result, add func(string, string, int, string)) {
	ids, covered := map[string]bool{}, map[string]bool{}
	current := ""
	refs := false
	fenced := false
	finish := func() {
		if current != "" && !refs {
			add("task_trace_missing", path, 0, current+" requires Requirements: REQ-DOMAIN-001 references")
		}
	}
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if match := taskLine.FindStringSubmatch(line); match != nil {
			finish()
			current = match[1]
			refs = false
			r.Tasks++
			r.TaskIDs = append(r.TaskIDs, current)
			if ids[current] {
				add("task_duplicate", path, index+1, "duplicate task "+current)
			}
			ids[current] = true
			continue
		}
		if current != "" && strings.HasPrefix(line, "Requirements:") {
			values := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Requirements:")), ",")
			matches := []string{}
			for _, value := range values {
				value = strings.TrimSpace(value)
				if requirementID.FindString(value) != value || value == "" {
					add("task_trace_invalid", path, index+1, "Requirements must be a comma-separated list of exact REQ IDs")
				} else {
					matches = append(matches, value)
				}
			}
			refs = len(matches) > 0
			for _, id := range matches {
				if !requirements[id] {
					add("task_trace_unknown", path, index+1, "unknown requirement "+id)
				}
				covered[id] = true
			}
		}
	}
	finish()
	if r.Tasks == 0 {
		add("tasks_missing", path, 0, "at least one task with an ID and requirement references is required")
	}
	keys := make([]string, 0, len(requirements))
	for id := range requirements {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if !covered[id] {
			add("requirement_uncovered", path, 0, id+" has no task reference")
		}
	}
}
