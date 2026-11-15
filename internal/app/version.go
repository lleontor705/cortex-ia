package app

import (
	"runtime/debug"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/updater"
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

// GitDescribeVersion returns the git-describe identifier of the checked-out
// cortex-ia source tree, or "" when the working directory is not this project's
// repository. Delegating to the updater keeps the module-path guard in one place
// so an unrelated repository's tags never leak into the reported version.
func GitDescribeVersion() string {
	return updater.GitDescribeVersion()
}
