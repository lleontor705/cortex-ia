package tui

import (
	"github.com/lleontor705/cortex-ia/internal/targets"
)

// fakeHosts is the zero-value-safe TargetAPI test double: no host is detected
// or installed unless a test says so, and every call is recorded.
type fakeHosts struct {
	detected       map[targets.TargetID]bool
	installed      map[targets.TargetID]bool
	installCalls   []targets.TargetID
	uninstallCalls []targets.TargetID
	installFn      func(targets.TargetID, targets.ApplyOptions) (*targets.TargetResult, error)
	uninstallFn    func(targets.TargetID) (*targets.TargetResult, error)
}

func (f *fakeHosts) Detected(id targets.TargetID) bool {
	return f.detected[id]
}

func (f *fakeHosts) Installed(id targets.TargetID) bool {
	return f.installed[id]
}

func (f *fakeHosts) Install(id targets.TargetID, opts targets.ApplyOptions) (*targets.TargetResult, error) {
	f.installCalls = append(f.installCalls, id)
	if f.installFn != nil {
		return f.installFn(id, opts)
	}
	return &targets.TargetResult{Target: id, Success: true, Message: "Installed " + targets.TargetLabel(id)}, nil
}

func (f *fakeHosts) Uninstall(id targets.TargetID) (*targets.TargetResult, error) {
	f.uninstallCalls = append(f.uninstallCalls, id)
	if f.uninstallFn != nil {
		return f.uninstallFn(id)
	}
	return &targets.TargetResult{Target: id, Success: true, Message: "Removed " + targets.TargetLabel(id)}, nil
}
