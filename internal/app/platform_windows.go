//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableVirtualTerminalProcessing = 0x0004

func init() {
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	procSetConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcessing))
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
		_ = os.Remove(tmp)
		return fmt.Errorf("создание update-bat: %w", err)
	}

	cmd := exec.Command("cmd", "/c", bat)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(bat)
		return fmt.Errorf("запуск update-bat: %w", err)
	}
	return nil
}
