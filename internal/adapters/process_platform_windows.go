//go:build windows

package adapters

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
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
	if cmd == nil {
		return
	}
	if v, ok := processJobs.LoadAndDelete(cmd); ok {
		if job, ok := v.(windows.Handle); ok {
			_ = windows.CloseHandle(job)
		}
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

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	vtProcessingFlag = 0x0004
	createNoWindow   = 0x00000008
)

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

func enableConsoleVirtualTerminal() {
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	_, _, err := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if err != nil {
		return
	}
	procSetConsoleMode.Call(uintptr(handle), uintptr(mode|vtProcessingFlag))
}

func applyUpdatePlatform(tmp, dest string) error {
	bat := strings.TrimSuffix(dest, ".exe") + ".update.bat"

	content := fmt.Sprintf(
		"@echo off\r\n"+
			"timeout /t 2 /nobreak >nul\r\n"+
			":retry\r\n"+
			"move /y \"%s\" \"%s\" >nul 2>&1\r\n"+
			"if errorlevel 1 ( timeout /t 2 /nobreak >nul & goto retry )\r\n"+
			"del \"%%~f0\"\r\n",
		tmp, dest,
	)

	if err := os.WriteFile(bat, []byte(content), 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write update bat: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/D", "/C", bat)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(bat)
		return fmt.Errorf("launch update bat: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
