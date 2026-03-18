//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func init() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
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
		os.Remove(tmp)
		return err
	}
	cmd := exec.Command("cmd", "/c", bat)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		os.Remove(tmp)
		os.Remove(bat)
		return err
	}
	return nil
}
