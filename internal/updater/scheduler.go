package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Scheduled update check registration (REQ-AU-005).
//
// The registered payload is strictly CHECK-ONLY: it may refresh the cached
// update state but never downloads or replaces a binary, so the scheduler can
// never open an unattended apply path.

const (
	// ScheduledTaskName is the stable identity of the managed scheduled task.
	ScheduledTaskName = "CortexIA Update Check"

	// ScheduledCheckArgs is the only payload the scheduled task is allowed to run.
	ScheduledCheckArgs = "update --check --scheduled"

	// ScheduledCheckFrequency is the cadence registered on every platform.
	ScheduledCheckFrequency = "daily at 12:00"

	scheduledTimeOfDay  = "12:00"
	scheduledCronFields = "0 12 * * *"
	scheduledCronMarker = "# cortex-ia-managed scheduled update check"

	schtasksCommand = "schtasks"
	crontabCommand  = "crontab"

	platformWindows     = "windows"
	platformPOSIX       = "posix"
	platformUnsupported = "unsupported"
)

// Typed scheduler errors.
var (
	// ErrSchedulerUnavailable means the platform scheduler command is absent.
	ErrSchedulerUnavailable = errors.New("scheduled update check unavailable: os scheduler command not found")
	// ErrForeignScheduledTask means the managed name is owned by a non-cortex-ia task.
	ErrForeignScheduledTask = errors.New("scheduled task name is already used by a task not managed by cortex-ia")
	// ErrUnsupportedSchedulerPlatform means the host OS has no supported registration path.
	ErrUnsupportedSchedulerPlatform = errors.New("scheduled update checks are not supported on this platform")
	// ErrMissingScheduledExecutable means no executable path was supplied.
	ErrMissingScheduledExecutable = errors.New("scheduled update check requires the cortex-ia executable path")
)

// schedulerGOOS is the platform seam: tests override it to exercise both the
// Windows and the POSIX paths from a single host.
var schedulerGOOS = runtime.GOOS

// CommandRunner is the injectable execution seam. Tests supply a recording stub
// so the real Task Scheduler and crontab are never touched.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// ScheduledStatus is the observed state of the scheduled update check.
type ScheduledStatus struct {
	Registered bool
	Executable string
	Frequency  string
	Detail     string
}

// OSCommandRunner returns the production runner backed by exec.Command.
func OSCommandRunner() CommandRunner { return osCommandRunner{} }

type osCommandRunner struct{}

func (osCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// EnableScheduledCheck registers the check-only update task for the current user.
// It never elevates privileges and never overwrites a task it does not recognize
// as its own.
func EnableScheduledCheck(execPath string, r CommandRunner) error {
	path := strings.TrimSpace(execPath)
	if path == "" {
		return ErrMissingScheduledExecutable
	}
	r = runnerOrDefault(r)

	switch schedulerPlatform() {
	case platformWindows:
		return enableWindowsScheduledCheck(path, r)
	case platformPOSIX:
		return enablePOSIXScheduledCheck(path, r)
	default:
		return ErrUnsupportedSchedulerPlatform
	}
}

// DisableScheduledCheck removes the managed task, reporting success when the
// task was already absent.
func DisableScheduledCheck(r CommandRunner) error {
	r = runnerOrDefault(r)

	switch schedulerPlatform() {
	case platformWindows:
		_, err := deleteWindowsScheduledCheck(r)
		return err
	case platformPOSIX:
		_, err := disablePOSIXScheduledCheck(r)
		return err
	default:
		return ErrUnsupportedSchedulerPlatform
	}
}

// ScheduledCheckStatus reports whether the managed task is registered, which
// executable it points at, and its cadence.
func ScheduledCheckStatus(r CommandRunner) (ScheduledStatus, error) {
	r = runnerOrDefault(r)

	switch schedulerPlatform() {
	case platformWindows:
		return windowsScheduledCheckStatus(r)
	case platformPOSIX:
		return posixScheduledCheckStatus(r)
	default:
		return ScheduledStatus{}, ErrUnsupportedSchedulerPlatform
	}
}

func runnerOrDefault(r CommandRunner) CommandRunner {
	if r == nil {
		return OSCommandRunner()
	}
	return r
}

func schedulerPlatform() string {
	switch schedulerGOOS {
	case platformWindows:
		return platformWindows
	case "linux", "darwin", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix":
		return platformPOSIX
	default:
		return platformUnsupported
	}
}

// --- Windows (schtasks, user-level) ---

func windowsTaskAction(execPath string) string {
	return `"` + execPath + `" ` + ScheduledCheckArgs
}

// queryWindowsTask returns the verbose task definition. When the name is not
// registered (or the query fails for a non-fatal reason) it reports absent, so
// the caller can still perform first-time provisioning.
func queryWindowsTask(r CommandRunner) (string, bool, error) {
	out, err := r.Run(schtasksCommand, "/Query", "/TN", ScheduledTaskName, "/FO", "CSV", "/V")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", false, fmt.Errorf("%w: %s", ErrSchedulerUnavailable, schtasksCommand)
		}
		return string(out), false, nil
	}
	return string(out), true, nil
}

