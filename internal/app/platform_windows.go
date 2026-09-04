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

const (
	vtProcessingFlag = 0x0004
	createNoWindow   = 0x00000008
)

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

func enableConsoleVirtualTerminal() {
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	_, _, err := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if err != nil {
		return
	}
	procSetConsoleMode.Call(uintptr(handle), uintptr(mode|vtProcessingFlag))
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
		return fmt.Errorf("write update bat: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/D", "/C", bat)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(bat)
		return fmt.Errorf("launch update bat: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
