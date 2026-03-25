//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	windowsUpdaterWaitTimeoutSeconds = 90
)

func init() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
}

func applyUpdatePlatform(dlURL, dest string) error {
	updater := WindowsUpdaterPath()
	if info, err := os.Stat(updater); err != nil || info.Size() == 0 {
		return fmt.Errorf("updater не найден: %s", updater)
	}

	cmd := exec.Command(
		updater,
		"--wait-pid", strconv.Itoa(os.Getpid()),
		"--download-url", dlURL,
		"--target", dest,
		"--restart", dest,
		"--timeout", strconv.Itoa(windowsUpdaterWaitTimeoutSeconds),
	)
	cmd.Dir = filepath.Dir(dest)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск updater-а: %w", err)
	}
	return nil
}
