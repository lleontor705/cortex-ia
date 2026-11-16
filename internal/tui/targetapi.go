package tui

import (
	"errors"
	"os/exec"

	"github.com/lleontor705/cortex-ia/v2/internal/targets"
)

// hostTargets is the production TargetAPI bound to one home directory. The
// OpenCode host is driven through ServiceAPI, so it is refused here.
type hostTargets struct {
	home string
}

var errOpenCodeHost = errors.New("OpenCode is installed through the install service, not the host adapter")

func hostBinary(id targets.TargetID) string {
	switch id {
	case targets.TargetClaude:
		return "claude"
	case targets.TargetAGY:
		return "agy"
	}
	return "opencode"
}

func (h hostTargets) Detected(id targets.TargetID) bool {
	_, err := exec.LookPath(hostBinary(id))
	return err == nil
}

func (h hostTargets) Installed(id targets.TargetID) bool {
	return targets.Installed(h.home, id)
}

func (h hostTargets) Install(id targets.TargetID, opts targets.ApplyOptions) (*targets.TargetResult, error) {
	switch id {
	case targets.TargetClaude:
		return targets.InstallClaudeWith(h.home, opts)
	case targets.TargetAGY:
		return targets.InstallAGYWith(h.home, opts)
	}
	return nil, errOpenCodeHost
}

func (h hostTargets) Uninstall(id targets.TargetID) (*targets.TargetResult, error) {
	switch id {
	case targets.TargetClaude:
		return targets.UninstallClaude(h.home, false)
	case targets.TargetAGY:
		return targets.UninstallAGY(h.home, false)
	}
	return nil, errOpenCodeHost
}
