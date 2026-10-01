//go:build !windows

package app

import "os/exec"

func hideConsoleWindow(*exec.Cmd) {}
