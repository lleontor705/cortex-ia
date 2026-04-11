package system

import (
	"runtime"
	"testing"
)

func TestDetect_ReturnsNonEmpty(t *testing.T) {
	info := Detect()

	if info.OS == "" {
		t.Error("OS should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
}

func TestDetect_ValidOS(t *testing.T) {
	info := Detect()

	validOS := map[string]bool{
		"linux":   true,
		"darwin":  true,
		"windows": true,
		"freebsd": true,
		"netbsd":  true,
		"openbsd": true,
	}

	if !validOS[info.OS] {
		t.Errorf("OS %q is not a recognized value", info.OS)
	}

	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want runtime.GOOS = %q", info.OS, runtime.GOOS)
	}
}

func TestDetect_ValidArch(t *testing.T) {
	info := Detect()

	validArch := map[string]bool{
		"amd64":   true,
		"arm64":   true,
		"arm":     true,
		"386":     true,
		"ppc64":   true,
		"ppc64le": true,
		"s390x":   true,
		"riscv64": true,
		"mips":    true,
		"mipsle":  true,
		"mips64":  true,
		"mips64le": true,
	}

	if !validArch[info.Arch] {
		t.Errorf("Arch %q is not a recognized value", info.Arch)
	}

	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want runtime.GOARCH = %q", info.Arch, runtime.GOARCH)
	}
}

func TestDetect_ToolsStructure(t *testing.T) {
	info := Detect()

	// The Tools struct should exist and Shell should be populated on most systems.
	// We don't require specific tool versions since they depend on the environment.
	tools := info.Tools

	// Shell detection should return something (even "unknown" on minimal systems).
	if tools.Shell == "" {
		t.Error("Tools.Shell should not be empty; expected at least 'unknown'")
	}

	// Version strings, if present, should be non-empty (no blank strings when detected).
	if tools.NodeVersion != "" && len(tools.NodeVersion) < 2 {
		t.Errorf("Tools.NodeVersion looks invalid: %q", tools.NodeVersion)
	}
	if tools.GitVersion != "" && len(tools.GitVersion) < 3 {
		t.Errorf("Tools.GitVersion looks invalid: %q", tools.GitVersion)
	}
	if tools.GoVersion != "" && len(tools.GoVersion) < 3 {
		t.Errorf("Tools.GoVersion looks invalid: %q", tools.GoVersion)
	}
}

func TestDetect_ProfileStructure(t *testing.T) {
	info := Detect()

	profile := info.Profile

	if profile.OS == "" {
		t.Error("Profile.OS should not be empty")
	}

	if profile.OS != runtime.GOOS {
		t.Errorf("Profile.OS = %q, want runtime.GOOS = %q", profile.OS, runtime.GOOS)
	}

	// PackageManager should be set on supported platforms.
	if profile.Supported && profile.PackageManager == "" {
		t.Error("Profile.PackageManager should not be empty on a supported platform")
	}

	// On the current test host, the platform should be supported
	// (linux, darwin, or windows are all supported).
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if !profile.Supported {
			t.Errorf("expected platform %q to be supported", runtime.GOOS)
		}
	}
}