func enableWindowsScheduledCheck(execPath string, r CommandRunner) error {
	out, exists, err := queryWindowsTask(r)
	if err != nil {
		return err
	}
	if exists && !strings.Contains(out, ScheduledCheckArgs) {
		return fmt.Errorf("%w: %s", ErrForeignScheduledTask, ScheduledTaskName)
	}

	// /F converges an existing managed task; no /RL or /RU keeps it user-level.
	_, err = r.Run(schtasksCommand,
		"/Create",
		"/TN", ScheduledTaskName,
		"/TR", windowsTaskAction(execPath),
		"/SC", "DAILY",
		"/ST", scheduledTimeOfDay,
		"/F",
	)
	if err != nil {
		return wrapSchedulerError("schtasks /Create", err)
	}
	return nil
}

func deleteWindowsScheduledCheck(r CommandRunner) (bool, error) {
	out, exists, err := queryWindowsTask(r)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if !strings.Contains(out, ScheduledCheckArgs) {
		return false, fmt.Errorf("%w: %s", ErrForeignScheduledTask, ScheduledTaskName)
	}
	if _, err := r.Run(schtasksCommand, "/Delete", "/TN", ScheduledTaskName, "/F"); err != nil {
		return false, wrapSchedulerError("schtasks /Delete", err)
	}
	return true, nil
}

func windowsScheduledCheckStatus(r CommandRunner) (ScheduledStatus, error) {
	status := ScheduledStatus{Frequency: ScheduledCheckFrequency, Detail: "task not registered"}

	out, exists, err := queryWindowsTask(r)
	if err != nil {
		return ScheduledStatus{}, err
	}
	if !exists {
		return status, nil
	}
	if !strings.Contains(out, ScheduledCheckArgs) {
		status.Detail = "a task with the managed name exists but is not managed by cortex-ia"
		return status, nil
	}

	status.Registered = true
	status.Executable = parseActionExecutable(out)
	status.Detail = "registered with the Windows Task Scheduler"
	return status, nil
}

// parseActionExecutable extracts the executable from the task action recorded by
// schtasks. The action is always emitted as `"<executable>" <args>`, optionally
// re-escaped inside a CSV field.
func parseActionExecutable(output string) string {
	normalized := strings.ReplaceAll(output, `\"`, `"`)
	normalized = strings.ReplaceAll(normalized, `""`, `"`)

	idx := strings.Index(normalized, ScheduledCheckArgs)
	if idx < 0 {
		return ""
	}

	prefix := strings.TrimRight(normalized[:idx], " \t")
	prefix = strings.TrimSuffix(prefix, `"`)
	if open := strings.LastIndex(prefix, `"`); open >= 0 {
		return prefix[open+1:]
	}
	if fields := strings.Fields(prefix); len(fields) > 0 {
		return fields[len(fields)-1]
	}
	return ""
}

// --- POSIX (user crontab) ---

