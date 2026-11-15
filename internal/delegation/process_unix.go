//go:build !windows

package delegation

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Process groups contain ordinary descendants, not processes deliberately escaping
// the group. This is lifecycle containment, not an operating-system sandbox.
func startProcessTree(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var mu sync.Mutex
	stopped := false
	stop := func() error {
		mu.Lock()
		defer mu.Unlock()
		if cmd.Process == nil || stopped {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			stopped = true
			return nil
		}
		if err != nil {
			return err
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			err = syscall.Kill(-cmd.Process.Pid, 0)
			if errors.Is(err, syscall.ESRCH) {
				stopped = true
				return nil
			}
			if err != nil {
				return err
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("external process group exit unconfirmed (including unreaped descendants)")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	cmd.Cancel = stop
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stop, nil
}
