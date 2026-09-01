package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvBackgroundSubagentsKey = "OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS"
	EnvBackgroundSubagentsVal = "true"
)

// ConfigureEnvironment ensures OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"
// is configured persistently across Windows, Linux, and macOS.
func ConfigureEnvironment(homeDir string) error {
	// 1. Process local environment
	_ = os.Setenv(EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)

	if runtime.GOOS == "windows" {
		return configureWindowsEnv()
	}
	return configureUnixEnv(homeDir)
}

func configureWindowsEnv() error {
	// Use PowerShell to set user-level persistent environment variable
	psCmd := fmt.Sprintf(`[Environment]::SetEnvironmentVariable("%s", "%s", "User")`,
		EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set Windows user environment variable: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func configureUnixEnv(homeDir string) error {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}

	targets := []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".zshrc"),
		filepath.Join(homeDir, ".profile"),
	}

	exportLine := fmt.Sprintf("export %s=\"%s\"", EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal)
	marker := "# cortex-ia: OpenCode background subagents"

	for _, target := range targets {
		if _, err := os.Stat(target); err == nil {
			data, err := os.ReadFile(target)
			if err != nil {
				continue
			}
			content := string(data)
			if !strings.Contains(content, EnvBackgroundSubagentsKey) {
				block := fmt.Sprintf("\n%s\n%s\n", marker, exportLine)
				_ = os.WriteFile(target, []byte(content+block), 0644)
			}
		}
	}
	return nil
}
