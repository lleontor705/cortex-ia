package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runOpenSpec(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia openspec <subcommand> [options]")
		fmt.Println("\nOpenSpec SDD (Spec-Driven Development) workflow manager:")
		fmt.Println("  validate [change-dir]       Validate proposal, specs, design and tasks")
		fmt.Println("  list                        List active changes in openspec/changes/")
		fmt.Println("  status [change-name]        Show task progress and status of changes")
		fmt.Println("  archive <change-name>       Archive a completed change to openspec/changes/archive/")
		fmt.Println("  new <change-name> [domain]  Scaffold a new OpenSpec change directory")
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "validate":
		target := ""
		if len(args) > 1 {
			target = args[1]
		}
		return validateOpenSpec(target)
	case "list":
		return listOpenSpec()
	case "status":
		target := ""
		if len(args) > 1 {
			target = args[1]
		}
		return statusOpenSpec(target)
	case "archive":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia openspec archive <change-name>")
		}
		return archiveOpenSpec(args[1])
	case "new":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia openspec new <change-name> [domain]")
		}
		domain := "core"
		if len(args) > 2 {
			domain = args[2]
		}
		return newOpenSpec(args[1], domain)
	default:
		return fmt.Errorf("unknown openspec subcommand %q (see 'cortex-ia openspec --help')", args[0])
	}
}

// resolveChangeDir determines if target refers to a specific change directory.
func resolveChangeDir(target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || target == "." || target == "openspec/changes" || target == "openspec/changes/" {
		return "", false
	}

	// 1. Direct path check
	if fi, err := os.Stat(target); err == nil && fi.IsDir() {
		if fileExists(filepath.Join(target, "proposal.md")) || fileExists(filepath.Join(target, "tasks.md")) {
			return target, true
		}
	}

	// 2. Relative to openspec/changes
	candidate := filepath.Join("openspec", "changes", target)
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return candidate, true
	}

	return "", false
}

type changeValidationResult struct {
	Name        string
	Dir         string
	HasProposal bool
	HasTasks    bool
	HasDesign   bool
	SpecsCount  int
	TotalTasks  int
	DoneTasks   int
	Errors      []string
	Warnings    []string
}

func validateSingleChange(changeDir string) changeValidationResult {
	res := changeValidationResult{
		Name: filepath.Base(changeDir),
		Dir:  changeDir,
	}

	proposalPath := filepath.Join(changeDir, "proposal.md")
	tasksPath := filepath.Join(changeDir, "tasks.md")
	designPath := filepath.Join(changeDir, "design.md")
	specsDir := filepath.Join(changeDir, "specs")

	// 1. Validate proposal.md (Required)
	if !fileExists(proposalPath) {
		res.Errors = append(res.Errors, "missing proposal.md")
	} else if !fileNonEmpty(proposalPath) {
		res.Errors = append(res.Errors, "proposal.md is empty (0 bytes)")
	} else {
		res.HasProposal = true
	}

	// 2. Validate tasks.md (Required)
	if !fileExists(tasksPath) {
		res.Errors = append(res.Errors, "missing tasks.md")
	} else if !fileNonEmpty(tasksPath) {
		res.Errors = append(res.Errors, "tasks.md is empty (0 bytes)")
	} else {
		res.HasTasks = true
		res.TotalTasks, res.DoneTasks = parseTasksProgress(tasksPath)
		if res.TotalTasks == 0 {
			res.Warnings = append(res.Warnings, "tasks.md has no checklist items (- [ ] / - [x])")
		}
	}

	// 3. Validate design.md (Optional but checked if present)
	if fileExists(designPath) {
		if !fileNonEmpty(designPath) {
			res.Warnings = append(res.Warnings, "design.md exists but is empty (0 bytes)")
		} else {
			res.HasDesign = true
		}
	}

	// 4. Validate specs/ (Delta specifications)
	if fi, err := os.Stat(specsDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(specsDir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
				return nil
			}
			res.SpecsCount++
			if info.Size() == 0 {
				res.Errors = append(res.Errors, fmt.Sprintf("spec %s is empty (0 bytes)", filepath.Base(p)))
				return nil
			}

			// Inspect delta specification requirements and scenarios
			content, readErr := os.ReadFile(p)
			if readErr == nil {
				text := string(content)
				hasDeltaHeader := strings.Contains(text, "Requirements") ||
					strings.Contains(text, "ADDED") ||
					strings.Contains(text, "MODIFIED") ||
					strings.Contains(text, "REMOVED")
				if !hasDeltaHeader {
					res.Warnings = append(res.Warnings, fmt.Sprintf("spec %s lacks standard ADDED/MODIFIED/REMOVED headers", filepath.Base(p)))
				}
				hasBDD := strings.Contains(text, "Scenario") ||
					strings.Contains(text, "GIVEN") ||
					strings.Contains(text, "WHEN") ||
					strings.Contains(text, "THEN")
				if !hasBDD {
					res.Warnings = append(res.Warnings, fmt.Sprintf("spec %s lacks verifiable GIVEN/WHEN/THEN scenarios", filepath.Base(p)))
				}
			}
			return nil
		})
	}

	return res
}

