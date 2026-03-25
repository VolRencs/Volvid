//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	windowsUpdateRetrySeconds = 90
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
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func applyUpdatePlatform(tmp, dest string) error {
	base := strings.TrimSuffix(dest, ".exe")
	script := base + ".update.bat"
	_ = os.Remove(base + ".update.ps1")
	_ = os.Remove(script)

	if err := os.WriteFile(script, []byte(windowsUpdateScript()), 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("создание update-bat: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/D", "/C", script, tmp, dest)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(script)
		return fmt.Errorf("запуск update-bat: %w", err)
	}
	return nil
}

func windowsUpdateScript() string {
	return fmt.Sprintf(
		"@echo off\r\n"+
			"setlocal enableextensions\r\n"+
			"set \"SRC=%%~1\"\r\n"+
			"set \"DST=%%~2\"\r\n"+
			"set \"SELF=%%~f0\"\r\n"+
			"set /a RETRIES=%d\r\n"+
			":retry\r\n"+
			"if not exist \"%%SRC%%\" goto fail\r\n"+
			"copy /Y \"%%SRC%%\" \"%%DST%%\" >nul 2>&1\r\n"+
			"if not errorlevel 1 goto cleanup\r\n"+
			"set /a RETRIES-=1\r\n"+
			"if %%RETRIES%% LEQ 0 goto fail\r\n"+
			"timeout /t 1 /nobreak >nul 2>&1\r\n"+
			"goto retry\r\n"+
			":cleanup\r\n"+
			"del /f /q \"%%SRC%%\" >nul 2>&1\r\n"+
			"start \"\" /b cmd.exe /D /C \"ping 127.0.0.1 -n 2 >nul & del /f /q \"\"%%SELF%%\"\" >nul 2>&1\"\r\n"+
			"exit /b 0\r\n"+
			":fail\r\n"+
			"del /f /q \"%%SRC%%\" >nul 2>&1\r\n"+
			"start \"\" /b cmd.exe /D /C \"ping 127.0.0.1 -n 2 >nul & del /f /q \"\"%%SELF%%\"\" >nul 2>&1\"\r\n"+
			"exit /b 1\r\n",
		windowsUpdateRetrySeconds,
	)
}
