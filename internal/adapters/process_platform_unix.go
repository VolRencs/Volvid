//go:build !windows

package adapters

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ---- merged from process_unix.go ----

func configureCommandForProcessTree(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		return
	}

	attr := *cmd.SysProcAttr
	attr.Setpgid = true
	cmd.SysProcAttr = &attr
}

func startProcessTree(*exec.Cmd) error {
	return nil
}

func cleanupProcessTree(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if v, ok := killTimers.LoadAndDelete(cmd); ok {
		if timer, ok := v.(*time.Timer); ok {
			timer.Stop()
		}
	}
}

var killTimers sync.Map // *exec.Cmd -> *time.Timer

func interruptProcessTree(cmd *exec.Cmd, grace time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		return nil
	}

	err := signalProcessGroup(pid, syscall.SIGTERM)
	if grace > 0 {
		timer := time.AfterFunc(grace, func() {
			if _, ok := killTimers.LoadAndDelete(cmd); ok {
				_ = signalProcessGroup(pid, syscall.SIGKILL)
			}
		})
		if old, loaded := killTimers.LoadOrStore(cmd, timer); loaded {
			if oldTimer, ok := old.(*time.Timer); ok {
				_ = oldTimer.Stop()
			}
			killTimers.Store(cmd, timer)
		}
	}
	return err
}

func signalProcessGroup(pid int, sig syscall.Signal) error {
	err := syscall.Kill(-pid, sig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if !errors.Is(err, syscall.EPERM) {
		return err
	}
	err = syscall.Kill(pid, sig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// ---- merged from platform_unix.go ----

func applyUpdatePlatform(tmp, dest string) error {
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("chmod new binary: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func enableConsoleVirtualTerminal() {}
