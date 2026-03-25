//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func waitForProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if err == nil {
			if timeout > 0 && time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for app process")
			}
			time.Sleep(updaterRetryStep)
			continue
		}
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return nil
	}
}

func startDetached(path string) error {
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	return cmd.Start()
}