func printValidationResult(res changeValidationResult) {
	fmt.Printf("🔍 Checking OpenSpec change: %s\n", res.Name)
	if res.HasProposal {
		fmt.Printf("  ✓ proposal.md found\n")
	}
	if res.HasDesign {
		fmt.Printf("  ✓ design.md found\n")
	}
	if res.HasTasks {
		fmt.Printf("  ✓ tasks.md found (%d/%d tasks completed)\n", res.DoneTasks, res.TotalTasks)
	}
	if res.SpecsCount > 0 {
		fmt.Printf("  ✓ %d delta specification file(s) found in specs/\n", res.SpecsCount)
	} else {
		fmt.Printf("  ⚠️ no delta specifications found under specs/\n")
	}

	for _, warn := range res.Warnings {
		fmt.Printf("  ⚠️ %s\n", warn)
	}
	for _, errStr := range res.Errors {
		fmt.Printf("  ❌ %s\n", errStr)
	}
}

func validateOpenSpec(target string) error {
	// Case 1: Specific change targeted
	if changeDir, isSingle := resolveChangeDir(target); isSingle {
		res := validateSingleChange(changeDir)
		printValidationResult(res)
		if len(res.Errors) > 0 {
			return fmt.Errorf("OpenSpec validation failed for %s: %s", res.Name, strings.Join(res.Errors, ", "))
		}
		fmt.Printf("\n✅ OpenSpec validation passed for %s\n", res.Name)
		return nil
	}

	// Case 2: Validate all active changes under openspec/changes
	baseDir := "openspec/changes"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("✓ OpenSpec workspace valid (no changes in %s yet)\n", baseDir)
			return nil
		}
		return err
	}

	totalChanges := 0
	failedChanges := 0

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "archive" || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		changeDir := filepath.Join(baseDir, entry.Name())
		res := validateSingleChange(changeDir)
		printValidationResult(res)
		totalChanges++
		if len(res.Errors) > 0 {
			failedChanges++
		}
	}

	if totalChanges == 0 {
		fmt.Printf("✓ OpenSpec workspace valid (no active changes in %s)\n", baseDir)
		return nil
	}

	if failedChanges > 0 {
		return fmt.Errorf("OpenSpec validation failed (%d of %d change sets have errors)", failedChanges, totalChanges)
	}

	fmt.Printf("\n✅ OpenSpec validation complete (%d active change sets inspected, all valid)\n", totalChanges)
	return nil
}

func listOpenSpec() error {
	baseDir := "openspec/changes"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No active changes found in openspec/changes/")
			return nil
		}
		return err
	}
	fmt.Printf("Active OpenSpec changes in %s:\n", baseDir)
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "archive" && !strings.HasPrefix(entry.Name(), ".") {
			fmt.Printf("  - %s\n", entry.Name())
			count++
		}
	}
	if count == 0 {
		fmt.Println("  (none)")
	}
	return nil
}

func statusOpenSpec(target string) error {
	baseDir := "openspec/changes"

	var changeDirs []string
	if changeDir, isSingle := resolveChangeDir(target); isSingle {
		changeDirs = append(changeDirs, changeDir)
	} else {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No active changes found in openspec/changes/")
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "archive" && !strings.HasPrefix(entry.Name(), ".") {
				changeDirs = append(changeDirs, filepath.Join(baseDir, entry.Name()))
			}
		}
	}

	if len(changeDirs) == 0 {
		fmt.Println("No active OpenSpec changes to report status on.")
		return nil
	}

	fmt.Printf("OpenSpec Changes Status (%d active):\n\n", len(changeDirs))
	for _, dir := range changeDirs {
		res := validateSingleChange(dir)
		statusIcon := "🟢"
		if len(res.Errors) > 0 {
			statusIcon = "🔴"
		} else if len(res.Warnings) > 0 {
			statusIcon = "🟡"
		}

		pct := 0
		if res.TotalTasks > 0 {
			pct = (res.DoneTasks * 100) / res.TotalTasks
		}

		fmt.Printf("%s %s\n", statusIcon, res.Name)
		fmt.Printf("   Proposal: %v | Design: %v | Specs: %d domain files\n",
			res.HasProposal, res.HasDesign, res.SpecsCount)
		fmt.Printf("   Tasks: %d/%d completed (%d%%)\n", res.DoneTasks, res.TotalTasks, pct)
		if len(res.Errors) > 0 {
			fmt.Printf("   Errors: %s\n", strings.Join(res.Errors, "; "))
		}
		fmt.Println()
	}
	return nil
}

