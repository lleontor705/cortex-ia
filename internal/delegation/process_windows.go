package delegation

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Assign the suspended child before it can spawn descendants. Closing a job with
// KILL_ON_JOB_CLOSE also contains descendants after an unexpected controller exit.
func startProcessTree(cmd *exec.Cmd) (func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	var mu sync.Mutex
	stop := func() error {
		mu.Lock()
		defer mu.Unlock()
		if job == 0 {
			return nil
		}
		if err := windows.TerminateJobObject(job, 1); err != nil {
			return err
		}
		// ActiveProcesses must be zero before temporary state or leases are released.
		var accounting struct {
			Times                             [4]int64
			Faults, Total, Active, Terminated uint32
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
				return err
			}
			if accounting.Active == 0 {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("external process tree termination unconfirmed")
			}
			time.Sleep(10 * time.Millisecond)
		}
		err := windows.CloseHandle(job)
		job = 0
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	cmd.Cancel = stop
	mu.Lock()
	err = cmd.Start()
	if err == nil {
		var process windows.Handle
		process, err = windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err == nil {
			err = windows.AssignProcessToJobObject(job, process)
			_ = windows.CloseHandle(process)
		}
		if err == nil {
			err = resumeProcess(uint32(cmd.Process.Pid))
		}
	}
	mu.Unlock()
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = stop()
		return nil, err
	}
	return stop, nil
}

func resumeProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		return errors.Join(err, windows.CloseHandle(thread))
	}
	return fmt.Errorf("cannot find suspended external process thread: %w", err)
}
