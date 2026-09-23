package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

// updateSchedulerRunner is the scheduler seam: production uses the OS-backed
// runner, while tests inject a recording stub so the real Task Scheduler and
// the user crontab are never touched.
var updateSchedulerRunner = updater.OSCommandRunner

func printUpdateScheduleHelp() {
	fmt.Println("Usage: cortex-ia update schedule <enable|disable|status>")
	fmt.Println("\nManage the daily, check-only update task registered with the OS scheduler.")
	fmt.Printf("The registered payload is always '%s': it refreshes the cached\n", updater.ScheduledCheckArgs)
	fmt.Println("update state and never downloads or applies a release.")
}

func runUpdateSchedule(args []string) error {
	if len(args) == 0 {
		return errors.New("update schedule requires a subcommand: enable, disable, or status")
	}
	switch strings.ToLower(args[0]) {
	case "enable":
		return rejectScheduleArgs(args[1:], runUpdateScheduleEnable)
	case "disable":
		return rejectScheduleArgs(args[1:], runUpdateScheduleDisable)
	case "status":
		return rejectScheduleArgs(args[1:], runUpdateScheduleStatus)
	case "--help", "-h", "help":
		printUpdateScheduleHelp()
		return nil
	default:
		return fmt.Errorf("unknown update schedule subcommand: %s (use 'cortex-ia update schedule --help' for usage)", args[0])
	}
}

func rejectScheduleArgs(args []string, run func() error) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument for 'update schedule': %s", args[0])
	}
	return run()
}

func runUpdateScheduleEnable() error {
	execPath := managedExecutablePath()
	if err := updater.EnableScheduledCheck(execPath, updateSchedulerRunner()); err != nil {
		return err
	}

	fmt.Println("Scheduled update check enabled.")
	fmt.Printf("  task:       %s\n", updater.ScheduledTaskName)
	fmt.Printf("  frequency:  %s\n", updater.ScheduledCheckFrequency)
	fmt.Printf("  executable: %s\n", execPath)
	fmt.Printf("  command:    %s\n", updater.ScheduledCheckArgs)
	printDualInstallWarning(updater.DetectInstallCandidates(execPath))
	return nil
}

func runUpdateScheduleDisable() error {
	if err := updater.DisableScheduledCheck(updateSchedulerRunner()); err != nil {
		return err
	}
	fmt.Println("Scheduled update check disabled: no managed task remains.")
	return nil
}

func runUpdateScheduleStatus() error {
	status, err := updater.ScheduledCheckStatus(updateSchedulerRunner())
	if err != nil {
		return err
	}

	registryState := "disabled"
	if status.Registered {
		registryState = "enabled"
	}
	fmt.Printf("Scheduled update check: %s\n", registryState)
	fmt.Printf("  task:       %s\n", updater.ScheduledTaskName)
	fmt.Printf("  frequency:  %s\n", status.Frequency)
	if status.Executable != "" {
		fmt.Printf("  executable: %s\n", status.Executable)
	}
	if status.Detail != "" {
		fmt.Printf("  detail:     %s\n", status.Detail)
	}
	fmt.Printf("  command:    %s\n", updater.ScheduledCheckArgs)
	printDualInstallWarning(updater.DetectInstallCandidates(managedExecutablePath()))
	return nil
}