func archiveOpenSpec(changeName string) error {
	changeDir, isSingle := resolveChangeDir(changeName)
	if !isSingle {
		return fmt.Errorf("OpenSpec change %q not found under openspec/changes/", changeName)
	}

	res := validateSingleChange(changeDir)
	if res.TotalTasks > 0 && res.DoneTasks < res.TotalTasks {
		fmt.Printf("⚠️ Warning: change %q has %d pending task(s) (%d/%d done)\n",
			res.Name, res.TotalTasks-res.DoneTasks, res.DoneTasks, res.TotalTasks)
	}

	archiveBase := filepath.Join("openspec", "changes", "archive")
	if err := os.MkdirAll(archiveBase, 0755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02")
	destDir := filepath.Join(archiveBase, fmt.Sprintf("%s-%s", timestamp, res.Name))

	if _, err := os.Stat(destDir); err == nil {
		destDir = filepath.Join(archiveBase, fmt.Sprintf("%s-%s-%d", timestamp, res.Name, time.Now().Unix()))
	}

	if err := os.Rename(changeDir, destDir); err != nil {
		return fmt.Errorf("move change to archive: %w", err)
	}

	fmt.Printf("✓ Successfully archived change %q to %s\n", res.Name, destDir)
	return nil
}

func newOpenSpec(changeName string, domain string) error {
	changeName = strings.TrimSpace(changeName)
	if changeName == "" {
		return fmt.Errorf("change name cannot be empty")
	}

	targetDir := filepath.Join("openspec", "changes", changeName)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("change directory %q already exists", targetDir)
	}

	specsDir := filepath.Join(targetDir, "specs", domain)
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return fmt.Errorf("create change directory structure: %w", err)
	}

	proposalContent := fmt.Sprintf(`# Change Proposal: %s

## Why
<!-- Describe the motivation, user value, and problem statement -->

## What Changes
<!-- High-level summary of behavior changes and affected components -->

## Scope & Non-goals
<!-- Explicit boundaries: what is IN scope and what is OUT of scope -->

## Risks
<!-- Architectural, migration, or backward compatibility risks -->
`, changeName)

	designContent := fmt.Sprintf(`# Technical Design: %s

## Architecture & Seams
<!-- Architectural context, dependency direction, and module boundaries -->

## Decisions & Alternatives (Design It Twice)
<!-- Selected design vs evaluated and rejected alternatives -->

## Component Interfaces & Data Models
<!-- Contracts, protocols, and data structures -->
`, changeName)

	tasksContent := fmt.Sprintf(`# Tasks DAG: %s

<!-- Atomic implementation tasks (<= 350 LOC per task) with deterministic verification -->
- [ ] 1.1 Foundation and interface definition
- [ ] 1.2 Core behavior implementation
- [ ] 1.3 Independent verification and test regression
`, changeName)

	specContent := fmt.Sprintf(`# Specification: %s

## ADDED Requirements

### Requirement: REQ-%s-001 — Standard Behavior
The system MUST provide verifiable behavior for %s.

#### Scenario: Happy path
- GIVEN valid inputs and initial system state
- WHEN the requested operation executes
- THEN expected outcome occurs with exit code 0

#### Scenario: Edge case
- GIVEN boundary conditions or concurrent access
- WHEN the operation executes
- THEN system handles gracefully without state corruption

#### Scenario: Error state
- GIVEN invalid inputs or unavailable dependencies
- WHEN the operation is attempted
- THEN fail-closed rejection occurs with deterministic error code
`, domain, strings.ToUpper(domain), changeName)

	if err := os.WriteFile(filepath.Join(targetDir, "proposal.md"), []byte(proposalContent), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "design.md"), []byte(designContent), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "tasks.md"), []byte(tasksContent), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(specsDir, "spec.md"), []byte(specContent), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Created OpenSpec change %q under %s\n", changeName, targetDir)
	fmt.Printf("  Created proposal.md, design.md, tasks.md, and specs/%s/spec.md\n", domain)
	return nil
}

func parseTasksProgress(tasksPath string) (total int, done int) {
	f, err := os.Open(tasksPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "* [ ]") {
			total++
		} else if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") ||
			strings.HasPrefix(line, "* [x]") || strings.HasPrefix(line, "* [X]") {
			total++
			done++
		}
	}
	return total, done
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Size() > 0
}
