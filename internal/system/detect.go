package system

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemInfo holds detected platform information.
type SystemInfo struct {
	OS      string
	Arch    string
	Profile PlatformProfile
	Tools   ToolsInfo
}

// ToolsInfo holds detected tool versions and availability.
type ToolsInfo struct {
	NodeVersion   string // e.g. "v20.11.0" or "" if not found
	NpxAvailable  bool
	GitVersion    string // e.g. "2.43.0" or ""
	GoVersion     string // e.g. "go1.26.1" or ""
	CortexFound   bool
	Shell         string // "bash", "zsh", "powershell", etc.
}

// PlatformProfile describes the platform for install command resolution.
type PlatformProfile struct {
	OS             string
	LinuxDistro    string
	PackageManager string
	Supported      bool
}

const (
	LinuxDistroUnknown = "unknown"
	LinuxDistroUbuntu  = "ubuntu"
	LinuxDistroDebian  = "debian"
	LinuxDistroArch    = "arch"
	LinuxDistroFedora  = "fedora"
)

// Detect returns system information for the current platform.
func Detect() SystemInfo {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	profile := resolvePlatformProfile(goos)
	tools := detectTools()

	return SystemInfo{
		OS:      goos,
		Arch:    goarch,
		Profile: profile,
		Tools:   tools,
	}
}

// detectTools checks availability and versions of runtime dependencies.
func detectTools() ToolsInfo {
	var t ToolsInfo

	if out, err := exec.Command("node", "--version").Output(); err == nil {
		t.NodeVersion = strings.TrimSpace(string(out))
	}
	_, t.NpxAvailable = ToolExists("npx")
	_, t.CortexFound = ToolExists("cortex")

	if out, err := exec.Command("git", "--version").Output(); err == nil {
		// "git version 2.43.0" → "2.43.0"
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) >= 3 {
			t.GitVersion = parts[2]
		}
	}
	if out, err := exec.Command("go", "version").Output(); err == nil {
		// "go version go1.26.1 ..." → "go1.26.1"
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) >= 3 {
			t.GoVersion = parts[2]
		}
	}

	t.Shell = detectShell()
	return t
}

func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		parts := strings.Split(shell, "/")
		return parts[len(parts)-1]
	}
	if _, ok := ToolExists("powershell"); ok {
		return "powershell"
	}
	if _, ok := ToolExists("bash"); ok {
		return "bash"
	}
	return "unknown"
}

// ToolExists checks if a command-line tool is available on PATH.
func ToolExists(name string) (string, bool) {
	path, err := exec.LookPath(name)
	return path, err == nil
}

func resolvePlatformProfile(goos string) PlatformProfile {
	profile := PlatformProfile{OS: goos}

	switch goos {
	case "darwin":
		profile.PackageManager = "brew"
		profile.Supported = true
	case "linux":
		distro := detectLinuxDistro()
		profile.LinuxDistro = distro
		if _, hasBrew := ToolExists("brew"); hasBrew {
			profile.PackageManager = "brew"
			profile.Supported = true
		} else {
			switch distro {
			case LinuxDistroUbuntu, LinuxDistroDebian:
				profile.PackageManager = "apt"
				profile.Supported = true
			case LinuxDistroArch:
				profile.PackageManager = "pacman"
				profile.Supported = true
			case LinuxDistroFedora:
				profile.PackageManager = "dnf"
				profile.Supported = true
			default:
				profile.Supported = false
			}
		}
	case "windows":
		profile.PackageManager = "winget"
		profile.Supported = true
	default:
		profile.Supported = false
	}

	return profile
}

func detectLinuxDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return LinuxDistroUnknown
	}

	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		fields[key] = strings.ToLower(value)
	}

	id := fields["ID"]
	idLike := fields["ID_LIKE"]

	if containsAny(id, idLike, LinuxDistroUbuntu, LinuxDistroDebian) {
		if id == LinuxDistroDebian {
			return LinuxDistroDebian
		}
		return LinuxDistroUbuntu
	}
	if containsAny(id, idLike, LinuxDistroArch) {
		return LinuxDistroArch
	}
	if containsAny(id, idLike, LinuxDistroFedora, "rhel", "centos", "rocky") {
		return LinuxDistroFedora
	}
	return LinuxDistroUnknown
}

func containsAny(id, idLike string, targets ...string) bool {
	for _, t := range targets {
		if id == t {
			return true
		}
		for _, token := range strings.Fields(idLike) {
			if token == t {
				return true
			}
		}
	}
	return false
}
