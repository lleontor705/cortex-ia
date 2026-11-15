package delegation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func observeSystemBoot(ctx context.Context) (bootObservation, error) {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return bootObservation{}, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Use the documented local CIM property, not opaque NT structures or PATH tools.
	// No caller data, profile or remote computer is supplied to the fixed query.
	const query = `$ErrorActionPreference='Stop'; [Console]::Write((Get-CimInstance -ClassName Win32_OperatingSystem -Property LastBootUpTime).LastBootUpTime.ToUniversalTime().ToString('o',[Globalization.CultureInfo]::InvariantCulture))`
	cmd := exec.CommandContext(queryCtx, filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", query)
	cmd.Env = []string{"SystemRoot=" + filepath.Dir(system), "WINDIR=" + filepath.Dir(system), "PSModulePath=" + filepath.Join(system, "WindowsPowerShell", "v1.0", "Modules")}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.WaitDelay = time.Second
	var output bootOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return bootObservation{}, fmt.Errorf("windows boot observation unavailable: %w", err)
	}
	boot, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(output)))
	if err != nil {
		return bootObservation{}, errors.New("windows boot observation returned an invalid timestamp")
	}
	return bootObservation{Time: boot, Source: "Windows.Win32_OperatingSystem.LastBootUpTime"}, nil
}

type bootOutput []byte

func (b *bootOutput) Write(p []byte) (int, error) {
	if len(*b)+len(p) > 128 {
		return 0, errors.New("boot observation exceeds output bound")
	}
	*b = append(*b, p...)
	return len(p), nil
}
