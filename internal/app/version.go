package app

import (
	"runtime/debug"
	"strings"
)

// Version holds the current build version, set via ldflags or defaulting to "dev".
var Version string

// ResolveVersion returns the ldflags version if set, otherwise tries the Go
// module version embedded by "go install", and falls back to "dev".
func ResolveVersion(ldflags string) string {
	v := strings.TrimSpace(ldflags)
	if v != "" && v != "dev" {
		return v
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
