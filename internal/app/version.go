package app

import (
	"os/exec"
	"runtime/debug"
	"strings"
)

// Version holds the current build version, set via ldflags or defaulting to "dev".
var (
	Version = "dev"
	Commit  string
	Date    string
)

// ResolveVersion returns the ldflags version if set, otherwise tries the Go
// module version embedded by "go install", and falls back to "dev".
func ResolveVersion(ldflags string) string {
	v := strings.TrimSpace(ldflags)
	if v != "" && v != "dev" {
		return v
	}

	if Version != "" && Version != "dev" {
		return Version
	}

	// When installed via "go install ...@v0.0.15", Go embeds the module
	// version in the binary's build info — use it as a fallback.
	if bi, ok := debug.ReadBuildInfo(); ok {
		mv := strings.TrimSpace(bi.Main.Version)
		if mv != "" && mv != "(devel)" {
			return mv
		}
	}

	return "dev"
}

// GitDescribeVersion attempts to read the current git tag if running within a git repository.
func GitDescribeVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	out, err := cmd.Output()
	if err == nil {
		tag := strings.TrimSpace(string(out))
		if tag != "" {
			return tag
		}
	}
	return ""
}