func posixCronLine(execPath string) string {
	return scheduledCronFields + " " + posixQuote(execPath) + " " + ScheduledCheckArgs + " " + scheduledCronMarker
}

func posixQuote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(value)
	return `"` + escaped + `"`
}

func posixUnquote(value string) string {
	trimmed := value
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	var b strings.Builder
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '\\' && i+1 < len(trimmed) {
			i++
		}
		b.WriteByte(trimmed[i])
	}
	return b.String()
}

// readCrontabLines returns the current crontab lines. present is false when the
// user has no crontab yet, which is not an error.
func readCrontabLines(r CommandRunner) (lines []string, present bool, err error) {
	out, err := r.Run(crontabCommand, "-l")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, false, fmt.Errorf("%w: %s", ErrSchedulerUnavailable, crontabCommand)
		}
		if strings.Contains(strings.ToLower(string(out)), "no crontab") {
			return nil, false, nil
		}
		// Fail closed: an unreadable crontab must never be replaced by a
		// cortex-ia-only crontab that drops the user's own entries.
		return nil, false, fmt.Errorf("cortex-ia: cannot read the current crontab: %w", err)
	}

	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	return strings.Split(text, "\n"), true, nil
}

func isManagedCronLine(line string) bool {
	return strings.Contains(line, scheduledCronMarker) || strings.Contains(line, ScheduledCheckArgs)
}

func filterManagedCronLines(lines []string) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isManagedCronLine(line) || strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

func writeCrontab(r CommandRunner, lines []string) error {
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}

	file, err := os.CreateTemp("", "cortex-ia-crontab-*")
	if err != nil {
		return fmt.Errorf("cortex-ia: cannot stage the crontab update: %w", err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("cortex-ia: cannot stage the crontab update: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cortex-ia: cannot stage the crontab update: %w", err)
	}

	if _, err := r.Run(crontabCommand, path); err != nil {
		return wrapSchedulerError("crontab", err)
	}
	return nil
}

func enablePOSIXScheduledCheck(execPath string, r CommandRunner) error {
	lines, present, err := readCrontabLines(r)
	if err != nil {
		return err
	}
	if !present {
		lines = nil
	}

	kept := filterManagedCronLines(lines)
	kept = append(kept, posixCronLine(execPath))
	return writeCrontab(r, kept)
}

func disablePOSIXScheduledCheck(r CommandRunner) (bool, error) {
	lines, present, err := readCrontabLines(r)
	if err != nil {
		return false, err
	}
	if !present {
		return false, nil
	}

	managed := false
	for _, line := range lines {
		if isManagedCronLine(line) {
			managed = true
			break
		}
	}
	if !managed {
		return false, nil
	}

	kept := filterManagedCronLines(lines)
	if err := writeCrontab(r, kept); err != nil {
		return false, err
	}
	return true, nil
}

func posixScheduledCheckStatus(r CommandRunner) (ScheduledStatus, error) {
	status := ScheduledStatus{Frequency: ScheduledCheckFrequency, Detail: "crontab entry not registered"}

	lines, present, err := readCrontabLines(r)
	if err != nil {
		return ScheduledStatus{}, err
	}
	if !present {
		return status, nil
	}

	for _, line := range lines {
		if !strings.Contains(line, ScheduledCheckArgs) {
			continue
		}
		status.Registered = true
		status.Executable = parseCronExecutable(line)
		status.Detail = "registered in the user crontab"
		return status, nil
	}
	return status, nil
}

// parseCronExecutable drops the five schedule fields and unquotes the command.
func parseCronExecutable(line string) string {
	idx := strings.Index(line, ScheduledCheckArgs)
	if idx < 0 {
		return ""
	}

	fields := strings.Fields(strings.TrimSpace(line[:idx]))
	if len(fields) < 6 {
		return posixUnquote(strings.TrimSpace(line[:idx]))
	}
	return posixUnquote(strings.Join(fields[5:], " "))
}

func wrapSchedulerError(operation string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrSchedulerUnavailable, operation)
	}
	return fmt.Errorf("cortex-ia: %s failed: %w", operation, err)
}
