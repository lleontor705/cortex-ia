package app

import "strings"

// Version holds the current build version, set via ldflags or defaulting to "dev".
var Version string

// ResolveVersion returns the ldflags version if set, otherwise "dev".
func ResolveVersion(ldflags string) string {
	v := strings.TrimSpace(ldflags)
	if v == "" || v == "dev" {
		return "dev"
	}
	return v
}
