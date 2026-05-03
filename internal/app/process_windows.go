//go:build windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var processJobs sync.Map

func configureCommandForProcessTree(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
		return
	}

	attr := *cmd.SysProcAttr
	attr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr = &attr
}

func startProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return err
	}

	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return err
	}
	defer windows.CloseHandle(process)

	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return err
	}

	processJobs.Store(cmd, job)
	return nil
}

func cleanupProcessTree(cmd *exec.Cmd) {
	if job, ok := loadCommandJob(cmd); ok {
		_ = windows.CloseHandle(job)
		processJobs.Delete(cmd)
	}
}

func interruptProcessTree(cmd *exec.Cmd, _ time.Duration) error {
	if job, ok := loadCommandJob(cmd); ok {
		if err := windows.TerminateJobObject(job, 1); err == nil {
			return nil
		}
	}
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err == nil || errors.Is(err, os.ErrProcessDone) {
		return nil
	} else {
		return err
	}
}

func loadCommandJob(cmd *exec.Cmd) (windows.Handle, bool) {
	if cmd == nil {
		return 0, false
	}
	value, ok := processJobs.Load(cmd)
	if !ok {
		return 0, false
	}
	job, ok := value.(windows.Handle)
	if !ok {
		processJobs.Delete(cmd)
		return 0, false
	}
	return job, true
}
