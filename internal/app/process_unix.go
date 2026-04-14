//go:build !windows

package app

import (
	"errors"
	"os/exec"
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

func cleanupProcessTree(*exec.Cmd) {}

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
		go func() {
			timer := time.NewTimer(grace)
			defer timer.Stop()
			<-timer.C
			_ = signalProcessGroup(pid, syscall.SIGKILL)
		}()
	}
	return err
}

func signalProcessGroup(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pid, sig); err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	} else if !errors.Is(err, syscall.EPERM) {
		return err
	}

	if err := syscall.Kill(pid, sig); err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	} else {
		return err
	}
}
