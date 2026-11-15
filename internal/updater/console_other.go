//go:build !windows

package updater

import "os/exec"

func hideConsoleWindow(*exec.Cmd) {}
