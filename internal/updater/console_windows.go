package updater

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// CREATE_NO_WINDOW denies the child a console entirely. HideWindow alone only
// hides an already-created console, which is not enough for a console-less
// parent such as the OpenCode service.
func hideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}
