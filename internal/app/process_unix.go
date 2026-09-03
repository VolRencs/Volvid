//go:build !windows

package app

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

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

// killTimers tracks pending SIGKILL timers armed by interruptProcessTree.
// The timer is stopped once the command is reaped, so a late SIGKILL can
// never hit a recycled PID.
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
			_ = signalProcessGroup(pid, syscall.SIGKILL)
		})
		if old, loaded := killTimers.LoadOrStore(cmd, timer); loaded {
			// A previous timer is still pending (e.g. Cancel ran twice):
			// keep a single SIGKILL schedule per command.
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
