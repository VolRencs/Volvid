//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

func waitForProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}

	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(handle)

	var waitMillis uint32 = windows.INFINITE
	if timeout > 0 {
		waitMillis = uint32(timeout.Milliseconds())
	}
	state, err := windows.WaitForSingleObject(handle, waitMillis)
	if err != nil {
		return fmt.Errorf("wait for app process: %w", err)
	}
	if state == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("timeout waiting for app process")
	}
	return nil
}

func startDetached(path string) error {
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	return cmd.Start()
}
