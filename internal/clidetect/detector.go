package clidetect

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CLIID identifies a supported coding CLI.
type CLIID string

const (
	CLIOpenCode CLIID = "opencode"
	CLIAGY      CLIID = "agy"
	CLIClaude   CLIID = "claude"
)

// CLIInfo captures detection status and environment for one CLI.
type CLIInfo struct {
	ID          CLIID  `json:"id"`
	DisplayName string `json:"display_name"`
	BinaryPath  string `json:"binary_path,omitempty"`
	Version     string `json:"version,omitempty"`
	Found       bool   `json:"found"`
	ConfigDir   string `json:"config_dir,omitempty"`
	ConfigFound bool   `json:"config_found"`
	Error       string `json:"error,omitempty"`
}

// DetectAll returns the detection result for all supported CLIs.
func DetectAll(homeDir string) []CLIInfo {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return []CLIInfo{
		DetectOpenCode(homeDir),
		DetectAGY(homeDir),
		DetectClaude(homeDir),
	}
}

// DetectOpenCode detects the OpenCode CLI installation and configuration.
func DetectOpenCode(homeDir string) CLIInfo {
	info := CLIInfo{
		ID:          CLIOpenCode,
		DisplayName: "OpenCode",
		ConfigDir:   filepath.Join(homeDir, ".config", "opencode"),
	}
	if st, err := os.Stat(info.ConfigDir); err == nil && st.IsDir() {
		info.ConfigFound = true
	}

	candidates := []string{}
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		appData := os.Getenv("APPDATA")
		candidates = append(candidates,
			filepath.Join(local, "Volta", "bin", "opencode.cmd"),
			filepath.Join(appData, "npm", "opencode.cmd"),
			filepath.Join(homeDir, "AppData", "Local", "Volta", "bin", "opencode.cmd"),
			filepath.Join(homeDir, "AppData", "Roaming", "npm", "opencode.cmd"),
		)
	} else {
		candidates = append(candidates,
			filepath.Join(homeDir, ".local", "bin", "opencode"),
			"/usr/local/bin/opencode",
			"/usr/bin/opencode",
		)
	}

	bin, err := resolveBinary("opencode", candidates)
	if err == nil {
		info.BinaryPath = bin
		info.Found = true
		info.Version = queryVersion(bin, "--version")
	} else {
		info.Error = err.Error()
	}

	return info
}

// DetectAGY detects the Google Antigravity (AGY) CLI installation and configuration.
func DetectAGY(homeDir string) CLIInfo {
	info := CLIInfo{
		ID:          CLIAGY,
		DisplayName: "Antigravity CLI (AGY)",
		ConfigDir:   filepath.Join(homeDir, ".gemini"),
	}
	if st, err := os.Stat(info.ConfigDir); err == nil && st.IsDir() {
		info.ConfigFound = true
	}

	candidates := []string{}
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		candidates = append(candidates,
			filepath.Join(local, "agy", "bin", "agy.exe"),
			filepath.Join(homeDir, "AppData", "Local", "agy", "bin", "agy.exe"),
			filepath.Join(homeDir, ".agy", "bin", "agy.exe"),
		)
	} else {
		candidates = append(candidates,
			filepath.Join(homeDir, ".agy", "bin", "agy"),
			"/usr/local/bin/agy",
			"/usr/bin/agy",
		)
	}

	bin, err := resolveBinary("agy", candidates)
	if err == nil {
		info.BinaryPath = bin
		info.Found = true
		info.Version = queryVersion(bin, "--version")
	} else {
		info.Error = err.Error()
	}

	return info
}

// DetectClaude detects the Anthropic Claude Code CLI installation and configuration.
func DetectClaude(homeDir string) CLIInfo {
	info := CLIInfo{
		ID:          CLIClaude,
		DisplayName: "Claude Code",
		ConfigDir:   filepath.Join(homeDir, ".claude"),
	}
	if st, err := os.Stat(info.ConfigDir); err == nil && st.IsDir() {
		info.ConfigFound = true
	} else {
		claudeJSON := filepath.Join(homeDir, ".claude.json")
		if st, err := os.Stat(claudeJSON); err == nil && !st.IsDir() {
			info.ConfigFound = true
			info.ConfigDir = claudeJSON
		}
	}

	candidates := []string{}
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		candidates = append(candidates,
			filepath.Join(appData, "npm", "claude.cmd"),
			filepath.Join(homeDir, "AppData", "Roaming", "npm", "claude.cmd"),
		)
	} else {
		candidates = append(candidates,
			filepath.Join(homeDir, ".local", "bin", "claude"),
			"/usr/local/bin/claude",
			"/usr/bin/claude",
		)
	}

	bin, err := resolveBinary("claude", candidates)
	if err == nil {
		info.BinaryPath = bin
		info.Found = true
		info.Version = queryVersion(bin, "--version")
	} else {
		info.Error = err.Error()
	}

	return info
}

func resolveBinary(name string, candidates []string) (string, error) {
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}
	return "", os.ErrNotExist
}

func queryVersion(bin string, flag string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, flag)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
