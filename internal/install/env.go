package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	EnvBackgroundSubagentsKey = "OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS"
	EnvBackgroundSubagentsVal = "true"
)

// WindowsEnvRunner runs an external command during Windows environment configuration.
type WindowsEnvRunner func(command string, args ...string) ([]byte, error)

// UnixEnvRunner configures the persistent environment on Unix systems or substitutes it during testing.
type UnixEnvRunner func(homeDir string) (bool, error)

var (
	defaultWindowsEnvRunner WindowsEnvRunner = func(command string, args ...string) ([]byte, error) {
		cmd := exec.Command(command, args...)
		return cmd.CombinedOutput()
	}
	currentWindowsEnvRunner = defaultWindowsEnvRunner
	envRunnerMu             sync.Mutex

	currentUnixEnvRunner UnixEnvRunner
	unixEnvRunnerMu      sync.Mutex
)

// SetWindowsEnvRunnerForTesting substitutes the Windows command runner during tests
// and returns a restoration function.
func SetWindowsEnvRunnerForTesting(runner WindowsEnvRunner) func() {
	envRunnerMu.Lock()
	prev := currentWindowsEnvRunner
	currentWindowsEnvRunner = runner
	envRunnerMu.Unlock()
	return func() {
		envRunnerMu.Lock()
		currentWindowsEnvRunner = prev
		envRunnerMu.Unlock()
	}
}

// SetUnixEnvRunnerForTesting substitutes the Unix environment configurator during tests
// and returns a restoration function.
func SetUnixEnvRunnerForTesting(runner UnixEnvRunner) func() {
	unixEnvRunnerMu.Lock()
	prev := currentUnixEnvRunner
	currentUnixEnvRunner = runner
	unixEnvRunnerMu.Unlock()
	return func() {
		unixEnvRunnerMu.Lock()
		currentUnixEnvRunner = prev
		unixEnvRunnerMu.Unlock()
	}
}

// ConfigureEnvironment ensures OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"
// is configured persistently across Windows, Linux, and macOS.
func ConfigureEnvironment(homeDir string) error {
	_, err := ConfigureEnvironmentWithResult(homeDir)
	return err
}

// ConfigureEnvironmentWithResult ensures OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"
// is configured persistently and reports whether persistent configuration changed.
func ConfigureEnvironmentWithResult(homeDir string) (bool, error) {
	_ = os.Setenv(EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)

	if runtime.GOOS == "windows" {
		return configureWindowsEnvWithResult()
	}
	return configureUnixEnv(homeDir)
}

func configureWindowsEnv() error {
	_, err := configureWindowsEnvWithResult()
	return err
}

func configureWindowsEnvWithResult() (bool, error) {
	envRunnerMu.Lock()
	runner := currentWindowsEnvRunner
	envRunnerMu.Unlock()

	if runner == nil {
		return false, errors.New("windows environment runner substitute is missing")
	}

	// Use PowerShell to set user-level persistent environment variable
	psCmd := fmt.Sprintf(`[Environment]::SetEnvironmentVariable("%s", "%s", "User")`,
		EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)
	out, err := runner("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if err != nil {
		return false, fmt.Errorf("set Windows user environment variable: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) == "unchanged" {
		return false, nil
	}
	return true, nil
}

func configureUnixEnv(homeDir string) (bool, error) {
	unixEnvRunnerMu.Lock()
	runner := currentUnixEnvRunner
	unixEnvRunnerMu.Unlock()

	if runner != nil {
		return runner(homeDir)
	}
	return configureUnixEnvDefault(homeDir)
}

func configureUnixEnvDefault(homeDir string) (bool, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return false, err
		}
	}

	targets := []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".zshrc"),
		filepath.Join(homeDir, ".profile"),
	}

	exportLine := fmt.Sprintf("export %s=\"%s\"", EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)
	marker := "# cortex-ia: OpenCode background subagents"

	changed := false
	foundAny := false
	for _, target := range targets {
		if _, err := os.Stat(target); err == nil {
			foundAny = true
			data, err := os.ReadFile(target)
			if err != nil {
				return false, fmt.Errorf("read %s: %w", target, err)
			}
			content := string(data)
			if !strings.Contains(content, EnvBackgroundSubagentsKey) {
				block := fmt.Sprintf("\n%s\n%s\n", marker, exportLine)
				if err := os.WriteFile(target, []byte(content+block), 0644); err != nil {
					return false, fmt.Errorf("write %s: %w", target, err)
				}
				changed = true
			}
		}
	}

	if !foundAny {
		target := filepath.Join(homeDir, ".profile")
		block := fmt.Sprintf("%s\n%s\n", marker, exportLine)
		if err := os.WriteFile(target, []byte(block), 0644); err != nil {
			return false, fmt.Errorf("write %s: %w", target, err)
		}
		return true, nil
	}

	return changed, nil
}
