//go:build !windows

package main

import (
	"os"
	"syscall"
)

func detachedProcess() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

func applyUpdatePlatform(tmp, dest string) error {
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
