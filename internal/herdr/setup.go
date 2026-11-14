package herdr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveHerdr includes the canonical Windows installer location because the
// installer does not always update the parent process PATH immediately.
func ResolveHerdr() (string, error) {
	if found, err := exec.LookPath("herdr"); err == nil {
		return found, nil
	}
	home, _ := os.UserHomeDir()
	local := os.Getenv("LOCALAPPDATA")
	if local == "" && home != "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	candidates := []string{
		filepath.Join(local, "Programs", "Herdr", "bin", "herdr.exe"),
		filepath.Join(home, ".cargo", "bin", "herdr.exe"),
		filepath.Join(home, ".cargo", "bin", "herdr"),
		filepath.Join(home, ".local", "bin", "herdr"),
		"/opt/homebrew/bin/herdr",
		"/usr/local/bin/herdr", "/usr/bin/herdr",
	}
	return firstRegular(candidates, "herdr")
}

func firstRegular(candidates []string, name string) (string, error) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s executable not found", name)
}

func HerdrRunning(herdrPath string) bool {
	if os.Getenv("HERDR_ENV") == "1" || os.Getenv("HERDR_WORKSPACE_ID") != "" {
		return true
	}
	if herdrPath == "" {
		return false
	}
	cmd := exec.Command(herdrPath, "pane", "list")
	out, err := cmd.Output()
	if err == nil && (strings.Contains(string(out), "pane_list") || strings.Contains(string(out), "panes")) {
		return true
	}
	return false
}
